package gogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/patherr"
)

type builder struct {
	fileSet *FileSet
	options Options
}

// fileForPackage returns the file for the given package name, creating if
// required. Returns nil when the package should not be generated (i.e. outside
// of the generate prefix, a reference to externally hosted code)
func (bb *builder) fileForPackage(grpcPackageName string) (*GeneratedFile, error) {
	objectPackage, err := bb.options.ReferenceGoPackage(grpcPackageName)
	if err != nil {
		return nil, fmt.Errorf("object package name '%s': %w", grpcPackageName, err)
	}
	return bb.fileSet.File(objectPackage, filepath.Base(objectPackage))
}

func (bb *builder) addPackage(j5Package *client_j5pb.Package) error {

	for _, service := range j5Package.Services {
		if err := bb.addService(j5Package, service); err != nil {
			return patherr.Wrap(err, "service", service.Name)

		}
	}
	for _, entity := range j5Package.StateEntities {
		if err := bb.addEntity(j5Package, entity); err != nil {
			return patherr.Wrap(err, "entity", entity.Name)
		}
	}

	for name, schema := range j5Package.Schemas {
		switch schemaType := schema.Type.(type) {
		case *schema_j5pb.RootSchema_Enum:
			if err := bb.addEnum(j5Package.Name, schemaType.Enum); err != nil {
				return patherr.Wrap(err, "schemas", name)
			}

		case *schema_j5pb.RootSchema_Object:
			if err := bb.addObject(j5Package.Name, schemaType.Object); err != nil {
				return patherr.Wrap(err, "schemas", name)
			}

		case *schema_j5pb.RootSchema_Oneof:
			if err := bb.addOneofWrapper(j5Package.Name, schemaType.Oneof); err != nil {
				return patherr.Wrap(err, "schemas", name)
			}
		default:
			return fmt.Errorf("unknown schema type %T", schemaType)

		}
	}

	return nil

}

func (bb *builder) addEntity(j5Package *client_j5pb.Package, entity *client_j5pb.StateEntity) error {
	if entity.QueryService != nil {
		if err := bb.addService(j5Package, entity.QueryService); err != nil {
			return err
		}
	}

	for _, command := range entity.CommandServices {
		if err := bb.addService(j5Package, command); err != nil {
			return err
		}
	}

	return nil
}

func (bb *builder) addService(j5Package *client_j5pb.Package, service *client_j5pb.Service) error {

	for _, method := range service.Methods {
		if err := bb.addMethod(j5Package.Name, service.Name, method); err != nil {
			return patherr.Wrap(err, method.Name)
		}
	}

	return nil
}

