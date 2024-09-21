package j5client

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/j5/lib/patherr"
)

func APIFromSource(api *source_j5pb.API) (*client_j5pb.API, error) {
	schemaSet, err := j5schema.PackageSetFromSourceAPI(api.Packages)
	if err != nil {
		return nil, fmt.Errorf("package set from source api: %w", err)
	}

	sb := &sourceBuilder{
		schemas: schemaSet,
	}

	apiBase, err := sb.apiBaseFromSource(api)
	if err != nil {
		return nil, fmt.Errorf("api base from desc: %w", err)
	}

	return apiBase.ToJ5Proto()
}

type sourceBuilder struct {
	schemas *j5schema.SchemaSet
}

func (sb *sourceBuilder) apiBaseFromSource(api *source_j5pb.API) (*API, error) {
	apiPkg := &API{
		Packages: []*Package{},
		Metadata: &client_j5pb.Metadata{},
	}

	for _, pkgSource := range api.Packages {
		pkg := &Package{
			Name:     pkgSource.Name,
			Label:    pkgSource.Label,
			Indirect: pkgSource.Indirect,
		}
		apiPkg.Packages = append(apiPkg.Packages, pkg)

		entities := map[string]*StateEntity{}
		schemaPackage, ok := sb.schemas.Packages[pkg.Name]
		if ok {
			var err error
			entities, err = sb.entitiesFromSource(pkg, schemaPackage)
			if err != nil {
				return nil, patherr.Wrap(err, pkg.Name)
			}

			for _, entity := range entities {
				pkg.StateEntities = append(pkg.StateEntities, entity)
			}
		}

		for _, subPkg := range pkgSource.SubPackages {
			sub := &subPackage{
				Package: pkg,
				Name:    subPkg.Name,
			}
			for _, serviceSrc := range subPkg.Services {
				service, err := sb.serviceFromSource(sub, serviceSrc)
				if err != nil {
					return nil, patherr.Wrap(err, pkg.Name, serviceSrc.Name)
				}

				if serviceSrc.Type != nil {
					switch st := serviceSrc.Type.Type.(type) {
					case *source_j5pb.ServiceType_StateEntityCommand_:
						entity, err := getEntity(sub, entities, st.StateEntityCommand.Entity)
						if err != nil {
							return nil, fmt.Errorf("state entity command: %w", err)
						}

						entity.Commands = append(entity.Commands, service)
						continue
					case *source_j5pb.ServiceType_StateEntityQuery_:
						entity, err := getEntity(sub, entities, st.StateEntityQuery.Entity)
						if err != nil {
							return nil, fmt.Errorf("state entity command: %w", err)
						}
						if entity.Query != nil {
							return nil, fmt.Errorf("duplicate query service for entity %q", entity.Name)
						}
						entity.Query = service

						continue
					}
				}

				pkg.Services = append(pkg.Services, service)
			}

		}

	}

	return apiPkg, nil
}

func getEntity(inPackage *subPackage, entities map[string]*StateEntity, name string) (*StateEntity, error) {
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		if parts[0] != inPackage.Package.Name {
			return nil, fmt.Errorf("state entity %q not in package %q", name, inPackage.Package.Name)
		}
		name = parts[1]
	} else if len(parts) != 1 {
		return nil, fmt.Errorf("invalid state entity name %q", name)
	}

	entity, ok := entities[name]
	if !ok {
		return nil, fmt.Errorf("unknown entity %q", name)
	}
	return entity, nil
}

type subPackage struct {
	Package *Package
	Name    string
}

func (sp *subPackage) FullName() string {
	return fmt.Sprintf("%s.%s", sp.Package.Name, sp.Name)
}

func (sb *sourceBuilder) entitiesFromSource(pkg *Package, schemaPackage *j5schema.Package) (map[string]*StateEntity, error) {
	found := map[string]*StateEntity{}

	for _, schema := range schemaPackage.Schemas {
		if schema.To == nil {
			continue
		}
		obj, ok := schema.To.(*j5schema.ObjectSchema)
		if !ok {
			continue
		}
		if obj.Entity == nil {
			continue
		}

		entity, ok := found[obj.Entity.Entity]
		if !ok {
			entity = &StateEntity{
				Package: pkg,
				Name:    obj.Entity.Entity,
			}
			found[obj.Entity.Entity] = entity
		}

		switch obj.Entity.Part {
		case schema_j5pb.EntityPart_KEYS:
			entity.KeysSchema = obj
		case schema_j5pb.EntityPart_STATE:
			entity.StateSchema = obj
		case schema_j5pb.EntityPart_EVENT:
			entity.EventSchema = obj
		case schema_j5pb.EntityPart_DATA:
			// ignore
		default:
			return nil, fmt.Errorf("unknown entity part %q", obj.Entity.Part)
		}
	}

	for _, entity := range found {
		if entity.KeysSchema == nil || entity.EventSchema == nil || entity.StateSchema == nil {
			return nil, fmt.Errorf(
				"missing schema for entity %q: Keys: %v Event %v State %v",
				entity.Name,
				schemaDescForEntity(entity.KeysSchema),
				schemaDescForEntity(entity.EventSchema),
				schemaDescForEntity(entity.StateSchema),
			)
		}

	}
	return found, nil
}

