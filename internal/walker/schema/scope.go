package schema

import (
	"fmt"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type ScalarField interface {
	SetASTValue(j5reflect.ASTValue) error
	FullTypeName() string
}

type SourceLocation struct {
	Line, Col int
}

type Scope interface {
	PrintScope(func(string, ...interface{}))
	SchemaNames() []string

	ChildBlock(name string, src SourceLocation) (Scope, error)
	Field(name string, src SourceLocation) (ScalarField, error)

	CurrentBlock() Container

	ListAttributes() []string
	ListBlocks() []string

	MergeScope(Scope) Scope
}

type Container interface {
	Path() []string
	Spec() BlockSpec
	Name() string
}

type schemaWalker struct {
	blockSet  containerSet
	leafBlock *containerField
	schemaSet *SchemaSet
}

func (sw *schemaWalker) CurrentBlock() Container {
	return sw.leafBlock
}

func NewRootSchemaWalker(spec *ConversionSpec, root j5reflect.Object, sourceLoc *sourcedef_j5pb.SourceLocation) (Scope, error) {
	ss := &SchemaSet{
		givenSpecs:  spec.GlobalDefs,
		cachedSpecs: map[string]*BlockSpec{},
	}
	if ss.givenSpecs == nil {
		ss.givenSpecs = map[string]*BlockSpec{}
	}

	if sourceLoc == nil {
		return nil, fmt.Errorf("source location required")
	}

	rootWrapped, err := ss.wrapContainer(root, []string{}, sourceLoc)
	if err != nil {
		return nil, err
	}

	return &schemaWalker{
		schemaSet: ss,

		blockSet:  containerSet{*rootWrapped},
		leafBlock: rootWrapped,
	}, nil
}

func (sw *schemaWalker) newChild(container *containerField, newScope bool) *schemaWalker {
	var newBlockSet containerSet
	if newScope {
		newBlockSet = containerSet{*container}
	} else {
		newBlockSet = append(sw.blockSet, *container)
	}
	return &schemaWalker{
		blockSet:  newBlockSet,
		leafBlock: container,
		schemaSet: sw.schemaSet,
	}
}

func (sw *schemaWalker) SchemaNames() []string {
	return sw.blockSet.schemaNames()
}

func (sw *schemaWalker) ListAttributes() []string {
	return sw.blockSet.listAttributes()
}

func (sw *schemaWalker) ListBlocks() []string {
	return sw.blockSet.listBlocks()
}

func (sw *schemaWalker) Field(name string, source SourceLocation) (ScalarField, error) {
	for _, blockSchema := range sw.blockSet {
		childSpec, ok := blockSchema.spec.Children[name]
		if !ok {
			continue
		}
		if !childSpec.IsScalar {
			return nil, &WalkPathError{
				Path: []string{name},
				Type: NodeNotScalar,
			}
		}

		walkContainer := blockSchema

		pathToContainer, final := childSpec.Path[:len(childSpec.Path)-1], childSpec.Path[len(childSpec.Path)-1]

		if len(pathToContainer) > 1 {

			// walk the block to the path specified in the config.
			container, walkErr := walkPath(&blockSchema, pathToContainer)
			if walkErr != nil {
				return nil, walkErr
			}
			walkContainer = *container

		}
		if !walkContainer.container.HasProperty(final) {
			return nil, &WalkPathError{
				Type: NodeNotFound,
			}

		}

		finalField, err := walkContainer.container.GetProperty(final)
		if err != nil {
			return nil, &WalkPathError{
				Type: UnknownPathError,
				Err:  err,
			}
		}

		walkLoc := walkContainer.location
		for _, p := range finalField.ProtoPath() {
			if walkLoc.Children == nil {
				walkLoc.Children = map[string]*sourcedef_j5pb.SourceLocation{}
			}
			if walkLoc.Children[p] == nil {
				walkLoc.Children[p] = &sourcedef_j5pb.SourceLocation{}
			}
			walkLoc = walkLoc.Children[p]
		}
		walkLoc.StartLine = int32(source.Line)
		walkLoc.StartColumn = int32(source.Col)

		asScalar, ok := finalField.AsScalar()
		if ok {
			return asScalar, nil
		}

		return nil, &WalkPathError{
			Path:   []string{name},
			Type:   NodeNotScalar,
			Schema: finalField.FullTypeName(),
		}
	}

	if sw.blockSet.hasBlock(name) {
		return nil, &WalkPathError{
			Path:   []string{name},
			Type:   NodeNotScalar,
			Schema: "block",
		}
	}

	return nil, &WalkPathError{
		Field:     name,
		Type:      RootNotFound,
		Available: sw.blockSet.listAttributes(),
	}
}

func (sw *schemaWalker) ChildBlock(name string, source SourceLocation) (Scope, error) {

	for _, blockSchema := range sw.blockSet {
		childSpec, ok := blockSchema.spec.Children[name]
		if !ok {
			continue
		}

		// walk the block to the path specified in the config.
		field, pathErr := walkPath(&blockSchema, childSpec.Path)
		if pathErr != nil {
			return nil, pathErr
		}
		field.name = name

		if field.location.StartLine == 0 {
			field.location.StartLine = int32(source.Line)
			field.location.StartColumn = int32(source.Col)
		}

		spec, err := sw.schemaSet.blockSpec(field.container)
		if err != nil {
			return nil, err
		}
		field.spec = *spec

		newWalker := sw.newChild(field, true)
		return newWalker, nil
	}

	return nil, &WalkPathError{
		Field:     name,
		Type:      RootNotFound,
		Available: sw.blockSet.listBlocks(),
	}

}

func (sw *schemaWalker) MergeScope(other Scope) Scope {
	otherWalker, ok := other.(*schemaWalker)
	if !ok {
		panic("invalid merge")
	}

	newBlockSet := append(sw.blockSet, otherWalker.blockSet...)
	return &schemaWalker{
		blockSet:  newBlockSet,
		leafBlock: otherWalker.leafBlock,
		schemaSet: sw.schemaSet,
	}
}

func (sw *schemaWalker) PrintScope(logf func(string, ...interface{})) {
	logf("available blocks:")
	for _, block := range sw.blockSet {
		if block.spec.DebugName != "" {
			logf("from %s : %s %q", block.schemaName, block.spec.source, block.spec.DebugName)
		} else {
			logf("from %s : %s", block.schemaName, block.spec.source)
		}
		for name, block := range block.spec.Children {
			logf(" - [%s] %q %#v", block.TagString(), name, block.Path)
		}
	}

	if sw.leafBlock == nil {
		logf("no leaf spec")
		return
	}

	spec := sw.leafBlock.spec
	logf("leaf spec: %s", spec.ErrName())
	if spec.Name != nil {
		logf(" - tag[name]: %#v", spec.Name)
	}
	if spec.TypeSelect != nil {
		logf(" - tag[type]: %#v", spec.TypeSelect)
	}
	logf("-------")
}
