package gogen

import (
	"fmt"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/patherr"
)

type builder struct {
	fileSet *FileSet
	options Options
	//schemas SchemaResolver
}

func (bb *builder) buildTypeName(currentPackage string, schema *schema_j5pb.Field) (*DataType, error) {

	switch schemaType := schema.Type.(type) {

	case *schema_j5pb.Field_Object:
		var refPackage, refSchema string

		switch linkType := schemaType.Object.Schema.(type) {
		case *schema_j5pb.ObjectField_Ref:
			refPackage = linkType.Ref.Package
			refSchema = linkType.Ref.Schema
		case *schema_j5pb.ObjectField_Object:
			refPackage = currentPackage
			refSchema = linkType.Object.Name

			if err := bb.addObject(currentPackage, linkType.Object); err != nil {
				return nil, fmt.Errorf("referencedType %q.%q: %w", refPackage, refSchema, err)
			}
		default:
			return nil, fmt.Errorf("Unknown object ref type: %T\n", schema)
		}

		objectPackage, err := bb.options.ReferenceGoPackage(refPackage)
		if err != nil {
			return nil, fmt.Errorf("referredType in %q.%q: %w", refPackage, refSchema, err)
		}

		return &DataType{
			Name:      goTypeName(refSchema),
			GoPackage: objectPackage,
			J5Package: refPackage,
			Pointer:   true,
		}, nil

	case *schema_j5pb.Field_Oneof:
		var refPackage, refSchema string

		switch linkType := schemaType.Oneof.Schema.(type) {
		case *schema_j5pb.OneofField_Ref:
			refPackage = linkType.Ref.Package
			refSchema = linkType.Ref.Schema

		case *schema_j5pb.OneofField_Oneof:
			refPackage = currentPackage
			refSchema = linkType.Oneof.Name

			if err := bb.addOneofWrapper(currentPackage, linkType.Oneof); err != nil {
				return nil, fmt.Errorf("referencedType %q.%q: %w", refPackage, refSchema, err)
			}
		default:
			return nil, fmt.Errorf("Unknown object ref type: %T\n", schema)
		}

		objectPackage, err := bb.options.ReferenceGoPackage(refPackage)
		if err != nil {
			return nil, fmt.Errorf("referredType in %q.%q: %w", refPackage, refSchema, err)
		}

		return &DataType{
			Name:      goTypeName(refSchema),
			GoPackage: objectPackage,
			Pointer:   true,
			J5Package: refPackage,
		}, nil

	case *schema_j5pb.Field_Enum:
		// TODO: Something better.
		return &DataType{
			Name:    "string",
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Array:
		itemType, err := bb.buildTypeName(currentPackage, schemaType.Array.Items)
		if err != nil {
			return nil, err
		}

		return &DataType{
			Name:      itemType.Name,
			Pointer:   itemType.Pointer,
			J5Package: itemType.J5Package,
			GoPackage: itemType.GoPackage,
			Slice:     true,
		}, nil

	case *schema_j5pb.Field_Map:
		valueType, err := bb.buildTypeName(currentPackage, schemaType.Map.ItemSchema)
		if err != nil {
			return nil, fmt.Errorf("map value: %w", err)
		}

		return &DataType{
			Name:    fmt.Sprintf("map[string]%s", valueType.Name),
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Any:
		return &DataType{
			Name:    "interface{}",
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_String_:
		item := schemaType.String_
		if item.Format == nil {
			return &DataType{
				Name:    "string",
				Pointer: false,
			}, nil
		}

		switch *item.Format {
		case "uuid", "date", "email", "uri":
			return &DataType{
				Name:    "string",
				Pointer: false,
			}, nil
		case "date-time":
			return &DataType{
				Name:      "Time",
				Pointer:   true,
				GoPackage: "time",
			}, nil
		case "byte":
			return &DataType{
				Name:    "[]byte",
				Pointer: false,
			}, nil
		default:
			return nil, fmt.Errorf("Unknown string format: %s", *item.Format)
		}

	case *schema_j5pb.Field_Bytes:
		return &DataType{
			Name:    "[]byte",
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Date:
		return &DataType{
			Name:    "string",
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Timestamp:
		return &DataType{
			Name:      "Time",
			Pointer:   true,
			GoPackage: "time",
		}, nil

	case *schema_j5pb.Field_Key:
		// TODO: Constrain UUID?
		return &DataType{
			Name:    "string",
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Float:
		return &DataType{
			Name:    goFloatTypes[schemaType.Float.Format],
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Integer:
		return &DataType{
			Name:    goIntTypes[schemaType.Integer.Format],
			Pointer: false,
		}, nil

	case *schema_j5pb.Field_Bool:
		return &DataType{
			Name:    "bool",
			Pointer: false,
		}, nil

	default:
		return nil, fmt.Errorf("Unknown type for Go Gen: %T\n", schemaType)
	}

}

var goFloatTypes = map[schema_j5pb.FloatField_Format]string{
	schema_j5pb.FloatField_FORMAT_FLOAT32: "float32",
	schema_j5pb.FloatField_FORMAT_FLOAT64: "float64",
}

var goIntTypes = map[schema_j5pb.IntegerField_Format]string{
	schema_j5pb.IntegerField_FORMAT_INT32:  "int32",
	schema_j5pb.IntegerField_FORMAT_INT64:  "int64",
	schema_j5pb.IntegerField_FORMAT_UINT32: "uint32",
	schema_j5pb.IntegerField_FORMAT_UINT64: "uint64",
}

func (bb *builder) jsonField(packageName string, property *schema_j5pb.ObjectProperty) (*Field, error) {

	tags := map[string]string{}

	tags["json"] = property.Name
	if !property.Required {
		tags["json"] += ",omitempty"
	}

	dataType, err := bb.buildTypeName(packageName, property.Schema)
	if err != nil {
		return nil, fmt.Errorf("building field %s: %w", property.Name, err)
	}

	if !dataType.Pointer && !dataType.Slice && property.ExplicitlyOptional {
		dataType.Pointer = true
	}

	name := goTypeName(property.Name)
	if obj := property.Schema.GetObject(); obj != nil && obj.Flatten {
		name = ""
		delete(tags, "json")
	}

	return &Field{
		Name:     name,
		DataType: *dataType,
		Tags:     tags,
		Property: property,
	}, nil

}

func (bb *builder) addObject(packageName string, object *schema_j5pb.Object) error {
	gen, err := bb.fileForPackage(packageName)
	if err != nil {
		return err
	}
	if gen == nil {
		return nil
	}

	typeName := goTypeName(object.Name)
	_, ok := gen.types[object.Name]
	if ok {
		return nil
	}

	structType := &Struct{
		Name: typeName,
		Comment: fmt.Sprintf(
			"Proto: %s",
			object.Name,
		),
	}
	gen.types[object.Name] = structType

	for _, property := range object.Properties {
		field, err := bb.jsonField(packageName, property)
		if err != nil {
			return patherr.Wrap(err, object.Name)
		}
		structType.Fields = append(structType.Fields, field)
	}

	return nil
}

func (bb *builder) addOneofWrapper(packageName string, wrapper *schema_j5pb.Oneof) error {
	gen, err := bb.fileForPackage(packageName)
	if err != nil {
		return err
	}
	if gen == nil {
		return nil
	}

	_, ok := gen.types[wrapper.Name]
	if ok {
		return nil
	}

	comment := fmt.Sprintf(
		"Proto Message: %s", wrapper.Name,
	)

	structType := &Struct{
		Name:    goTypeName(wrapper.Name),
		Comment: comment,
	}
	gen.types[wrapper.Name] = structType

	keyMethod := &Function{
		Name: "OneofKey",
		Returns: []*Parameter{{
			DataType: DataType{
				Name:    "string",
				Pointer: false,
			}},
		},
		StringGen: gen.ChildGen(),
	}

	valueMethod := &Function{
		Name: "Type",
		Returns: []*Parameter{{
			DataType: DataType{
				Name:    "interface{}",
				Pointer: false,
			}},
		},
		StringGen: gen.ChildGen(),
	}

	structType.Fields = append(structType.Fields, &Field{
		Name:     "J5TypeKey",
		DataType: DataType{Name: "string", Pointer: false},
		Tags:     map[string]string{"json": "!type,omitempty"},
	})

	for _, property := range wrapper.Properties {
		field, err := bb.jsonField(packageName, property)
		if err != nil {
			return fmt.Errorf("object %s: %w", wrapper.Name, err)
		}
		field.DataType.Pointer = true
		structType.Fields = append(structType.Fields, field)
		keyMethod.P("if s.", field.Name, " != nil {")
		keyMethod.P("  return \"", property.Name, "\"")
		keyMethod.P("}")
		valueMethod.P("if s.", field.Name, " != nil {")
		valueMethod.P("  return s.", field.Name)
		valueMethod.P("}")
	}
	keyMethod.P("return \"\"")
	valueMethod.P("return nil")

	structType.Methods = append(structType.Methods, keyMethod, valueMethod)

	return nil
}
