package walker

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker/schema"
)

type ErrExpectedTag struct {
	Label  string
	Schema string
}

func (e *ErrExpectedTag) Error() string {
	if e.Schema != "" {
		return fmt.Sprintf("expected %s tag for %s", e.Label, e.Schema)
	}
	return fmt.Sprintf("expected %s tag", e.Label)
}

func pointPosition(point ast.Position) errpos.Position {
	return errpos.Position{
		Start: point,
		End:   point,
	}
}

func spanPosition(start, end ast.Position) errpos.Position {
	return errpos.Position{
		Start: start,
		End:   end,
	}
}

var ErrUnexpectedTag = fmt.Errorf("unexpected tag")
var ErrUnexpectedQualifier = fmt.Errorf("unexpected qualifier")

func doBody(sc Context, body ast.Body) error {
	for _, decl := range body.Statements {
		switch decl := decl.(type) {
		case ast.ImportStatement:
			if body.IsRoot {
				continue // handled externally
			}

			return fmt.Errorf("import statement not allowed in non-root block")

		case ast.Assignment:
			sc.Logf("Assign Statement %#v <- %#v (%s)", decl.Key, decl.Value, decl.SourceNode.Start)
			err := doAssign(sc, decl)
			if err != nil {
				err = fmt.Errorf("doAssign: %w", err)
				err = sc.WrapErr(err, decl)
				return err
			}
			sc.Logf("Assign OK")

		case ast.BlockStatement:
			sc.Logf("Block Statement %#v", decl.BlockHeader)
			blockLocation := schema.SourceLocation{
				Start: decl.BlockHeader.Start,
				End:   decl.BlockHeader.End,
			}

			gotTags := newPopSet(decl.BlockHeader.Name)
			typeTag, ok := gotTags.popFirst() // "Type".
			if !ok {
				return sc.WrapErr(&ErrExpectedTag{Label: "type"}, decl.BlockHeader)
			}

			err := sc.WithContainer(&blockLocation, nil, typeTag.Idents, ResetScope, func(sc Context, blockSpec schema.BlockSpec) error {
				return doBlock(sc, blockSpec, gotTags, decl)
			})
			if err != nil {
				return err
			}
			sc.Logf("Block OK")

		default:
			return fmt.Errorf("unexpected statement type %T", decl)
		}
	}
	return nil
}

func doAssign(sc Context, a ast.Assignment) error {
	return sc.SetAttribute(nil, a.Key.Idents, a.Value)
}

func doScalarTag(searchPath Context, tagSpec schema.Tag, gotTag ast.Reference) error {
	searchPath.Logf("doScalarTag %#v, %q", tagSpec, gotTag)
	err := tagSpec.Validate(schema.TagTypeScalar)
	if err != nil {
		return err
	}

	err = applyScalarTag(searchPath, tagSpec, gotTag)
	if err != nil {
		return searchPath.WrapErr(err, gotTag)
	}
	return nil
}

type popSet[T any] struct {
	items    []T
	lastItem T
}

func newPopSet[T any](items []T) popSet[T] {
	return popSet[T]{
		items: items,
	}
}

func (ps *popSet[T]) popFirst() (T, bool) {
	if len(ps.items) == 0 {
		return ps.lastItem, false
	}
	item := ps.items[0]
	ps.lastItem = item
	ps.items = ps.items[1:]
	return item, true
}

func (ps *popSet[T]) popLast() (T, bool) {
	if len(ps.items) == 0 {
		return ps.lastItem, false
	}
	item := ps.items[len(ps.items)-1]
	ps.lastItem = item
	ps.items = ps.items[:len(ps.items)-1]
	return item, true
}

func (ps *popSet[T]) hasMore() bool {
	return len(ps.items) > 0
}

func doBlock(sc Context, spec schema.BlockSpec, gotTags popSet[ast.Reference], bs ast.BlockStatement) error {

	return walkTags(sc, spec, gotTags, func(sc Context, spec schema.BlockSpec) error {

		gotQualifiers := newPopSet(bs.BlockHeader.Qualifiers)

		return walkQualifiers(sc, spec, gotQualifiers, func(sc Context, spec schema.BlockSpec) error {
			if bs.BlockHeader.Description != nil {
				if len(spec.Description) == 0 {
					spec.Description = []string{"description"}
				}
				if err := sc.SetAttribute(spec.Description, nil, *bs.BlockHeader.Description); err != nil {
					return err
				}
			}

			if err := doBody(sc, bs.Body); err != nil {
				return err
			}

			return nil
		})
	})
}