func (bb *builder) addMethod(packageName string, serviceName string, operation *client_j5pb.Method) error {

	gen, err := bb.fileForPackage(packageName)
	if err != nil {
		return err
	}

	gen.EnsureInterface(&Interface{
		Name: "Requester",
		Methods: []*Function{{
			Name: "Request",
			Parameters: []*Parameter{{
				Name: "ctx",
				DataType: DataType{
					GoPackage: "context",
					Name:      "Context",
				},
			}, {
				Name:     "method",
				DataType: DataType{Name: "string"},
			}, {
				Name:     "path",
				DataType: DataType{Name: "string"},
			}, {
				Name:     "body",
				DataType: DataType{Name: "interface{}"},
			}, {
				Name:     "response",
				DataType: DataType{Name: "interface{}"},
			}},
			Returns: []*Parameter{{
				DataType: DataType{
					Name: "error",
				}},
			},
		}},
	})

	service := gen.Service(serviceName)

	responseType := fmt.Sprintf("%sResponse", operation.Name)

	req, err := bb.prepareRequestObject(packageName, operation)
	if err != nil {
		return err
	}
	{

		if err := gen.AddStruct(req.Request); err != nil {
			return err
		}

		if req.pageRequestField != nil {
			if err := bb.addPaginationMethod(gen, req); err != nil {
				return err
			}
		}

		requestMethod := &Function{
			Name: operation.Name,
			Parameters: []*Parameter{{
				Name: "ctx",
				DataType: DataType{
					GoPackage: "context",
					Name:      "Context",
				},
			}, {
				Name: "req",
				DataType: DataType{
					Name:    req.Request.Name,
					Pointer: true,
				},
			}},
			Returns: []*Parameter{{
				DataType: DataType{
					Name:    responseType,
					Pointer: true,
				},
			}, {
				DataType: DataType{
					Name: "error",
				},
			}},
			StringGen: gen.ChildGen(),
		}

		requestMethod.P("  pathParts := make([]string, ", len(req.path), ")")
		for idx, param := range req.path {
			if param.Field == nil {
				requestMethod.P("  pathParts[", idx, "] = ", quoteString(param.String))
				continue
			}

			switch fieldType := param.Field.Property.Schema.Type.(type) {
			case *schema_j5pb.Field_String_, *schema_j5pb.Field_Key, *schema_j5pb.Field_Date:

			default:
				// Only string-like is supported for now
				return fmt.Errorf("unsupported path parameter type %T", fieldType)
			}

			if param.Field.DataType.Pointer {
				requestMethod.P("  if req.", param.Field.Name, " == nil || *req.", param.Field.Name, " == \"\" {")
				requestMethod.P("    return nil, ", requestMethod.ImportPath("errors"), ".New(", quoteString(fmt.Sprintf("required field %q not set", param.Field.Name)), ")")
				requestMethod.P("  }")
				requestMethod.P("  pathParts[", idx, "] = *req.", param.Field.Name)
			} else {
				requestMethod.P("  if req.", param.Field.Name, " == \"\" {")
				requestMethod.P("    return nil, ", requestMethod.ImportPath("errors"), ".New(", quoteString(fmt.Sprintf("required field %q not set", param.Field.Name)), ")")
				requestMethod.P("  }")
				requestMethod.P("  pathParts[", idx, "] = req.", param.Field.Name)
			}
		}
		requestMethod.P("  path := ", requestMethod.ImportPath("strings"), ".Join(pathParts, \"/\")")

		if operation.HttpMethod == client_j5pb.HTTPMethod_GET {
			if err := bb.addQueryMethod(gen, req); err != nil {
				return err
			}
			requestMethod.P("  if query, err := req.QueryParameters(); err != nil {")
			requestMethod.P("    return nil, err")
			requestMethod.P("  } else if len(query) > 0 {")
			requestMethod.P("    path += \"?\" + query.Encode()")
			requestMethod.P("  }")
		}
		requestMethod.P("  resp := &", responseType, "{}")
		requestMethod.P("  err := s.Request(ctx, \"", operation.HttpMethod.ShortString(), "\", path, req, resp)")
		requestMethod.P("  if err != nil {")
		requestMethod.P("    return nil, err")
		requestMethod.P("  }")

		requestMethod.P("  return resp, nil")

		service.Methods = append(service.Methods, requestMethod)
	}

	{

		responseStruct := &Struct{
			Name: responseType,
		}

		if err := gen.AddStruct(responseStruct); err != nil {
			return err
		}

		var pageResponseField *Field

		sliceFields := make([]*Field, 0)
		responseSchema := operation.ResponseBody
		if responseSchema != nil {

			for _, property := range responseSchema.Properties {
				field, err := bb.jsonField(packageName, property)
				if err != nil {
					return fmt.Errorf("%s.ResponseBody: %w", operation.Name, err)
				}
				responseStruct.Fields = append(responseStruct.Fields, field)
				if field.DataType.J5Package == J5ListPackage && field.DataType.Name == J5PageResponse {
					pageResponseField = field
				} else if field.DataType.Slice {
					sliceFields = append(sliceFields, field)
				}
			}
		}

		if pageResponseField != nil {
			setter := &Function{
				Name: "GetPageToken",
				Returns: []*Parameter{{
					DataType: DataType{
						Name:    "string",
						Pointer: true,
					}},
				},
				StringGen: gen.ChildGen(),
			}
			setter.P("if s.", pageResponseField.Name, " == nil {")
			setter.P("  return nil")
			setter.P("}")
			setter.P("return s.", pageResponseField.Name, ".NextToken")
			responseStruct.Methods = append(responseStruct.Methods, setter)

			// Special case for list responses
			if len(sliceFields) == 1 {
				field := sliceFields[0]
				setter := &Function{
					Name: "GetItems",
					Returns: []*Parameter{{
						DataType: field.DataType.AsSlice(),
					}},
					StringGen: gen.ChildGen(),
				}
				setter.P("return s.", field.Name)
				responseStruct.Methods = append(responseStruct.Methods, setter)
			}
		}
	}

	return nil

}

type builtRequest struct {
	Request          *Struct
	pageRequestField *Field
	path             []PathPart
}

type PathPart struct {
	String string // the string name, being :field or just a plain string when field is nil
	Field  *Field
}

const (
	J5ListPackage  = "j5.list.v1"
	J5PageRequest  = "PageRequest"
	J5PageResponse = "PageResponse"
)

