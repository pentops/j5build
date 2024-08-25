package schema

import (
	"github.com/pentops/j5/lib/j5reflect"
)

type ScalarField interface {
	SetASTValue(j5reflect.ASTValue) error
}

type Scope interface {
	PrintScope(func(string, ...interface{}))
	SchemaNames() []string

	ChildBlock(name string) (Scope, error)
	Field(name string) (ScalarField, error)
	CurrentBlock() Container

	ListAttributes() []string
	ListBlocks() []string

	MergeScope(Scope) Scope
}

type Container interface {
	Path() []string
	Spec() BlockSpec
}

type schemaWalker struct {
	blockSet  containerSet
	leafBlock *containerField
	schemaSet *SchemaSet
}

func (sw *schemaWalker) CurrentBlock() Container {
	return sw.leafBlock
}

func NewRootSchemaWalker(spec *ConversionSpec, root j5reflect.Object) (Scope, error) {
	ss := &SchemaSet{
		givenSpecs:  spec.GlobalDefs,
		cachedSpecs: map[string]*BlockSpec{},
	}
	if ss.givenSpecs == nil {
		ss.givenSpecs = map[string]*BlockSpec{}
	}

	rootWrapped, err := ss.wrapContainer(root, []string{})
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

func (sw *schemaWalker) Field(name string) (ScalarField, error) {
	field, walkErr := sw.blockSet.fieldForAttribute(name)
	if walkErr != nil {
		return nil, walkErr
	}
	return field, nil
}

func (sw *schemaWalker) ChildBlock(name string) (Scope, error) {

	container, walkErr := sw.blockSet.containerForBlock(name)
	if walkErr != nil {
		return nil, walkErr
	}
	wrappedContainer, err := sw.schemaSet.wrapContainer(container, []string{name})
	if err != nil {
		return nil, err
	}
	newWalker := sw.newChild(wrappedContainer, true)
	return newWalker, nil
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
			logf("from %s : %s %q", block.rootName, block.spec.source, block.spec.DebugName)
		} else {
			logf("from %s : %s", block.rootName, block.spec.source)
		}
		for name, block := range block.spec.Blocks {
			logf(" - block[%s] %s", name, block)
		}
		for name, attr := range block.spec.Attributes {
			logf(" - attr[%s] %s", name, attr)
		}
	}

	if sw.leafBlock == nil {
		logf("no leaf spec")
		return
	}

	spec := sw.leafBlock.spec
	logf("leaf spec: %s", spec.errName())
	if spec.Name != nil {
		logf(" - tag[name]: %#v", spec.Name)
	}
	if spec.TypeSelect != nil {
		logf(" - tag[type]: %#v", spec.TypeSelect)
	}
	logf("-------")
}
