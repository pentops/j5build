package j5parse

import (
	"fmt"

	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/lib/j5reflect"
)

var globalDefs = map[string]*schema.BlockSpec{
	"j5.schema.v1.Ref": {
		Name: &schema.Tag{
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
	},
	"j5.schema.v1.KeyFormat": {
		TypeSelect: &schema.Tag{
			Path: []string{},
		},
	},
	"j5.schema.v1.ArrayField": {
		Qualifier: &schema.Tag{
			Path:    []string{"items"},
			IsBlock: true,
		},
	},
	"j5.schema.v1.KeyField": {
		Qualifier: &schema.Tag{
			Path:    []string{"format"},
			IsBlock: true,
		},
	},
	"j5.schema.v1.IntegerField": {
		Qualifier: &schema.Tag{
			Path: []string{"format"},
		},
	},
	"j5.schema.v1.Field": {
		TypeSelect: &schema.Tag{
			Path: []string{}, // self
		},
	},
	"j5.schema.v1.ObjectProperty": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		TypeSelect: &schema.Tag{
			Path: []string{"schema"},
		},
		RunAfter: combineHooks(
			nestedObjectNameHook,
			autoNumberPropertyHook,
		),
	},
	"j5.schema.v1.ObjectField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:        schema.PathSpec{"object", "properties"},
				IsContainer: true,
			},
		},
	},
	"j5.schema.v1.OneofField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:        schema.PathSpec{"oneof", "properties"},
				IsContainer: true,
			},
		},
	},
	"j5.schema.v1.EnumField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
	},
	"j5.sourcedef.v1.Object": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		RunAfter: combineHooks(objectHook),
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
			"object": {
				Path:         schema.PathSpec{"schemas", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.schema.v1.Object": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
		RunAfter: combineHooks(objectHook),
	},
	"j5.sourcedef.v1.Oneof": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		RunAfter: combineHooks(objectHook),
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.schema.v1.Enum_Option": {
		RunAfter: combineHooks(autoNumberHook),
	},
	"j5.schema.v1.Enum": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		RunAfter: combineHooks(enumHook),
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:         schema.PathSpec{"options"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.sourcedef.v1.Entity": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		//RunAfter: entityHook,
		Children: map[string]schema.ChildSpec{
			"key": {
				Path:         schema.PathSpec{"keys"},
				IsContainer:  true,
				IsCollection: true,
			},
			"data": {
				Path:        schema.PathSpec{"data"},
				IsContainer: true,
			},
			"status": {
				Path:         schema.PathSpec{"status"},
				IsCollection: true,
				IsScalar:     true,
			},
			"event": {
				Path:         schema.PathSpec{"events"},
				IsContainer:  true,
				IsCollection: true,
			},
			"object": {
				Path:         schema.PathSpec{"schemas", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
			"enum": {
				Path:         schema.PathSpec{"schemas", "enum"},
				IsContainer:  true,
				IsCollection: true,
			},
			"oneof": {
				Path:         schema.PathSpec{"schemas", "oneof"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.sourcedef.v1.Partial_Field": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		TypeSelect: &schema.Tag{
			Path: []string{"schema"},
		},
	},
	"j5.sourcedef.v1.Partial": {
		Description: []string{"description"},
		TypeSelect: &schema.Tag{
			Path: []string{},
		},
	},

	"j5.sourcedef.v1.SourceFile": {
		Description: []string{"description"},
		//OnlyDefined: true,

		Children: map[string]schema.ChildSpec{
			"object": {
				Path:         schema.PathSpec{"elements", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
			"enum": {
				Path:         schema.PathSpec{"elements", "enum"},
				IsContainer:  true,
				IsCollection: true,
			},
			"oneof": {
				Path:         schema.PathSpec{"elements", "oneof"},
				IsContainer:  true,
				IsCollection: true,
			},
			"entity": {
				Path:         schema.PathSpec{"elements", "entity"},
				IsContainer:  true,
				IsCollection: true,
			},

			"partial": {
				Path: schema.PathSpec{"elements", "partial"},
			},
			//"import": {
			//	Path: schema.PathSpec{"imports"},
			//},

			"version": {
				Path:     schema.PathSpec{"version"},
				IsScalar: true,
			},
		},
	},
}

var J5SchemaSpec = &schema.ConversionSpec{
	GlobalDefs: globalDefs,
}

func init() {
	err := J5SchemaSpec.Validate()
	if err != nil {
		panic(err)
	}
}

type combinedHook []func(j5reflect.ContainerField) error

func (ch combinedHook) RunHook(field j5reflect.ContainerField) error {
	for _, hook := range ch {
		if err := hook(field); err != nil {
			return err
		}
	}
	return nil
}

func combineHooks(hooks ...func(j5reflect.ContainerField) error) schema.BlockHook {
	return combinedHook(hooks)
}

func enumHook(field j5reflect.ContainerField) error {
	return nil
}

func autoNumberHook(field j5reflect.ContainerField) error {
	idx := field.IndexInParent()
	option := CWalk(field)
	err := option.WalkCreate("number").Scalar().SetGoValue(idx + 1)
	return err
}

func nestedObjectNameHook(field j5reflect.ContainerField) error {
	property := CWalk(field)
	// ... so do I want to be Rust or Java?
	obj, ok, err := property.Walk("schema", "object", "object").Maybe()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	objectName := obj.Walk("name").Scalar()
	_, ok, err = objectName.Maybe()
	if err != nil {
		return fmt.Errorf("object name: %w", err)
	}
	if !ok {
		propNameVal, ok, err := property.Walk("name").Scalar().GoValue()
		if err != nil {
			return fmt.Errorf("prop name: %w", err)
		}
		if !ok {
			return fmt.Errorf("property name not set")
		}
		propName := propNameVal.(string)

		if err := obj.WalkCreate("name").Scalar().SetGoValue(strcase.ToCamel(propName)); err != nil {
			return fmt.Errorf("set: %w", err)
		}
	}

	return nil
}

func autoNumberPropertyHook(field j5reflect.ContainerField) error {
	idx := field.IndexInParent()
	prop, err := field.GetOrCreateValue("protoField")
	if err != nil {
		return err
	}
	arrayField, ok := prop.AsArrayOfScalar()
	if !ok {
		return fmt.Errorf("protoField not an ArrayOfScalar: %s / %s %T", prop.TypeName(), prop.FullTypeName(), prop)
	}

	if arrayField.Length() != 0 {
		return nil
	}

	if _, err := arrayField.AppendGoValue(idx + 1); err != nil {
		return err
	}

	return nil
}

/*
func entityHook(field j5reflect.ContainerField) error {

	if err := rangeArray(field, []string{"keys"}, true, fixProtoFields); err != nil {
		return err
	}

	if err := rangeArray(field, []string{"data"}, true, fixProtoFields); err != nil {
		return err
	}

	return nil

}*/

func objectHook(field j5reflect.ContainerField) error {
	return nil
	/*
		return rangeArray(field, []string{"properties"}, true, func(idx int, propertyField j5reflect.ContainerField) error {
			if err := fixProtoFields(idx, propertyField); err != nil {
				return err
			}
			property := CWalk(propertyField)

			// ... so do I want to be Rust or Java?
			obj, ok, err := property.Walk("schema", "object", "object").Maybe()
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}

			objectName := obj.Walk("name").Scalar()
			_, ok, err = objectName.Maybe()
			if err != nil {
				return fmt.Errorf("object name: %w", err)
			}
			if !ok {
				propNameVal, ok, err := property.Walk("name").Scalar().GoValue()
				if err != nil {
					return fmt.Errorf("prop name: %w", err)
				}
				if !ok {
					return fmt.Errorf("property name not set")
				}
				propName := propNameVal.(string)

				if err := obj.WalkCreate("name").Scalar().SetGoValue(strcase.ToCamel(propName)); err != nil {
					return fmt.Errorf("set: %w", err)
				}
			}

			return nil

		})*/
}

type walkInner struct {
	err      error
	notFound bool
}

func (wi *walkInner) fail(e error) {
	wi.err = e
}

func (wi *walkInner) failNotFound(e error) {
	wi.notFound = true
	wi.err = e
}

type CWalker struct {
	field j5reflect.Field
	walkInner
}

func CWalk(field j5reflect.Field) CWalker {
	return CWalker{
		field: field,
	}
}

func (w CWalker) Walk(path ...string) CWalker {
	if w.err != nil {
		return w
	}
	container, ok := w.field.AsContainer()
	if !ok {
		w.fail(fmt.Errorf("CWalk: not containe at %q", path))
		return w
	}
	field, ok, err := reflWalk(container, path, false, false)
	if err != nil {
		w.fail(err)
		return w
	}
	if !ok {
		w.failNotFound(fmt.Errorf("CWalk: not found at %q %q", container.FullTypeName(), path))
		return w
	}

	return CWalker{field: field}
}

func (w CWalker) WalkCreate(path ...string) CWalker {
	if w.err != nil {
		return w
	}
	container, ok := w.field.AsContainer()
	if !ok {
		w.fail(fmt.Errorf("CWalk: not containe at %q", path))
		return w
	}
	field, ok, err := reflWalk(container, path, false, true)
	if err != nil {
		w.fail(err)
		return w
	}
	if !ok {
		w.failNotFound(fmt.Errorf("CWalk Create: not found at %q %q", container.FullTypeName(), path))

		return w
	}
	return CWalker{field: field}
}

func (w CWalker) Scalar() SWalker {
	if w.err != nil {
		w.fail(w.err)
		return SWalker{walkInner: w.walkInner}
	}
	scalar, ok := w.field.AsScalar()
	if !ok {
		w.fail(fmt.Errorf("CWalker: not scalar"))
		return SWalker{walkInner: w.walkInner}
	}
	return SWalker{field: scalar}
}

type SWalker struct {
	field j5reflect.ScalarField
	walkInner
}

func (w SWalker) GoValue() (interface{}, bool, error) {
	if w.err != nil {
		return nil, false, w.err
	}
	if !w.field.IsSet() {
		return nil, false, nil
	}
	val, err := w.field.ToGoValue()
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (w SWalker) SetGoValue(val interface{}) error {
	if w.err != nil {
		return w.err
	}
	return w.field.SetGoValue(val)
}

func (w CWalker) Maybe() (CWalker, bool, error) {
	if w.notFound {
		return CWalker{}, false, nil
	}
	if w.err != nil {
		return CWalker{}, false, w.err
	}
	return w, true, nil
}

func (w CWalker) OK() (CWalker, error) {
	return w, w.err
}

func (w SWalker) OK() (SWalker, error) {
	return w, w.err
}

func (w SWalker) Maybe() (SWalker, bool, error) {
	if w.notFound {
		return SWalker{}, false, nil
	}
	if w.err != nil {
		return SWalker{}, false, w.err
	}
	return w, true, nil
}

func reflWalk(propSet j5reflect.ContainerField, path []string, must bool, create bool) (j5reflect.Field, bool, error) {
	name, rest := path[0], path[1:]
	if !propSet.HasProperty(name) {
		return nil, false, fmt.Errorf("%q: REFLW: invalid property name", name)
	}
	prop, ok, err := propSet.GetValue(name)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		if must {
			return nil, false, fmt.Errorf("%q: REFLW: required property not set", name)
		}
		if create {
			prop, err = propSet.NewValue(name)
			if err != nil {
				return nil, false, fmt.Errorf("%q: REFLW: create: %w", name, err)
			}
		} else {

			return nil, false, nil
		}
	}
	if len(rest) == 0 {
		return prop, true, nil
	}

	propContainer, ok := prop.AsContainer()
	if !ok {
		if must {
			return nil, false, fmt.Errorf("%q: not container", name)
		}
		return nil, false, nil
	}

	field, ok, err := reflWalk(propContainer, rest, must, create)
	if err != nil {
		return nil, false, fmt.Errorf("%q.%w", name, err)
	}
	if !ok {
		if must {
			return nil, false, fmt.Errorf("%q not set", name)
		}
		return nil, false, nil
	}
	return field, true, nil

}
