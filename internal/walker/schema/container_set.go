package schema

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/lib/j5reflect"
)

type PathErrorType int

const (
	UnknownPathError PathErrorType = iota
	NodeNotContainer
	NodeNotScalar
	RootNotFound
)

type WalkPathError struct {
	Path []string

	Type PathErrorType

	// for unknown:
	Err error

	// for RootNotFound:
	Available []string
	Schema    string
}

func (wpe *WalkPathError) Error() string {
	path := strings.Join(wpe.Path, ".")
	switch wpe.Type {
	case UnknownPathError:
		return fmt.Sprintf("error walking path %q: %s", path, wpe.Err)
	case NodeNotContainer:
		return fmt.Sprintf("error walking path %q: wanted a container, got %s", path, wpe.Schema)
	case NodeNotScalar:
		return fmt.Sprintf("error walking path %q: wanted a scalar, got %s", path, wpe.Schema)
	case RootNotFound:
		return fmt.Sprintf("error walking path %q: root not found, available: %v", path, wpe.Available)
	}
	return fmt.Sprintf("error walking path %v: unknown error", wpe.Path)
}

type containerSet []containerField

// containerField is a spec linked to a reflection container field.
type containerField struct {
	rootName string
	path     []string

	container j5reflect.PropertySet
	spec      BlockSpec
}

func (sc *containerField) Spec() BlockSpec {
	return sc.spec
}

func (sc *containerField) Path() []string {
	return sc.path
}

func (sc *containerField) SchemaName() string {
	if sc.spec.DebugName != "" {
		return sc.rootName + " (" + sc.spec.DebugName + ")"
	}
	return sc.rootName
}

func (bs containerSet) schemaNames() []string {
	names := make([]string, 0, len(bs))
	for _, block := range bs {
		names = append(names, block.rootName)
	}
	return names
}

func (bs containerSet) hasAttribute(name string) bool {
	for _, blockSchema := range bs {
		if _, ok := blockSchema.spec.Attributes[name]; ok {
			return true
		}
	}
	return false
}

func (bs containerSet) hasBlock(name string) bool {
	for _, blockSchema := range bs {
		if _, ok := blockSchema.spec.Blocks[name]; ok {
			return true
		}
	}
	return false
}

func (bs containerSet) listAttributes() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName := range blockSchema.spec.Attributes {
			possibleNames = append(possibleNames, blockName)
		}
	}

	return possibleNames
}

func (bs containerSet) listBlocks() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName := range blockSchema.spec.Blocks {
			possibleNames = append(possibleNames, blockName)
		}
	}
	return possibleNames
}

/*
func (bs containerSet) ListAvailableBlocks() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName := range blockSchema.spec.Blocks {
			possibleNames = append(possibleNames, blockName)
		}
	}

	sort.Strings(possibleNames)
	return possibleNames
}*/

func (bs containerSet) fieldForAttribute(name string) (j5reflect.ScalarField, *WalkPathError) {
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

		asScalar, ok := field.AsScalar()
		if ok {
			return asScalar, nil
		}

		return nil, &WalkPathError{
			Path:   []string{name},
			Type:   NodeNotScalar,
			Schema: field.TypeName(),
		}
	}

	if bs.hasBlock(name) {
		return nil, &WalkPathError{
			Path:   []string{name},
			Type:   NodeNotScalar,
			Schema: "block",
		}
	}

	return nil, &WalkPathError{
		Path:      []string{name},
		Type:      RootNotFound,
		Available: bs.listAttributes(),
	}

}

// containerForChildBlock finds a block with the given name which is registered as a
// child of *any one of the blocks* in the block set. Values along the way are
// created at default value, as is the final container, which will likely be an
// empty object or oneof.
func (bs containerSet) containerForBlock(name string) (j5reflect.PropertySet, *WalkPathError) {
	for _, blockSchema := range bs {
		pathToBlock, ok := blockSchema.spec.Blocks[name]
		if !ok {
			continue
		}

		// walk the block to the path specified in the config.
		field, pathErr := walkPath(blockSchema.container, pathToBlock)
		if pathErr != nil {
			return nil, pathErr
		}

		propAsContainer, ok, err := fieldToContainer(field)
		if err != nil {
			return nil, &WalkPathError{
				Path: pathToBlock,
				Type: UnknownPathError,
				Err:  err,
			}
		}

		if !ok {
			return nil, &WalkPathError{
				Path:   pathToBlock,
				Type:   NodeNotContainer,
				Schema: field.TypeName(),
			}
		}

		return propAsContainer, nil
	}

	return nil, &WalkPathError{
		Path:      []string{name},
		Type:      RootNotFound,
		Available: bs.listBlocks(),
	}
}

func fieldToContainer(prop j5reflect.Field) (j5reflect.PropertySet, bool, error) {
	containerField, ok := prop.AsContainer()
	if ok {
		container, err := containerField.GetOrCreateContainer()
		if err != nil {
			return nil, false, err
		}
		return container, true, nil
	}

	arrayField, ok := prop.AsArrayOfContainer()
	if ok {
		container, err := arrayField.NewContainerElement()
		if err != nil {
			return nil, false, err
		}

		return container, true, nil
	}

	return nil, false, nil
}

func walkPath(container j5reflect.PropertySet, path []string) (j5reflect.Field, *WalkPathError) {
	if len(path) == 0 {
		return nil, &WalkPathError{
			Path: path,
			Err:  fmt.Errorf("empty path"),
		}
	}

	name, resst := path[0], path[1:]
	if !container.HasProperty(name) {
		return nil, &WalkPathError{
			Path: []string{name},
			Err:  fmt.Errorf("property %q not found", name),
		}
	}

	prop, err := container.GetProperty(name)
	if err != nil {
		return nil, &WalkPathError{
			Path: []string{name},
			Err:  err,
		}
	}

	if len(resst) == 0 {
		return prop, nil
	}

	propAsContainer, ok, err := fieldToContainer(prop)
	if err != nil {
		return nil, &WalkPathError{
			Path: []string{name},
			Type: UnknownPathError,
			Err:  err,
		}
	}

	if !ok {
		return nil, &WalkPathError{
			Path: []string{name},
			Type: NodeNotContainer,
		}
	}

	endField, pathErr := walkPath(propAsContainer, resst)
	if pathErr != nil {
		pathErr.Path = append([]string{name}, pathErr.Path...)
		return nil, pathErr
	}
	return endField, nil
}
