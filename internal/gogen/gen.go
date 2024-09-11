package gogen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/patherr"
)

type Options struct {
	TrimPackagePrefix   string
	FilterPackagePrefix string
	GoPackagePrefix     string
}

// ReferenceGoPackage returns the go package for the given proto package. It may
// be within the generated code, or a reference to an external package.
func (o Options) ReferenceGoPackage(pkg string) (string, error) {
	if pkg == "" {
		return "", fmt.Errorf("empty package")
	}

	if !strings.HasPrefix(pkg, o.FilterPackagePrefix) {
		return "", fmt.Errorf("package %s not in prefix %s", pkg, o.FilterPackagePrefix)
	}

	if o.TrimPackagePrefix != "" {
		pkg = strings.TrimPrefix(pkg, o.TrimPackagePrefix)
	}

	pkg = strings.TrimSuffix(pkg, ".service")
	pkg = strings.TrimSuffix(pkg, ".topic")
	pkg = strings.TrimSuffix(pkg, ".sandbox")

	parts := strings.Split(pkg, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid package: %s", pkg)
	}
	nextName := parts[len(parts)-2]
	parts = append(parts, nextName)

	pkg = strings.Join(parts, "/")

	pkg = path.Join(o.GoPackagePrefix, pkg)
	return pkg, nil
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

type FileWriter interface {
	WriteFile(name string, data []byte) error
}

type DirFileWriter string

func (fw DirFileWriter) WriteFile(relPath string, data []byte) error {
	fullPath := filepath.Join(string(fw), relPath)
	dirName := filepath.Dir(fullPath)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return fmt.Errorf("mkdirall for %s: %w", fullPath, err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("writefile for %s: %w", fullPath, err)
	}
	return nil
}

func WriteGoCode(api *client_j5pb.API, output FileWriter, options Options) error {

	/*
		reflect, err := j5schema.APIFromDesc(api)
		if err != nil {
			return err
		}*/
	fileSet := NewFileSet(options.GoPackagePrefix)

	bb := &builder{
		fileSet: fileSet,
		options: options,
	}

	for _, j5Package := range api.Packages {
		if err := bb.addPackage(j5Package); err != nil {
			return patherr.Wrap(err, j5Package.Name)
		}
	}

	return fileSet.WriteAll(output)
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
			// nothing to do
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

		if _, ok := gen.types[req.Request.Name]; ok {
			return fmt.Errorf("request type %q already exists", req.Request.Name)
		}
		gen.types[req.Request.Name] = req.Request

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
			case *schema_j5pb.Field_String_, *schema_j5pb.Field_Key:

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

		if _, ok := gen.types[responseType]; ok {
			return fmt.Errorf("response type %q already exists", responseType)
		}

		gen.types[responseType] = responseStruct

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
				queryMethod.P("  values.Set(\"", field.Property.Name, "\", s.", field.Name, ")")
			} else {
				queryMethod.P("  if s.", field.Name, " != nil {")
				queryMethod.P("    values.Set(\"", field.Property.Name, "\", *s.", field.Name, ")")
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
