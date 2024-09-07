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

		case *ast.Description:
			sc.Logf("Description Statement %#v", decl)
			if err := sc.SetDescription(decl.Value); err != nil {
				err = sc.WrapErr(err, decl)
				return err
			}

		case *ast.Assignment:
			sc.Logf("Assign Statement %#v <- %#v (%s)", decl.Key, decl.Value, decl.SourceNode.Start)
			err := doAssign(sc, decl)
			if err != nil {
				err = fmt.Errorf("doAssign: %w", err)
				err = sc.WrapErr(err, decl)
				return err
			}
			sc.Logf("Assign OK")

		case *ast.Block:
			sc.Logf("Block Statement %#v", decl.BlockHeader)
			blockLocation := schema.SourceLocation{
				Start: decl.BlockHeader.Start,
				End:   decl.BlockHeader.End,
			}

			gotTags := newPopSet(decl.BlockHeader.Tags)

			typeTag := decl.BlockHeader.Type

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

func doAssign(sc Context, a *ast.Assignment) error {
	return sc.SetAttribute(nil, a.Key.Idents, a.Value)
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

func (ps *popSet[T]) hasMore() bool {
	return len(ps.items) > 0
}

func doBlock(sc Context, spec schema.BlockSpec, gotTags popSet[ast.TagValue], bs *ast.Block) error {

	return walkTags(sc, spec, gotTags, func(sc Context, spec schema.BlockSpec) error {

		gotQualifiers := newPopSet(bs.BlockHeader.Qualifiers)

		return walkQualifiers(sc, spec, gotQualifiers, func(sc Context, spec schema.BlockSpec) error {
			if bs.BlockHeader.Description != nil {
				if len(spec.Description) == 0 {
					spec.Description = []string{"description"}
				}
				if err := sc.SetAttribute(spec.Description, nil, ast.NewStringValue(*bs.Description, bs.SourceNode)); err != nil {
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

func walkTags(sc Context, spec schema.BlockSpec, gotTags popSet[ast.TagValue], outerCallback SpanCallback) error {

	if spec.Name != nil {
		gotTag, ok := gotTags.popFirst()
		if !ok {
			err := &ErrExpectedTag{
				Label:  "name",
				Schema: spec.ErrName(),
			}
			return sc.WrapErr(err, pointPosition(gotTags.lastItem.Position().End))
		}

		tagSpec := *spec.Name

		sc.Logf("Applying Name tag, %#v %s", tagSpec, gotTag)
		err := sc.SetAttribute(tagSpec.Path, nil, gotTag)
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
			return sc.WrapErr(err, pointPosition(gotTags.lastItem.Position().End))
		}

		tagSpec := *spec.TypeSelect

		sc.Logf("TypeSelect %#v %s", tagSpec, gotTag)
		if gotTag.Reference == nil {
			return fmt.Errorf("type-select %s needs to be a reference", tagSpec.Path)
		}
		typeScope, err := sc.BuildScope(tagSpec.Path, gotTag.Reference.Idents, KeepScope)
		if err != nil {
			return err
		}

		return sc.WithScope(typeScope, func(sc Context, spec schema.BlockSpec) error {
			return walkTags(sc, spec, gotTags, outerCallback)
		})
	}

	if gotTags.hasMore() {
		if spec.ScalarSplit != nil {
			if len(gotTags.items) != 1 {
				return fmt.Errorf("expected exactly one tag for type %s", spec.ErrName())
			}

			sc.Logf("Applying ScalarSplit %#v %#v", spec.ScalarSplit, gotTags.items[0])

			ref := gotTags.items[0]

			if err := sc.setContainerFromScalar(spec, ref); err != nil {
				return err
			}

		} else {

			err := fmt.Errorf("no more tags expected for type %s", spec.ErrName())
			return errpos.AddPosition(err, spanPosition(gotTags.items[0].Position().Start, gotTags.items[len(gotTags.items)-1].Position().End))
		}
	}

	return outerCallback(sc, spec)
}

func walkQualifiers(sc Context, spec schema.BlockSpec, gotQualifiers popSet[ast.TagValue], outerCallback SpanCallback) error {

	qualifier, ok := gotQualifiers.popFirst()
	if !ok {
		return outerCallback(sc, spec)
	}

	if spec.Qualifier == nil {
		err := fmt.Errorf("not expecting a qualifier for type %s", spec.ErrName())
		return sc.WrapErr(err, qualifier.Position())
	}

	tagSpec := spec.Qualifier
	sc.Logf("Qualifier %#v %s", tagSpec, qualifier)

	if !tagSpec.IsBlock {
		if err := sc.SetAttribute(tagSpec.Path, nil, qualifier); err != nil {
			return err
		}

		if gotQualifiers.hasMore() {
			return errpos.AddPosition(ErrUnexpectedQualifier, spanPosition(gotQualifiers.items[0].Position().Start, gotQualifiers.items[len(gotQualifiers.items)-1].Position().End))
		}

		return outerCallback(sc, spec)

	}

	if qualifier.Reference == nil {
		return fmt.Errorf("qualifier %s needs to be a reference to specify a block", tagSpec.Path)
	}

	// WithTypeSelect selects a child container from a wrapper container at path.
	// It is intended to be used where exactly one option of the wrapper should be
	// set, so the wrapper is not included in the callback scope.
	// The node it finds at givenName should must be a block, which is appended to
	// the scope and becomes the new leaf for the callback.
	return sc.WithContainer(nil, tagSpec.Path, qualifier.Reference.Idents, KeepScope, func(sc Context, spec schema.BlockSpec) error {
		return walkQualifiers(sc, spec, gotQualifiers, outerCallback)
	})

}
