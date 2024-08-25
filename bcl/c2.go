package bcl

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
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

var ErrUnexpectedTag = fmt.Errorf("unexpected tag")
var ErrUnexpectedQualifier = fmt.Errorf("unexpected qualifier")

func ConvertTreeToSource(f *ast.File, obj j5reflect.Object, source *sourcedef_j5pb.SourceLocation, spec *schema.ConversionSpec) error {
	return walker.Walk(obj, source, spec, func(sc walker.Context, blockSpec schema.BlockSpec) error {
		return doBody(sc, f.Body)
	})
}

func doBody(sc walker.Context, body ast.Body) error {
	for _, decl := range body.Statements {
		switch decl := decl.(type) {
		case ast.Assignment:
			sc.Logf("Assign Statement %#v <- %#v", decl.Key, decl.Value)
			err := doAssign(sc, decl)
			if err != nil {
				return err
			}
			sc.Logf("Assign OK")

		case ast.BlockStatement:
			sc.Logf("Block Statement %#v", decl.BlockHeader)

			gotTags := newPopSet(decl.BlockHeader.Name)
			typeTag, ok := gotTags.popFirst() // "Type".
			if !ok {
				return sc.WrapErr(&ErrExpectedTag{Label: "type"}, decl.BlockHeader.End)
			}

			err := sc.WithBlock(nil, typeTag, walker.ResetScope, func(sc walker.Context, blockSpec schema.BlockSpec) error {
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

func doAssign(sc walker.Context, a ast.Assignment) error {
	return sc.SetAttribute(nil, a.Key, a.Value)
}

func doScalarTag(searchPath walker.Context, tagSpec schema.Tag, gotTag ast.Reference) error {
	searchPath.Logf("doScalarTag %#v, %q", tagSpec, gotTag)
	err := tagSpec.Validate(schema.TagTypeScalar)
	if err != nil {
		return err
	}

	err = applyScalarTag(searchPath, tagSpec, gotTag)
	if err != nil {
		return searchPath.WrapErr(err, gotTag[0].Start)
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

func doBlock(sc walker.Context, spec schema.BlockSpec, gotTags popSet[ast.Reference], bs ast.BlockStatement) error {

	return walkTags(sc, spec, gotTags, func(sc walker.Context, spec schema.BlockSpec) error {

		gotQualifiers := newPopSet(bs.BlockHeader.Qualifiers)

		return walkQualifiers(sc, spec, gotQualifiers, func(sc walker.Context, spec schema.BlockSpec) error {
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

func walkTags(sc walker.Context, spec schema.BlockSpec, gotTags popSet[ast.Reference], outerCallback walker.SpanCallback) error {

	if spec.Name != nil {
		gotTag, ok := gotTags.popFirst()
		if !ok {
			err := &ErrExpectedTag{
				Label:  "name",
				Schema: spec.ErrName(),
			}
			return sc.WrapErr(err, gotTags.lastItem[0].End)
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
			return sc.WrapErr(&ErrExpectedTag{
				Label:  "type-select",
				Schema: spec.ErrName(),
			}, gotTags.lastItem[0].End)
		}

		tagSpec := *spec.TypeSelect

		sc.Logf("TypeSelect %#v %s", tagSpec, gotTag)
		return sc.WithBlock(tagSpec.Path, gotTag, walker.KeepScope, func(sc walker.Context, spec schema.BlockSpec) error {
			return walkTags(sc, spec, gotTags, outerCallback)
		})
	}

	if gotTags.hasMore() {
		err := fmt.Errorf("no more tags expected for type %s", spec.ErrName())
		return errpos.AddPosition(err, gotTags.items[0][0].Start)
	}

	return outerCallback(sc, spec)
}

func walkQualifiers(sc walker.Context, spec schema.BlockSpec, gotQualifiers popSet[ast.Reference], outerCallback walker.SpanCallback) error {

	qualifier, ok := gotQualifiers.popFirst()
	if !ok {
		return outerCallback(sc, spec)
	}
	if spec.Qualifier == nil {
		err := fmt.Errorf("not expecting a qualifier for type %s", spec.ErrName())
		return sc.WrapErr(err, qualifier[0].Start)
	}

	tagSpec := spec.Qualifier
	sc.Logf("Qualifier %#v %s", tagSpec, qualifier)

	if !tagSpec.IsBlock {
		if err := doScalarTag(sc, *tagSpec, qualifier); err != nil {
			return err
		}

		if gotQualifiers.hasMore() {
			return errpos.AddPosition(ErrUnexpectedQualifier, gotQualifiers.items[0][0].Start)
		}

		return outerCallback(sc, spec)

	}

	// WithTypeSelect selects a child container from a wrapper container at path.
	// It is intended to be used where exactly one option of the wrapper should be
	// set, so the wrapper is not included in the callback scope.
	// The node it finds at givenName should must be a block, which is appended to
	// the scope and becomes the new leaf for the callback.
	return sc.WithBlock(tagSpec.Path, qualifier, walker.KeepScope, func(sc walker.Context, spec schema.BlockSpec) error {
		return walkQualifiers(sc, spec, gotQualifiers, outerCallback)
	})

}

func applyScalarTag(sc walker.Context, tagSpec schema.Tag, gotTag ast.Reference) error {
	if len(tagSpec.SplitRef) == 0 {
		err := sc.SetAttribute(tagSpec.Path, nil, gotTag.AsValue())
		if err != nil {
			return err
		}
		return nil
	}

	return sc.WithBlock(tagSpec.Path, nil, walker.ResetScope, func(sc walker.Context, spec schema.BlockSpec) error {

		// element 0 is the 'remainder' of the tag, after popping idents off
		// of the *RIGHT* side and setting the scalar at the TagSpec to the
		// Ident.
		tagVals := newPopSet(gotTag)
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
		reconstructedReference := ast.Reference(tagVals.items)
		remainderElement, _ := refElements.popFirst()
		err := sc.SetAttribute(remainderElement, nil, reconstructedReference.AsStringValue())
		if err != nil {
			return err
		}
		return nil
	})
}