func schemaDescForEntity(schema *j5schema.ObjectSchema) string {
	if schema == nil {
		return "<missing>"
	}
	return schema.FullName()
}

func (sb *sourceBuilder) serviceFromSource(pkg *subPackage, src *source_j5pb.Service) (*Service, error) {

	service := &Service{
		Package:     pkg.Package,
		Name:        src.Name,
		Methods:     make([]*Method, len(src.Methods)),
		defaultAuth: src.DefaultAuth,
	}

	for idx, src := range src.Methods {
		method, err := sb.methodFromSource(pkg, service, src)
		if err != nil {
			return nil, patherr.Wrap(err, src.Name)
		}
		service.Methods[idx] = method
	}

	return service, nil
}

func (sb *sourceBuilder) methodFromSource(pkg *subPackage, service *Service, src *source_j5pb.Method) (*Method, error) {

	requestSchema, err := sb.schemas.SchemaByName(pkg.FullName(), src.RequestSchema)
	if err != nil {
		return nil, patherr.Wrap(err, "request")
	}
	requestObject, ok := requestSchema.(*j5schema.ObjectSchema)
	if !ok {
		return nil, fmt.Errorf("request schema is not an object")
	}

	method := &Method{
		Service:        service,
		GRPCMethodName: src.Name,
		HTTPPath:       src.HttpPath,
		HTTPMethod:     src.HttpMethod,
		HasBody:        src.HttpMethod != client_j5pb.HTTPMethod_GET,
		MethodType:     src.MethodType,
	}

	if src.Auth != nil {
		method.Auth = src.Auth
	} else if service.defaultAuth != nil {
		method.Auth = service.defaultAuth
	}

	if src.ResponseSchema == "HttpBody" {
		method.RawResponse = true
	} else {
		response, err := sb.schemas.SchemaByName(pkg.FullName(), src.ResponseSchema)
		if err != nil {
			return nil, patherr.Wrap(err, "response")
		}
		responseObject, ok := response.(*j5schema.ObjectSchema)
		if !ok {
			return nil, fmt.Errorf("response schema is not an object")
		}
		method.ResponseBody = responseObject
	}

	if err := method.fillRequest(requestObject); err != nil {
		return nil, fmt.Errorf("fill request: %w", err)
	}

	return method, nil
}

func (mm *Method) fillRequest(requestObject *j5schema.ObjectSchema) error {

	pathParameterNames := map[string]struct{}{}
	pathParts := strings.Split(mm.HTTPPath, "/")
	for _, part := range pathParts {
		if !strings.HasPrefix(part, ":") {
			continue
		}
		fieldName := strings.TrimPrefix(part, ":")
		pathParameterNames[fieldName] = struct{}{}
	}

	pathProperties := make([]*j5schema.ObjectProperty, 0)
	bodyProperties := make([]*j5schema.ObjectProperty, 0)

	isQueryRequest := false

	for _, prop := range requestObject.Properties {
		if propObj, ok := prop.Schema.(*j5schema.ObjectField); ok {
			ref := propObj.Ref
			if ref != nil {
				if ref.Package.Name == "j5.list.v1" && ref.Schema == "QueryRequest" {
					isQueryRequest = true
				}
			}
		}
		_, isPath := pathParameterNames[prop.JSONName]
		if isPath {
			prop.Required = true
			pathProperties = append(pathProperties, prop)
		} else {
			bodyProperties = append(bodyProperties, prop)
		}
	}

	request := &Request{
		PathParameters: pathProperties,
	}

	if mm.HasBody {
		request.Body = requestObject.Clone()
		request.Body.Properties = bodyProperties
	} else {
		request.QueryParameters = bodyProperties
	}

	responseSchema := mm.ResponseBody

	if isQueryRequest {
		listRequest, err := buildListRequest(responseSchema)
		if err != nil {
			return fmt.Errorf("build list request: %w", err)
		}
		request.List = listRequest
	}

	mm.Request = request

	return nil
}