func (bb *builder) prepareRequestObject(currentPackage string, operation *client_j5pb.Method) (*builtRequest, error) {
	requestType := fmt.Sprintf("%sRequest", operation.Name)

	requestStruct := &Struct{
		Name: requestType,
	}

	req := &builtRequest{
		Request: requestStruct,
	}

	fields := map[string]*Field{}

	if operation.Request.Body != nil {
		for _, property := range operation.Request.Body.Properties {
			field, err := bb.jsonField(currentPackage, property)
			if err != nil {
				return nil, err
			}
			fields[property.Name] = field
			requestStruct.Fields = append(requestStruct.Fields, field)

		}
	}
	for _, property := range operation.Request.PathParameters {
		field, err := bb.jsonField(currentPackage, property)
		if err != nil {
			return nil, err
		}
		field.Tags["json"] = "-"
		field.Tags["path"] = field.Property.Name
		fields[property.Name] = field
		requestStruct.Fields = append(requestStruct.Fields, field)
	}
	for _, property := range operation.Request.QueryParameters {
		field, err := bb.jsonField(currentPackage, property)
		if err != nil {
			return nil, err
		}
		field.Tags["json"] = "-"
		field.Tags["query"] = field.Property.Name
		fields[property.Name] = field
		requestStruct.Fields = append(requestStruct.Fields, field)
	}

	for _, field := range fields {
		if field.DataType.J5Package == J5ListPackage && field.DataType.Name == J5PageRequest {
			req.pageRequestField = field
		}
	}

	pathParts := strings.Split(operation.HttpPath, "/")
	req.path = make([]PathPart, len(pathParts))
	for idx, part := range pathParts {
		part := part
		req.path[idx].String = part
		if len(part) == 0 || part[0] != ':' {
			continue
		}
		name := part[1:]

		field, ok := fields[name]
		if !ok {
			return nil, fmt.Errorf("path parameter %q not found in request object %s", name, requestType)
		}

		req.path[idx].Field = field
	}

	return req, nil
}

func (bb *builder) addPaginationMethod(gen *GeneratedFile, req *builtRequest) error {

	field := req.pageRequestField
	if field.DataType.J5Package != J5ListPackage || field.DataType.Name != "PageRequest" {
		return fmt.Errorf("invalid page request field %q %q", field.DataType.J5Package, field.DataType.Name)
	}

	setter := &Function{
		Name:     "SetPageToken",
		TakesPtr: true,
		Parameters: []*Parameter{{
			Name: "pageToken",
			DataType: DataType{
				Name:    "string",
				Pointer: false,
			}},
		},
		StringGen: gen.ChildGen(),
	}
	setter.P("if s.", field.Name, " == nil {")
	setter.P("  s.", field.Name, " = ", field.DataType.Addr(), "{}")
	setter.P("}")
	setter.P("s.", field.Name, ".Token = &pageToken")

	req.Request.Methods = append(req.Request.Methods, setter)

	return nil
}

