package schema

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type ScalarField interface {
	SetASTValue(j5reflect.ASTValue) error
	FullTypeName() string
}

type SourceLocation = errpos.Position

type Scope interface {
	PrintScope(func(string, ...interface{}))
	SchemaNames() []string

	ChildBlock(name string, src SourceLocation) (Scope, *WalkPathError)
	Field(name string, src SourceLocation) (ScalarField, *WalkPathError)

	CurrentBlock() Container

	ListAttributes() []string
	ListBlocks() []string

	MergeScope(Scope) Scope

	TailScope() Scope
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

	rootWrapped.isRoot = true
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

func (sw *schemaWalker) Field(name string, source SourceLocation) (ScalarField, *WalkPathError) {
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

		pathToContainer, final := childSpec.Path[:len(childSpec.Path)-1], childSpec.Path[len(childSpec.Path)-1]

		walkContainer, err := sw.walkToChild(&blockSchema, pathToContainer, source)
		if err != nil {
			return nil, err
		}

		if !walkContainer.container.HasProperty(final) {
			return nil, &WalkPathError{
				Type: NodeNotFound,
			}

		}

		finalField, newVal := walkContainer.container.NewValue(final)
		if newVal != nil {
			return nil, &WalkPathError{
				Type: UnknownPathError,
				Err:  newVal,
			}
		}

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

func (sw *schemaWalker) walkToChild(blockSchema *containerField, path []string, sourceLocation SourceLocation) (*containerField, *WalkPathError) {
	if len(path) == 0 {
		return blockSchema, nil
	}

	// walk the block to the path specified in the config.
	visitedFields, pathErr := blockSchema.walkPath(path, sourceLocation)
	if pathErr != nil {
		return nil, pathErr
	}

	for _, field := range visitedFields {
		spec, err := sw.schemaSet.blockSpec(field.container)
		if err != nil {
			return nil, unexpectedPathError(field.name, err)
		}
		field.spec = *spec
	}

	mainField := visitedFields[0]
	mainField.transparentPath = visitedFields[1:]
	return mainField, nil
}

func (sw *schemaWalker) ChildBlock(name string, source SourceLocation) (Scope, *WalkPathError) {

	for _, blockSchema := range sw.blockSet {
		childSpec, ok := blockSchema.spec.Children[name]
		if !ok {
			continue
		}
		mainField, err := sw.walkToChild(&blockSchema, childSpec.Path, source)
		if err != nil {
			return nil, err
		}
		mainField.name = name

		newWalker := sw.newChild(mainField, true)
		return newWalker, nil
	}

	return nil, &WalkPathError{
		Field:     name,
		Type:      RootNotFound,
		Available: sw.blockSet.listBlocks(),
	}
}

func (sw *schemaWalker) TailScope() Scope {
	return &schemaWalker{
		blockSet:  containerSet{*sw.leafBlock},
		leafBlock: sw.leafBlock,
		schemaSet: sw.schemaSet,
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
