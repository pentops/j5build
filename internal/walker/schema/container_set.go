package schema

import (
	"fmt"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type PathErrorType int

const (
	UnknownPathError PathErrorType = iota
	NodeNotContainer
	NodeNotScalar
	NodeNotFound
	RootNotFound
)

type WalkPathError struct {
	Type PathErrorType

	// for unknown:
	Err error

	// for RootNotFound and NodeNotFound:
	Available []string
	Schema    string
	Field     string
	Path      []string
}

func (wpe *WalkPathError) Error() string {
	return wpe.LongMessage()

}

func (wpe *WalkPathError) LongMessage() string {
	switch wpe.Type {
	case NodeNotContainer:
		return fmt.Sprintf("wanted a container field, got %s", wpe.Schema)
	case NodeNotScalar:
		return fmt.Sprintf("wanted a scalar, got %s", wpe.Schema)
	case RootNotFound:
		return fmt.Sprintf("root %q unknown, available: %v", wpe.Field, wpe.Available)
	case NodeNotFound:
		return fmt.Sprintf("node %q not found in %s", wpe.Field, wpe.Schema)
	}
	return wpe.Err.Error()
}

type containerSet []containerField

// containerField is a spec linked to a reflection container field.
type containerField struct {
	schemaName string
	path       []string
	name       string

	container j5reflect.PropertySet
	spec      BlockSpec
	location  *sourcedef_j5pb.SourceLocation
}

func (sc *containerField) Spec() BlockSpec {
	return sc.spec
}

func (sc *containerField) Name() string {
	return sc.name
}

func (sc *containerField) Path() []string {
	return sc.path
}

func (sc *containerField) Location() *sourcedef_j5pb.SourceLocation {
	return sc.location
}

func (sc *containerField) SchemaName() string {
	if sc.spec.DebugName != "" {
		return sc.schemaName + " (" + sc.spec.DebugName + ")"
	}
	return sc.schemaName
}

func (bs containerSet) schemaNames() []string {
	names := make([]string, 0, len(bs))
	for _, block := range bs {
		names = append(names, block.schemaName)
	}
	return names
}

func (bs containerSet) hasAttribute(name string) bool {
	for _, blockSchema := range bs {
		if node, ok := blockSchema.spec.Children[name]; ok && node.IsScalar {
			return true
		}
	}
	return false
}

func (bs containerSet) hasBlock(name string) bool {
	for _, blockSchema := range bs {
		if _, ok := blockSchema.spec.Children[name]; ok {
			return true
		}
	}
	return false
}

func (bs containerSet) listAttributes() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName, spec := range blockSchema.spec.Children {
			if !spec.IsScalar {
				continue
			}
			possibleNames = append(possibleNames, blockName)
		}
	}

	return possibleNames
}

func (bs containerSet) listBlocks() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName, spec := range blockSchema.spec.Children {
			if !spec.IsContainer {
				continue
			}
			possibleNames = append(possibleNames, blockName)
		}
	}
	return possibleNames
}

type child struct {
	spec      ChildSpec
	container j5reflect.PropertySet
}

func (bs containerSet) child(name string) *child {
	for _, blockSchema := range bs {
		pathToField, ok := blockSchema.spec.Children[name]
		if !ok {
			continue
		}
		return &child{
			spec:      pathToField,
			container: blockSchema.container,
		}
	}
	return nil
}

func fieldToContainer(parent *containerField, prop j5reflect.Field) (*containerField, bool, error) {
	if parent.location.Children == nil {
		parent.location.Children = map[string]*sourcedef_j5pb.SourceLocation{}
	}

	ps := prop.ProtoPath()

	last := parent.location
	for _, p := range ps {
		next := last.Children[p]
		if next == nil {
			next = &sourcedef_j5pb.SourceLocation{
				Children: map[string]*sourcedef_j5pb.SourceLocation{},
			}
			last.Children[p] = next
		}
		last = next
	}
	childLocation := last

	propContainer, ok := prop.AsContainer()
	if ok {
		container, err := propContainer.GetOrCreateContainer()
		if err != nil {
			return nil, false, err
		}
		wrapped := &containerField{
			path:       append(parent.path, ps...),
			schemaName: container.SchemaName(),
			container:  container,
			location:   childLocation,
		}
		return wrapped, true, nil
	}

	arrayField, ok := prop.AsArrayOfContainer()
	if ok {
		container, idx, err := arrayField.NewContainerElement()
		if err != nil {
			return nil, false, err
		}

		idxStr := fmt.Sprintf("%d", idx)
		newPath := append(ps, idxStr)
		cl := &sourcedef_j5pb.SourceLocation{}
		childLocation.Children[idxStr] = cl

		wrapped := &containerField{
			path:       append(parent.path, newPath...),
			schemaName: container.SchemaName(),
			container:  container,
			location:   cl,
		}
		return wrapped, true, nil
	}

	return nil, false, nil
}

func walkPath(container *containerField, path []string) (*containerField, *WalkPathError) {
	if len(path) == 0 {
		return nil, &WalkPathError{
			Path: path,
			Err:  fmt.Errorf("empty path"),
		}
	}

	name, resst := path[0], path[1:]
	if !container.container.HasProperty(name) {
		return nil, &WalkPathError{
			Field:     name,
			Type:      NodeNotFound,
			Schema:    container.SchemaName(),
			Available: container.container.ListPropertyNames(),
		}
	}

	prop, err := container.container.GetProperty(name)
	if err != nil {
		return nil, &WalkPathError{
			Field:  name,
			Err:    err,
			Schema: container.SchemaName(),
		}
	}

	childContainer, ok, err := fieldToContainer(container, prop)
	if err != nil {
		return nil, &WalkPathError{
			Field:  name,
			Type:   UnknownPathError,
			Err:    err,
			Schema: container.SchemaName(),
		}
	}
	if !ok {
		return nil, &WalkPathError{
			Field:  name,
			Type:   NodeNotContainer,
			Schema: container.SchemaName(),
		}
	}

	if len(resst) == 0 {
		return childContainer, nil
	}

	endField, pathErr := walkPath(childContainer, resst)
	if pathErr != nil {
		pathErr.Path = append([]string{pathErr.Field}, pathErr.Path...)
		pathErr.Field = name
		return nil, pathErr
	}
	return endField, nil
}
