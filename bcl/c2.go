package bcl

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker"
	"github.com/pentops/j5/lib/j5reflect"
)

func ConvertTreeToSource(f *ast.File, obj j5reflect.Object, spec *walker.ConversionSpec) error {
	return walker.Walk(obj, spec, func(sc walker.Context, blockSpec walker.BlockSpec) error {
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

		case ast.BlockStatement:
			err := sc.WithBlock(decl.Name[0], func(sc walker.Context, blockSpec walker.BlockSpec) error {
				sc.Logf("Block Statement %#v", decl.BlockHeader)
				return doBlock(sc, blockSpec, decl)
			})
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unexpected statement type %T", decl)
		}
	}
	return nil
}

func doAssign(sc walker.Context, a ast.Assignment) error {
	attr, err := sc.FindAttribute(a.Key)
	if err != nil {
		return sc.WrapErr(err, a.Key[0].Start)
	}

	err = attr.SetScalar(a.Value)
	if err != nil {
		return sc.WrapErr(err, a.Key[0].Start)
	}
	return nil
}

func doScalarTag(searchPath walker.Context, tagSpec walker.Tag, gotTag ast.Reference) error {
	searchPath.LogSelf("doScalarTag %#v, %q", tagSpec, gotTag)
	err := tagSpec.Validate(walker.TagTypeScalar)
	if err != nil {
		return err
	}

	err = applyScalarTag(searchPath, tagSpec, gotTag)
	if err != nil {
		return searchPath.WrapErr(err, gotTag[0].Start)
	}
	return nil
}

type tagSet struct {
	gotTags      []ast.Reference
	lastPosition errpos.Position
}

func (tags *tagSet) pop() (ast.Reference, bool) {
	if len(tags.gotTags) == 0 {
		return nil, false
	}
	tag := tags.gotTags[0]
	tags.lastPosition = tag[0].End
	tags.gotTags = tags.gotTags[1:]
	return tag, true
}

func doBlock(sc walker.Context, spec walker.BlockSpec, bs ast.BlockStatement) error {

	gotTags := tagSet{
		gotTags:      bs.BlockHeader.Name[1:],
		lastPosition: bs.BlockHeader.Name[0][0].End,
	}

	if spec.Name != nil {
		gotTag, ok := gotTags.pop()
		if !ok {
			return sc.WrapErr(fmt.Errorf("expected name tag"), gotTags.lastPosition)
		}

		if spec.Name == nil {
			err := fmt.Errorf("unexpected tag %#v", gotTag)
			return sc.WrapErr(err, gotTag[0].Start)
		}
		tagSpec := *spec.Name

		err := doScalarTag(sc, tagSpec, gotTag)
		if err != nil {
			return err
		}
	}

	return walkTags(sc, spec, gotTags, func(sc walker.Context, spec walker.BlockSpec) error {

		if bs.BlockHeader.Description != nil {
			if len(spec.Description) == 0 {
				spec.Description = []string{"description"}
			}
			if err := sc.SetScalar(spec.Description, *bs.BlockHeader.Description); err != nil {
				return err
			}
		}

		if err := doBody(sc, bs.Body); err != nil {
			return err
		}

		return nil
	})
}

func walkTags(sc walker.Context, spec walker.BlockSpec, gotTags tagSet, outerCallback walker.SpanCallback) error {
	if spec.TypeSelect != nil {
		gotTag, ok := gotTags.pop()
		if !ok {
			return sc.WrapErr(fmt.Errorf("expected type tag"), gotTags.lastPosition)
		}

		tagSpec := *spec.TypeSelect

		sc.Logf("TypeSelect %#v %s", tagSpec, gotTag)
		return sc.WithTagProperty(tagSpec.Path, gotTag[0].String(), func(sc walker.Context, spec walker.BlockSpec) error {
			return walkTags(sc, spec, gotTags, outerCallback)
		})
	}

	if len(gotTags.gotTags) > 0 {
		return errpos.AddPosition(fmt.Errorf("not expecting any more tags, got %s", gotTags.gotTags), gotTags.gotTags[0][0].Start)
	}

	return outerCallback(sc, spec)

	/*
		for _, givenQualifier := range bs.BlockHeader.Qualifiers {
			if leafSpec.Qualifier == nil {
				err := fmt.Errorf("unexpected qualifier %#v", givenQualifier)
				err = errpos.AddPosition(err, givenQualifier[0].Start)
				return err
			}
			tagSpec := leafSpec.Qualifier

			childBlock, err := doTag(leafContext, *tagSpec, givenQualifier)
			if err != nil {
				return err
			}

			leafContext = leafContext.ChildSpan(childBlock.SchemaName(), childBlock)
			leafSpec = childBlock.spec

			switch tagSpec.Type {
			case TagTypeScalar: // already handled
			case TagTypeAppendContext:
				bodyContext.Append(childBlock)
			case TagTypeReplaceContext:
				bodyContext.Logf("Replacing context with %s", childBlock.SchemaName())
				bodyContext = leafContext
			default:
				return fmt.Errorf("unexpected tag type %d", tagSpec.Type)
			}
		}*/

}

func applyScalarTag(sc walker.Context, tagSpec walker.Tag, gotTag ast.Reference) error {
	sc.Logf("Applying scalar tag %#v %s", tagSpec, gotTag)
	if len(tagSpec.SplitRef) == 0 {
		err := sc.SetScalar(tagSpec.Path, gotTag.AsValue())
		if err != nil {
			return err
		}
		sc.Logf("Tag OK. %#v %s", tagSpec, gotTag)
		return nil
	}

	sc.Logf("Applying split-ref tag %#v %s", tagSpec.Path, gotTag)

	// element 0 is the 'remainder' of the tag, after popping idents off
	// of the *RIGHT* side and setting the scalar at the TagSpec to the
	// Ident.
	tagVals := gotTag[:]
	remainderElement := tagSpec.SplitRef[:]
	var thisElement []string
	var thisVal ast.Ident

	for len(remainderElement) > 1 {
		remainderElement, thisElement = remainderElement[:len(remainderElement)-1], remainderElement[len(remainderElement)-1]
		if len(tagVals) == 0 {
			return fmt.Errorf("expected more tags for %s", gotTag)
		}
		tagVals, thisVal = tagVals[:len(tagVals)-1], tagVals[len(tagVals)-1]
		err := sc.SetScalar(append(tagSpec.Path, thisElement...), thisVal.AsStringValue())
		if err != nil {
			return err
		}
	}

	if len(tagVals) > 0 {
		err := sc.SetScalar(append(tagSpec.Path, remainderElement[0]...), tagVals.AsStringValue())
		if err != nil {
			return err
		}
	}
	sc.Logf("Split Ref Tag OK. %#v %s", tagSpec, gotTag)
	return nil
}
