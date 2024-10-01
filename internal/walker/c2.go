package walker

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/parser"
	"github.com/pentops/bcl.go/internal/walker/schema"
)

func WalkSchema(scope *schema.Scope, body parser.Body, verbose bool) error {

	rootContext := &walkContext{
		scope:   scope,
		path:    []string{""},
		verbose: verbose,
	}

	rootErr := rootContext.run(func(sc Context) error {
		return doBody(sc, body)
	})
	if rootErr == nil {
		return nil
	}
	if rootContext.verbose {
		logError(rootErr)
	}
	return rootErr

}

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

func pointPosition(point parser.Position) errpos.Position {
	return errpos.Position{
		Start: point,
		End:   point,
	}
}

func spanPosition(start, end parser.Position) errpos.Position {
	return errpos.Position{
		Start: start,
		End:   end,
	}
}

var ErrUnexpectedTag = fmt.Errorf("unexpected tag")
var ErrUnexpectedQualifier = fmt.Errorf("unexpected qualifier")

func doBody(sc Context, body parser.Body) error {
	for _, decl := range body.Statements {
		switch decl := decl.(type) {

		case *parser.Description:
			sc.Logf("Description Statement %#v", decl)
			err := doDescription(sc, decl)
			if err != nil {
				err = errpos.AddPosition(err, decl.Position())
				return err
			}

		case *parser.Assignment:
			sc.Logf("Assign Statement %#v <- %#v (%s)", decl.Key, decl.Value, decl.SourceNode.Start)
			err := doAssign(sc, decl)
			if err != nil {
				err = errpos.AddPosition(err, decl.Position())
				return err
			}
			sc.Logf("Assign OK")

		case *parser.Block:
			sc.Logf("Block Statement %#v", decl.BlockHeader)
			err := doFullBlock(sc, decl)
			if err != nil {
				err = errpos.AddPosition(err, decl.Position())
				return err
			}
			sc.Logf("Block OK")

		default:
			return fmt.Errorf("unexpected statement type %T", decl)
		}
	}
	return nil
}

func doAssign(sc Context, a *parser.Assignment) error {
	if a.Append {
		return sc.AppendAttribute(nil, a.Key.Idents, a.Value)
	}
	return sc.SetAttribute(nil, a.Key.Idents, a.Value)
}

func doDescription(sc Context, decl *parser.Description) error {
	if err := sc.SetDescription(parser.NewStringValue(decl.Value, decl.SourceNode)); err != nil {
		err = errpos.AddPosition(err, decl.Position())
		return err
	}
	return nil
}

func doFullBlock(sc Context, decl *parser.Block) error {

	typeTag := decl.BlockHeader.Type

	newScope, err := sc.BuildScope(nil, typeTag.Idents, ResetScope)
	if err != nil {
		return fmt.Errorf("WithContainer, building scope: %w", err)
	}

	err = sc.WithScope(newScope, func(sc Context, blockSpec schema.BlockSpec) error {
		return doBlock(sc, blockSpec, decl)
	})
	if err != nil {
		return err
	}

	return nil
}

type popSet struct {
	items        []parser.TagValue
	lastItem     parser.TagValue
	lastPosition parser.Position
}

func newPopSet(items []parser.TagValue, startPos parser.Position) popSet {
	return popSet{
		lastPosition: startPos,
		items:        items,
	}
}

func (ps *popSet) popFirst() (parser.TagValue, bool) {
	if len(ps.items) == 0 {
		return ps.lastItem, false
	}
	item := ps.items[0]
	ps.lastItem = item
	ps.items = ps.items[1:]
	ps.lastPosition = item.Position().End
	return item, true
}

func (ps *popSet) hasMore() bool {
	return len(ps.items) > 0
}