func (bb *builder) addQueryMethod(gen *GeneratedFile, req *builtRequest) error {
	queryMethod := &Function{
		Name:       "QueryParameters",
		Parameters: []*Parameter{},
		Returns: []*Parameter{{
			DataType: DataType{
				Name:      "Values",
				GoPackage: "net/url",
			},
		}, {
			DataType: DataType{
				Name: "error",
			},
		}},
		StringGen: gen.ChildGen(),
	}

	queryMethod.P("  values := ", DataType{GoPackage: "net/url", Name: "Values"}, "{}")

	for _, field := range req.Request.Fields {
		if _, ok := field.Tags["query"]; !ok {
			continue
		}

		switch fieldType := field.Property.Schema.Type.(type) {

		case *schema_j5pb.Field_Bool:
			// TODO: we need to figure out more around bools with optional and omitemtpy.
			// For the moment it at least appears like bools are implicitly optional but not a pointer.
			accessor := "s." + field.Name
			accessor = queryMethod.ImportPath("fmt") + ".Sprintf(\"%v\", " + accessor + ")"
			queryMethod.P("  values.Set(\"", field.Property.Name, "\", ", accessor, ")")

		case *schema_j5pb.Field_String_,
			*schema_j5pb.Field_Key,
			*schema_j5pb.Field_Timestamp:
			accessor := "s." + field.Name
			if !field.Property.Required {
				accessor = "*s." + field.Name
			}

			switch fieldType.(type) {
			case *schema_j5pb.Field_Timestamp:
				accessor = accessor + ".String()"

			}

			if field.Property.Required {
				queryMethod.P("  values.Set(\"", field.Property.Name, "\", ", accessor, ")")
			} else {
				queryMethod.P("  if s.", field.Name, " != nil {")
				queryMethod.P("    values.Set(\"", field.Property.Name, "\", ", accessor, ")")
				queryMethod.P("  }")
			}

		case *schema_j5pb.Field_Object, *schema_j5pb.Field_Oneof, *schema_j5pb.Field_Map, *schema_j5pb.Field_Array:
			// include as JSON
			queryMethod.P("  if s.", field.Name, " != nil {")
			queryMethod.P("    bb, err := ", DataType{GoPackage: "encoding/json", Name: "Marshal"}, "(s.", field.Name, ")")
			queryMethod.P("    if err != nil {")
			queryMethod.P("      return nil, err")
			queryMethod.P("    }")
			queryMethod.P("    values.Set(\"", field.Property.Name, "\", string(bb))")
			queryMethod.P("  }")

		case *schema_j5pb.Field_Enum:

			if field.Property.Required {
				queryMethod.P("  values.Set(\"", field.Property.Name, "\", string(s.", field.Name, "))")
			} else {
				queryMethod.P("  if s.", field.Name, " != nil {")
				queryMethod.P("    values.Set(\"", field.Property.Name, "\", string(*s.", field.Name, "))")
				queryMethod.P("  }")
			}

		default:
			queryMethod.P(" // Skipping query parameter ", field.Property.Name)
			//queryMethod.P("    values.Set(\"", parameter.Name, "\", fmt.Sprintf(\"%v\", *s.", GoName(parameter.Name), "))")
			return fmt.Errorf("unsupported type for query %T", fieldType)

		}
	}

	queryMethod.P("  return values, nil")

	req.Request.Methods = append(req.Request.Methods, queryMethod)
	return nil
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
		var refPackage, refSchema string

		switch linkType := schemaType.Enum.Schema.(type) {
		case *schema_j5pb.EnumField_Ref:
			refPackage = linkType.Ref.Package
			refSchema = linkType.Ref.Schema

		case *schema_j5pb.EnumField_Enum:
			refPackage = currentPackage
			refSchema = linkType.Enum.Name

			if err := bb.addEnum(currentPackage, linkType.Enum); err != nil {
				return nil, fmt.Errorf("referencedType %q.%q: %w", refPackage, refSchema, err)
			}
		default:
			return nil, fmt.Errorf("Unknown enum ref type: %T\n", schema)
		}

		enumPackage, err := bb.options.ReferenceGoPackage(refPackage)
		if err != nil {
			return nil, fmt.Errorf("referredType in %q.%q: %w", refPackage, refSchema, err)
		}

		return &DataType{
			Name:      goTypeName(refSchema),
			GoPackage: enumPackage,
			Pointer:   false,
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
			Name:      valueType.Name,
			Pointer:   valueType.Pointer,
			J5Package: valueType.J5Package,
			GoPackage: valueType.GoPackage,
			Map:       true,
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
		case "uuid", "date", "email", "uri", "id62":
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

	case *schema_j5pb.Field_Decimal:
		return &DataType{
			Name:    "string",
			Pointer: false,
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

	if dataType.Name == "int64" || dataType.Name == "uint64" {
		tags["json"] += ",string"
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

func (bb *builder) addEnum(packageName string, enum *schema_j5pb.Enum) error {
	gen, err := bb.fileForPackage(packageName)
	if err != nil {
		return err
	}
	if gen == nil {
		return nil
	}

	enumType := &Enum{
		Name: goTypeName(enum.Name),
		Comment: fmt.Sprintf(
			"Proto Enum: %s.%s",
			packageName, enum.Name,
		),
	}

	for _, value := range enum.Options {
		enumType.Values = append(enumType.Values, &EnumValue{
			Name:    goTypeName(value.Name),
			Comment: value.Description,
		})
	}

	if err := gen.AddEnum(enumType); err != nil {
		return err
	}

	return nil
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

	structType := &Struct{
		Name: typeName,
		Comment: fmt.Sprintf(
			"Proto: %s",
			object.Name,
		),
	}

	for _, property := range object.Properties {
		field, err := bb.jsonField(packageName, property)
		if err != nil {
			return patherr.Wrap(err, object.Name)
		}
		structType.Fields = append(structType.Fields, field)
	}

	if err := gen.AddStruct(structType); err != nil {
		return err
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

	comment := fmt.Sprintf(
		"Proto Oneof: %s.%s", packageName, wrapper.Name,
	)

	structType := &Struct{
		Name:    goTypeName(wrapper.Name),
		Comment: comment,
	}
	if err := gen.AddStruct(structType); err != nil {
		return err
	}

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
