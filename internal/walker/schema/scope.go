package schema

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type ScalarField interface {
	SetASTValue(j5reflect.ASTValue) error
	FullTypeName() string
}

type ArrayOfScalarField interface {
	AppendASTValue(j5reflect.ASTValue) (int, error)
	FullTypeName() string
}

type SourceLocation = errpos.Position

type Scope interface {
	PrintScope(func(string, ...interface{}))
	SchemaNames() []string

	ChildBlock(name string, src SourceLocation) (Scope, *WalkPathError)
	ScalarField(name string, src SourceLocation) (ScalarField, *WalkPathError)
	Field(name string, src SourceLocation) (j5reflect.Field, *WalkPathError)

	WrapContainer(j5reflect.ContainerField) (Scope, error)

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

func NewRootSchemaWalker(ss *SchemaSet, root j5reflect.Object, sourceLoc *bcl_j5pb.SourceLocation) (Scope, error) {
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

func (sw *schemaWalker) ChildBlock(name string, source SourceLocation) (Scope, *WalkPathError) {
	root, spec, ok := sw.findBlock(name)
	if !ok {
		return nil, &WalkPathError{
			Field:     name,
			Type:      RootNotFound,
			Available: sw.blockSet.listChildren(),
		}
	}

	container, err := sw.walkToChild(root, spec.Path, source)
	if err != nil {
		return nil, err
	}

	newWalker := sw.newChild(container, true)
	return newWalker, nil
}

func (sw *schemaWalker) ScalarField(name string, source SourceLocation) (ScalarField, *WalkPathError) {
	finalField, spec, err := sw.field(name, source)
	if err != nil {
		return nil, err
	}

	if !spec.IsScalar {
		return nil, &WalkPathError{
			Path:   []string{name},
			Type:   NodeNotScalar,
			Schema: finalField.TypeName(),
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

func (sw *schemaWalker) Field(name string, source SourceLocation) (j5reflect.Field, *WalkPathError) {
	finalField, _, err := sw.field(name, source)
	if err != nil {
		return nil, err
	}

	return finalField, nil
}

func (sw *schemaWalker) field(name string, source SourceLocation) (j5reflect.Field, *ChildSpec, *WalkPathError) {
	// Root, Parent and Field.
	// The 'Root' is the container within the current scope which is identified
	// by the block name.

	// Parent is the second last element in the path, the object/oneof etc which
	// holds the field we are looking for.

	// The 'Field' is the leaf at the end of the path.

	// A Path from 'Root' to 'Parent' gives us the place we can get the field,
	// but we can't walk all the way to the field because it is a scalar, so we
	// need it in context.

	root, spec, ok := sw.findBlock(name)
	if !ok {
		return nil, nil, &WalkPathError{
			Field:     name,
			Type:      RootNotFound,
			Schema:    sw.leafBlock.schemaName,
			Available: sw.blockSet.listChildren(),
		}
	}
	if len(spec.Path) == 0 {
		return nil, nil, &WalkPathError{
			Field:  name,
			Type:   UnknownPathError,
			Schema: root.schemaName,
			Err:    fmt.Errorf("empty path, spec issue"),
		}
	}

	final, pathToParent := popLast(spec.Path)
	parentScope, err := sw.walkToChild(root, pathToParent, source)
	if err != nil {
		return nil, nil, err
	}

	if !parentScope.container.HasProperty(final) {
		return nil, nil, &WalkPathError{
			Type:      NodeNotFound,
			Available: sw.blockSet.listChildren(),
		}
	}

	finalField, newValErr := parentScope.container.NewValue(final)
	if newValErr != nil {
		return nil, nil, &WalkPathError{
			Type: UnknownPathError,
			Err:  newValErr,
		}
	}

	return finalField, spec, nil
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

func (sw *schemaWalker) WrapContainer(container j5reflect.ContainerField) (Scope, error) {
	wrapped, err := sw.schemaSet.wrapContainer(container, []string{}, nil)
	if err != nil {
		return nil, err
	}

	return sw.newChild(wrapped, true), nil
}

func (sw *schemaWalker) findBlock(name string) (*containerField, *ChildSpec, bool) {
	for _, blockSchema := range sw.blockSet {
		childSpec, ok := blockSchema.spec.Children[name]
		if !ok {
			continue
		}

		return &blockSchema, &childSpec, true
	}
	return nil, nil, false
}

func popLast[T any](list []T) (T, []T) {
	return list[len(list)-1], list[:len(list)-1]
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