func walkTags(sc Context, spec schema.BlockSpec, gotTags popSet[ast.Reference], outerCallback SpanCallback) error {

	if spec.Name != nil {
		gotTag, ok := gotTags.popFirst()
		if !ok {
			err := &ErrExpectedTag{
				Label:  "name",
				Schema: spec.ErrName(),
			}
			return sc.WrapErr(err, pointPosition(gotTags.lastItem.End))
		}

		tagSpec := *spec.Name
		sc.SetName(gotTag.String())

		err := doScalarTag(sc, tagSpec, gotTag)
		if err != nil {
			return err
		}
		sc.Logf("Applied Name, remaining tags: %#v", gotTags.items)
	}
	if spec.TypeSelect != nil {
		gotTag, ok := gotTags.popFirst()
		if !ok {
			err := &ErrExpectedTag{
				Label:  "type-select",
				Schema: spec.ErrName(),
			}
			return sc.WrapErr(err, pointPosition(gotTags.lastItem.End))
		}

		tagSpec := *spec.TypeSelect

		sc.Logf("TypeSelect %#v %s", tagSpec, gotTag)
		typeScope, err := sc.BuildScope(tagSpec.Path, gotTag.Idents, KeepScope)
		if err != nil {
			return err
		}

		return sc.WithScope(typeScope, func(sc Context, spec schema.BlockSpec) error {
			return walkTags(sc, spec, gotTags, outerCallback)
		})
	}

	if gotTags.hasMore() {
		err := fmt.Errorf("no more tags expected for type %s", spec.ErrName())
		return errpos.AddPosition(err, spanPosition(gotTags.items[0].Start, gotTags.items[len(gotTags.items)-1].End))
	}

	return outerCallback(sc, spec)
}

func walkQualifiers(sc Context, spec schema.BlockSpec, gotQualifiers popSet[ast.Reference], outerCallback SpanCallback) error {

	qualifier, ok := gotQualifiers.popFirst()
	if !ok {
		return outerCallback(sc, spec)
	}
	if spec.Qualifier == nil {
		err := fmt.Errorf("not expecting a qualifier for type %s", spec.ErrName())
		return sc.WrapErr(err, spanPosition(qualifier.Start, qualifier.End))
	}

	tagSpec := spec.Qualifier
	sc.Logf("Qualifier %#v %s", tagSpec, qualifier)

	if !tagSpec.IsBlock {
		if err := doScalarTag(sc, *tagSpec, qualifier); err != nil {
			return err
		}

		if gotQualifiers.hasMore() {
			return errpos.AddPosition(ErrUnexpectedQualifier, spanPosition(gotQualifiers.items[0].Start, gotQualifiers.items[len(gotQualifiers.items)-1].End))
		}

		return outerCallback(sc, spec)

	}

	// WithTypeSelect selects a child container from a wrapper container at path.
	// It is intended to be used where exactly one option of the wrapper should be
	// set, so the wrapper is not included in the callback scope.
	// The node it finds at givenName should must be a block, which is appended to
	// the scope and becomes the new leaf for the callback.
	return sc.WithContainer(nil, tagSpec.Path, qualifier.Idents, KeepScope, func(sc Context, spec schema.BlockSpec) error {
		return walkQualifiers(sc, spec, gotQualifiers, outerCallback)
	})

}

func applyScalarTag(sc Context, tagSpec schema.Tag, gotTag ast.Reference) error {
	if len(tagSpec.SplitRef) == 0 {
		err := sc.SetAttribute(tagSpec.Path, nil, gotTag)
		if err != nil {
			return err
		}
		return nil
	}

	return sc.WithContainer(nil, tagSpec.Path, nil, ResetScope, func(sc Context, spec schema.BlockSpec) error {

		// element 0 is the 'remainder' of the tag, after popping idents off
		// of the *RIGHT* side and setting the scalar at the TagSpec to the
		// Ident.
		tagVals := newPopSet(gotTag.Idents)
		refElements := newPopSet(tagSpec.SplitRef)

		// [package, schema]
		// path.to.Foo
		// package = path.to
		// schema = Foo

		for len(refElements.items) > 1 { // all but the first
			thisElement, _ := refElements.popLast()
			thisVal, ok := tagVals.popLast()
			if !ok {
				return fmt.Errorf("expected more elements for %s", gotTag)
			}

			err := sc.SetAttribute(thisElement, nil, thisVal.AsStringValue())
			if err != nil {
				return err
			}
		}

		if !tagVals.hasMore() {
			return nil
		}
		reconstructedReference := ast.NewReference(tagVals.items)
		remainderElement, _ := refElements.popFirst()
		err := sc.SetAttribute(remainderElement, nil, reconstructedReference)
		if err != nil {
			return err
		}
		return nil
	})
}
