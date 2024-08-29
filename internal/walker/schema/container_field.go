package schema

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

// containerField is a spec linked to a reflection container field.
type containerField struct {
	schemaName string
	path       []string
	name       string

	transparentPath []*containerField

	container j5reflect.PropertySet
	field     j5reflect.ContainerField
	spec      BlockSpec
	isRoot    bool
	location  *sourcedef_j5pb.SourceLocation
}

func (sc *containerField) RunCloseHooks() error {
	if sc.spec.RunAfter != nil {
		if err := sc.spec.RunAfter.RunHook(sc.field); err != nil {
			err := fmt.Errorf("In Close Hook for %s: %w", sc.schemaName, err)
			err = errpos.AddPosition(err, sc.errPosition())
			err = errpos.AddContext(err, sc.schemaName)
			return err
		}
	}

	for _, child := range sc.transparentPath {
		if err := child.RunCloseHooks(); err != nil {
			return err
		}
	}
	return nil
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

func childSourceLocation(in *sourcedef_j5pb.SourceLocation, name string, hint SourceLocation) *sourcedef_j5pb.SourceLocation {
	if in.Children == nil {
		in.Children = map[string]*sourcedef_j5pb.SourceLocation{}
	}
	if _, ok := in.Children[name]; !ok {
		in.Children[name] = &sourcedef_j5pb.SourceLocation{
			StartLine:   int32(hint.Start.Line),
			StartColumn: int32(hint.Start.Column),
			EndLine:     int32(hint.End.Line),
			EndColumn:   int32(hint.End.Column),
		}
	}
	return in.Children[name]
}

func (sc *containerField) errPosition() errpos.Position {
	return errpos.Position{
		Start: errpos.Point{
			Line:   int(sc.location.StartLine),
			Column: int(sc.location.StartColumn),
		},
		End: errpos.Point{
			Line:   int(sc.location.EndLine),
			Column: int(sc.location.EndColumn),
		},
	}
}

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
	var fieldWithContainer j5reflect.ContainerField
	if array, ok := val.AsArrayOfContainer(); ok {
		element, idx := array.NewContainerElement()
		protoPath = append(protoPath, element.ProtoPath()...)
		schemaPath = append(schemaPath, name, fmt.Sprintf("[%d]", idx))
		fieldWithContainer = element

	} else if container, ok := val.AsContainer(); ok {
		fieldWithContainer = container
		schemaPath = append(schemaPath, name)
	} else {
		return nil, &WalkPathError{
			Field:  name,
			Type:   NodeNotContainer,
			Schema: container.SchemaName(),
		}
	}

	//	fmt.Printf("WALK PATH %s\n", path)
	//	fmt.Printf("  Schema: %s\n", fieldWithContainer.SchemaName())
	//	fmt.Printf("  Path:   %s\n", strings.Join(schemaPath, "."))
	//	fmt.Printf("  Proto:  %s\n", strings.Join(protoPath, "."))

	sourceLocation := container.location
	for _, elem := range protoPath {
		sourceLocation = childSourceLocation(sourceLocation, elem, loc)
	}
	childContainer := &containerField{
		path:       schemaPath,
		schemaName: fieldWithContainer.SchemaName(),
		container:  fieldWithContainer,
		field:      fieldWithContainer,
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
