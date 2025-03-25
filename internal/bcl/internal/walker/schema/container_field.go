package schema

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/lib/j5reflect"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/j5build/internal/bcl/gen/j5/bcl/v1/bcl_j5pb"
)

// containerField is a spec linked to a reflection container field.
type containerField struct {
	schemaName string
	path       []string
	name       string

	transparentPath []*containerField

	container j5PropSet
	spec      BlockSpec
	isRoot    bool
	location  *bcl_j5pb.SourceLocation
}

type j5PropSet interface {
	SchemaName() string
	RangePropertySchemas(j5reflect.RangePropertySchemasCallback) error
	NewValue(name string) (j5reflect.Field, error)
	HasProperty(name string) bool
	GetProperty(name string) (j5reflect.Property, error)
	GetOrCreateValue(name string) (j5reflect.Field, error)
	ContainerSchema() j5schema.Container
	ListPropertyNames() []string
}

type mapContainer struct {
	mapNode j5reflect.MapField
}

func (mc mapContainer) SchemaName() string {
	return mc.mapNode.FullTypeName()
}

func (mc mapContainer) RangePropertySchemas(cb j5reflect.RangePropertySchemasCallback) error {
	return cb("*", false, mc.mapNode.ItemSchema().ToJ5Field())
}

func (mc mapContainer) NewValue(name string) (j5reflect.Field, error) {
	return mc.mapNode.NewElement(name)
}

func (mc mapContainer) GetOrCreateValue(name string) (j5reflect.Field, error) {
	return mc.mapNode.GetOrCreateElement(name)
}

func (mc mapContainer) HasProperty(name string) bool {
	return true
}

type mapSchema struct {
	itemSchema j5schema.FieldSchema
}

func (ms mapSchema) PropertyField(name string) j5schema.FieldSchema {
	return ms.itemSchema
}

func (ms mapSchema) WalkToProperty(name ...string) (j5schema.FieldSchema, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	// name becomes the key, the rest is in the item
	if len(name) == 1 {
		return ms.itemSchema, nil
	}
	itemAsContainer, ok := ms.itemSchema.AsContainer()
	if !ok {
		return nil, fmt.Errorf("map item is not a container")
	}
	return itemAsContainer.WalkToProperty(name[1:]...)
}

func (mc mapContainer) ContainerSchema() j5schema.Container {
	return mapSchema{itemSchema: mc.mapNode.ItemSchema()}
}

func (mc mapContainer) GetProperty(name string) (j5reflect.Property, error) {
	return nil, fmt.Errorf("maps have no property")
}

func (mc mapContainer) ListPropertyNames() []string {
	return []string{"<any map key>"}
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

func (sc *containerField) SchemaName() string {
	if sc.spec.DebugName != "" {
		return sc.schemaName + " (" + sc.spec.DebugName + ")"
	}
	return sc.schemaName
}
func (sc *containerField) getOrSetValue(name string, hint SourceLocation) (Field, error) {
	val, err := sc.container.GetOrCreateValue(name)
	if err != nil {
		return nil, err
	}
	return sc.wrap(val, hint)
}

func (sc *containerField) newValue(name string, hint SourceLocation) (Field, error) {
	val, err := sc.container.NewValue(name)
	if err != nil {
		return nil, err
	}
	return sc.wrap(val, hint)
}

func (sc *containerField) wrap(val j5reflect.Field, hint SourceLocation) (Field, error) {
	protoPath := val.ProtoPath()
	location := sc.location
	for _, elem := range protoPath {
		location = childSourceLocation(location, elem, hint)
	}
	ff := &field{
		Field:    val,
		location: location,
	}
	return ff, nil
}

func childSourceLocation(in *bcl_j5pb.SourceLocation, name string, hint SourceLocation) *bcl_j5pb.SourceLocation {
	if in.Children == nil {
		in.Children = map[string]*bcl_j5pb.SourceLocation{}
	}
	if _, ok := in.Children[name]; !ok {
		in.Children[name] = &bcl_j5pb.SourceLocation{
			StartLine:   int32(hint.Start.Line),
			StartColumn: int32(hint.Start.Column),
			EndLine:     int32(hint.End.Line),
			EndColumn:   int32(hint.End.Column),
		}
	}
	return in.Children[name]
}

type PathErrorType int

const (
	UnknownPathError PathErrorType = iota
	NodeNotContainer
	NodeNotScalar
	NodeNotScalarArray
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
		return fmt.Sprintf("node at %q (%s) is %s, not a container", strings.Join(wpe.Path, "."), wpe.Field, wpe.Schema)
	case NodeNotScalar:
		return fmt.Sprintf("node at %q is not a scalar (is %s)", strings.Join(wpe.Path, "."), wpe.Schema)
	case NodeNotScalarArray:
		return fmt.Sprintf("node at %q is not a scalar array (is %s)", strings.Join(wpe.Path, "."), wpe.Schema)
	case RootNotFound:
		if wpe.Schema != "" {
			return fmt.Sprintf("root %q unknown in %s, available: %v", wpe.Field, wpe.Schema, wpe.Available)
		}
		return fmt.Sprintf("root %q unknown, available: %v", wpe.Field, wpe.Available)

	case NodeNotFound:
		return fmt.Sprintf("node %q not found in %s", wpe.Field, wpe.Schema)
	}
	return wpe.Err.Error()
}

func unexpectedPathError(field string, err error) *WalkPathError {
	return &WalkPathError{
		Field: field,
		Type:  UnknownPathError,
		Err:   err,
	}
}

func (container *containerField) walkPath(path []string, loc SourceLocation) ([]*containerField, *WalkPathError) {
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

	val, err := container.container.GetOrCreateValue(name)
	if err != nil {
		return nil, unexpectedPathError(name, err)
	}

	protoPath := val.ProtoPath()
	schemaPath := container.path
	var fieldWithContainer j5PropSet
	if array, ok := val.AsArrayOfContainer(); ok {
		element, idx := array.NewContainerElement()
		protoPath = append(protoPath, element.ProtoPath()...)
		schemaPath = append(schemaPath, name, fmt.Sprintf("[%d]", idx))
		fieldWithContainer = element

	} else if container, ok := val.AsContainer(); ok {
		fieldWithContainer = container
		schemaPath = append(schemaPath, name)
	} else if mapNode, ok := val.AsMap(); ok {
		fieldWithContainer = mapContainer{mapNode: mapNode}
		schemaPath = append(schemaPath, name)
	} else {
		return nil, &WalkPathError{
			Path:   path,
			Field:  val.FullTypeName(),
			Type:   NodeNotContainer,
			Schema: val.TypeName(),
		}
	}

	sourceLocation := container.location
	for _, elem := range protoPath {
		sourceLocation = childSourceLocation(sourceLocation, elem, loc)
	}
	childContainer := &containerField{
		name:       name,
		path:       schemaPath,
		schemaName: fieldWithContainer.SchemaName(),
		container:  fieldWithContainer,
		location:   sourceLocation,
	}

	if len(resst) == 0 {
		return []*containerField{childContainer}, nil
	}

	endField, pathErr := childContainer.walkPath(resst, loc)
	if pathErr != nil {
		pathErr.Path = append([]string{pathErr.Field}, pathErr.Path...)
		pathErr.Field = name
		return nil, pathErr
	}
	endField = append(endField, childContainer)
	return endField, nil
}
