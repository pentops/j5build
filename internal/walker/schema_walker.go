package walker

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/reflwrap"
	"github.com/pentops/j5/lib/j5reflect"
)

// containerField is a spec linked to a reflection container field.
type containerField struct {
	rootName string
	path     []string

	container reflwrap.ContainerField
	spec      *BlockSpec
}

func (sc *containerField) SchemaName() string {
	if sc.spec.DebugName != "" {
		return sc.rootName + " (" + sc.spec.DebugName + ")"
	}
	return sc.rootName
}

type schemaWalker struct {
	blockSet  blockScope
	leafBlock *containerField
	schemaSet *SchemaSet
}

func newRootSchemaWalker(spec *ConversionSpec, root j5reflect.Object) (*schemaWalker, error) {
	ss := &SchemaSet{
		givenSpecs:  spec.GlobalDefs,
		cachedSpecs: map[string]*BlockSpec{},
	}
	if ss.givenSpecs == nil {
		ss.givenSpecs = map[string]*BlockSpec{}
	}

	rootContainer := reflwrap.NewContainerField(root)
	rootWrapped, err := ss.wrapContainer(rootContainer, []string{})
	if err != nil {
		return nil, err
	}

	return &schemaWalker{
		schemaSet: ss,
		blockSet:  blockScope{*rootWrapped},
		leafBlock: rootWrapped,
	}, nil
}

func (sw *schemaWalker) newChild(container *containerField, newScope bool) *schemaWalker {
	var newBlockSet blockScope
	if newScope {
		newBlockSet = blockScope{*container}
	} else {
		newBlockSet = append(sw.blockSet, *container)
	}
	return &schemaWalker{
		blockSet:  newBlockSet,
		leafBlock: container,
		schemaSet: sw.schemaSet,
	}
}

func (sw *schemaWalker) schemaNames() []string {
	return sw.blockSet.SchemaNames()
}

// fieldPathInLeaf is called when parsing tag lines, where only the leaf context
// is considered.
func (sw *schemaWalker) fieldPathInLeaf(path PathSpec) (reflwrap.Field, error) {
	atPath, err := walkPath(sw.leafBlock.container, path)
	if err != nil {
		return nil, schemaError(path, err)
	}

	return atPath, nil
}

func (sw *schemaWalker) findAttribute(ref ast.Reference) (reflwrap.Field, error) {
	if len(ref) == 0 {
		return nil, fmt.Errorf("invalid attribute reference %#v", ref)
	}

	return sw.walkReferences(ref)
}

func (sw *schemaWalker) walkReferences(ref ast.Reference) (reflwrap.Field, error) {
	root, rest := ref[0], ref[1:]
	if len(rest) == 0 {
		return sw.blockSet.fieldForAttribute(root.String())
	}

	container, err := sw.findBlockStep(root)
	if err != nil {
		return nil, err
	}

	newWalker := sw.newChild(container, true)
	return newWalker.walkReferences(rest)
}

func (sw *schemaWalker) findBlock(ref ast.Reference) (*containerField, error) {
	if len(ref) != 1 {
		return nil, fmt.Errorf("TODO: BLOCK Namespace Tags %#v", ref)
	}

	name := ref[0]
	return sw.findBlockStep(name)
}

func (sw *schemaWalker) findBlockStep(ref ast.Ident) (*containerField, error) {

	container, err := sw.blockSet.containerForBlock(ref.String())
	if err != nil {
		if _, ok := err.(NoBlockFoundError); ok {
			err = fmt.Errorf("no block named %q", ref)
		}
		err = errpos.AddPosition(err, ref.Start)
		return nil, err
	}

	return sw.schemaSet.wrapContainer(container, []string{ref.String()})
}

func (sw *schemaWalker) containerFromLeaf(path PathSpec) (*containerField, error) {
	atPath, err := walkPath(sw.leafBlock.container, path)
	if err != nil {
		return nil, schemaError(path, err)
	}

	container, err := atPath.AsContainer()
	if err != nil {
		return nil, err
	}

	return sw.schemaSet.wrapContainer(container, path)
}

func (sw *schemaWalker) printScope(logf func(string, ...interface{})) {
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