func doBlock(sc Context, spec schema.BlockSpec, bs *parser.Block) error {

	rootBlockSpec := spec

	gotTags := newPopSet(bs.BlockHeader.Tags, bs.BlockHeader.Type.End)

	return walkTags(sc, spec, gotTags, func(sc Context, spec schema.BlockSpec) error {

		gotQualifiers := newPopSet(bs.BlockHeader.Qualifiers, bs.BlockHeader.Start)

		return walkQualifiers(sc, spec, gotQualifiers, func(sc Context, spec schema.BlockSpec) error {
			if bs.BlockHeader.Description != nil {

				if rootBlockSpec.Description == nil {
					return sc.WrapErr(fmt.Errorf("block %q has no description field", spec.ErrName()), bs.BlockHeader.Description)
				}
				if err := sc.SetAttribute(schema.PathSpec{*rootBlockSpec.Description}, nil, parser.NewStringValue(bs.Description.Value, bs.SourceNode)); err != nil {
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

func checkBang(sc Context, tagSpec schema.Tag, gotTag parser.TagValue) error {
	if gotTag.Mark == parser.TagMarkNone {
		return nil
	}
	var path schema.PathSpec
	if gotTag.Mark == parser.TagMarkBang {
		if tagSpec.BangFieldName == nil {
			return sc.WrapErr(fmt.Errorf("tag %s does not support bang", tagSpec.FieldName), gotTag)
		}
		path = schema.PathSpec{*tagSpec.BangFieldName}
	} else if gotTag.Mark == parser.TagMarkQuestion {
		if tagSpec.QuestionFieldName == nil {
			return sc.WrapErr(fmt.Errorf("tag %s does not support question", tagSpec.FieldName), gotTag)
		}
		path = schema.PathSpec{*tagSpec.QuestionFieldName}
	}

	sc.Logf("Applying Tag Mark, %#v %s", tagSpec, gotTag)
	err := sc.SetAttribute(path, nil, parser.NewBoolValue(true, gotTag.Start))
	if err != nil {
		return err
	}
	return nil
}

func walkTags(sc Context, spec schema.BlockSpec, gotTags popSet, outerCallback SpanCallback) error {

	if spec.Name != nil {
		gotTag, ok := gotTags.popFirst()
		if !ok {
			if spec.Name.IsOptional {
				return outerCallback(sc, spec)
			}
			err := &ErrExpectedTag{
				Label:  "name",
				Schema: spec.ErrName(),
			}
			return sc.WrapErr(err, pointPosition(gotTags.lastPosition))
		}

		tagSpec := *spec.Name

		if err := checkBang(sc, tagSpec, gotTag); err != nil {
			return err
		}

		sc.Logf("Applying Name tag, %#v %#v", tagSpec, gotTag)
		err := sc.SetAttribute(schema.PathSpec{tagSpec.FieldName}, nil, gotTag)
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
			return sc.WrapErr(err, pointPosition(gotTags.lastPosition))
		}

		tagSpec := *spec.TypeSelect

		sc.Logf("TypeSelect %#v %s", tagSpec, gotTag)
		if gotTag.Reference == nil {
			return fmt.Errorf("type-select %s needs to be a reference", tagSpec.FieldName)
		}

		pathToType := schema.PathSpec{tagSpec.FieldName}
		if tagSpec.FieldName == "" || tagSpec.FieldName == "." {
			pathToType = nil
		}
		typeScope, err := sc.BuildScope(pathToType, gotTag.Reference.Idents, KeepScope)
		if err != nil {
			return err
		}

		return sc.WithScope(typeScope, func(sc Context, spec schema.BlockSpec) error {
			if err := checkBang(sc, tagSpec, gotTag); err != nil {
				return err
			}
			return walkTags(sc, spec, gotTags, outerCallback)
		})
	}

	if gotTags.hasMore() {
		for _, tag := range gotTags.items {
			if tag.Mark != parser.TagMarkNone {
				return sc.WrapErr(fmt.Errorf("unexpected tag mark"), tag)
			}
		}
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

func walkQualifiers(sc Context, spec schema.BlockSpec, gotQualifiers popSet, outerCallback SpanCallback) error {

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
		if err := checkBang(sc, *tagSpec, qualifier); err != nil {
			return err
		}

		if err := sc.SetAttribute(schema.PathSpec{tagSpec.FieldName}, nil, qualifier); err != nil {
			return err
		}

		if gotQualifiers.hasMore() {
			return errpos.AddPosition(ErrUnexpectedQualifier, spanPosition(gotQualifiers.items[0].Position().Start, gotQualifiers.items[len(gotQualifiers.items)-1].Position().End))
		}

		return outerCallback(sc, spec)

	}

	if qualifier.Reference == nil {
		return fmt.Errorf("qualifier %s needs to be a reference to specify a block", tagSpec.FieldName)
	}

	// WithTypeSelect selects a child container from a wrapper container at path.
	// It is intended to be used where exactly one option of the wrapper should be
	// set, so the wrapper is not included in the callback scope.
	// The node it finds at givenName should must be a block, which is appended to
	// the scope and becomes the new leaf for the callback.

	newScope, err := sc.BuildScope(schema.PathSpec{tagSpec.FieldName}, qualifier.Reference.Idents, KeepScope)
	if err != nil {
		return fmt.Errorf("building scope: %w", err)
	}

	return sc.WithScope(newScope, func(sc Context, spec schema.BlockSpec) error {
		if err := checkBang(sc, *tagSpec, qualifier); err != nil {
			return err
		}

		return walkQualifiers(sc, spec, gotQualifiers, outerCallback)
	})

}
