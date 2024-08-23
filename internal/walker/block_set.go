package walker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/reflwrap"
)

type NoNodeFoundError struct {
	path []string
}

func (e NoNodeFoundError) Error() string {
	return fmt.Sprintf("no node found for path %q", strings.Join(e.path, "."))
}

type NoBlockFoundError struct {
	name string
}

func (e NoBlockFoundError) Error() string {
	return fmt.Sprintf("unknown block %q", e.name)
}

type blockScope []containerField

func (bs blockScope) SchemaNames() []string {
	names := make([]string, 0, len(bs))
	for _, block := range bs {
		names = append(names, block.rootName)
	}
	return names
}

/*
func (bs blockSet) FindField(path []string) (reflwrap.Field, error) {
	for _, blockSchema := range bs {
		node, ok, err := blockSchema.MaybeFindField(path)
		if err != nil {
			return nil, err
		}
		if node != nil {
			return node, nil
		}
		if ok {
			return nil, nil
		}
	}
	return nil, NoNodeFoundError{path: path}
}*/

func (bs blockScope) ListAvailableBlocks() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName := range blockSchema.spec.Blocks {
			possibleNames = append(possibleNames, blockName)
		}
	}

	sort.Strings(possibleNames)
	return possibleNames
}

func (bs blockScope) fieldForAttribute(name string) (reflwrap.Field, error) {
	for _, blockSchema := range bs {
		pathToField, ok := blockSchema.spec.Attributes[name]
		if !ok {
			continue
		}

		// walk the block to the path specified in the config.
		field, err := walkPath(blockSchema.container, pathToField)
		if err != nil {
			return nil, err
		}

		return field, nil
	}

	return nil, NoBlockFoundError{name: name}
}

// containerForChildBlock finds a block with the given name which is registered as a
// child of *any one of the blocks* in the block set.
func (bs blockScope) containerForBlock(name string) (reflwrap.ContainerField, error) {
	for _, blockSchema := range bs {
		pathToBlock, ok := blockSchema.spec.Blocks[name]
		if !ok {
			continue
		}

		// walk the block to the path specified in the config.
		field, err := walkPath(blockSchema.container, pathToBlock)
		if err != nil {
			return nil, err
		}

		container, err := field.AsContainer()
		if err != nil {
			return nil, err
		}

		return container, nil
	}

	return nil, NoBlockFoundError{name: name}
}

func (bs blockScope) SetScalar(path []string, value ast.Value) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	name := path[0]
	rest := path[1:]

	block, err := bs.containerForBlock(name)
	if err != nil {
		return err
	}

	leaf, err := walkPath(block, rest)
	if err != nil {
		return err
	}

	return leaf.SetScalar(value)
}

// containerField is a spec linked to a reflection container field.
type containerField struct {
	rootName string

	container reflwrap.ContainerField
	spec      *BlockSpec
}

func newContainerField(container reflwrap.ContainerField, spec *BlockSpec) (*containerField, error) {
	return &containerField{
		rootName:  container.SchemaName(),
		container: container,
		spec:      spec,
	}, nil
}

func (sc *containerField) SchemaName() string {
	if sc.spec.DebugName != "" {
		return sc.rootName + " (" + sc.spec.DebugName + ")"
	}
	return sc.rootName
}

func walkPath(container reflwrap.ContainerField, path []string) (reflwrap.Field, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("empty path")
	}

	name, resst := path[0], path[1:]

	prop, err := container.Property(name)
	if err != nil {
		return nil, err
	}

	if len(resst) == 0 {
		return prop, nil
	}

	propAsContainer, err := prop.AsContainer()
	if err != nil {
		return nil, err
	}

	return walkPath(propAsContainer, resst)
}
