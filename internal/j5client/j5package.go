package j5client

import (
	"fmt"

	"github.com/pentops/j5/gen/j5/auth/v1/auth_j5pb"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/patherr"
	"github.com/pentops/j5build/internal/structure"
	"github.com/pentops/j5/lib/j5schema"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type API struct {
	Packages []*Package
	Metadata *client_j5pb.Metadata
}

type schemaSource interface {
	AnonymousObjectFromSchema(packageName string, schema *schema_j5pb.Object) (*j5schema.ObjectSchema, error)
}

func (api *API) ToJ5Proto() (*client_j5pb.API, error) {

	// preserves order
	packages := make([]*client_j5pb.Package, 0, len(api.Packages))
	packageMap := map[string]*client_j5pb.Package{}

	for _, pkg := range api.Packages {
		apiPkg, err := pkg.ToJ5Proto()
		if err != nil {
			return nil, fmt.Errorf("package %q: %w", pkg.Name, err)
		}

		packages = append(packages, apiPkg)
		packageMap[pkg.Name] = apiPkg

	}

	referencedSchemas, err := collectPackageRefs(api)
	if err != nil {
		return nil, fmt.Errorf("collecting package refs: %w", err)
	}

	for _, schema := range referencedSchemas {
		pkg, subPkg, err := structure.SplitPackageParts(schema.inPackage)
		if err != nil {
			return nil, fmt.Errorf("splitting package %q: %w", schema.inPackage, err)
		}
		schemaName := schema.name
		if subPkg != nil {
			schemaName = fmt.Sprintf("%s.%s", *subPkg, schemaName)
		}

		apiPkg, ok := packageMap[pkg]
		if ok {
			apiPkg.Schemas[schemaName] = schema.schema
			continue
		}
		refPackage := &client_j5pb.Package{
			Name: schema.inPackage,
			Schemas: map[string]*schema_j5pb.RootSchema{
				schemaName: schema.schema,
			},
		}
		packageMap[pkg] = refPackage
		packages = append(packages, refPackage)
	}
	return &client_j5pb.API{
		Packages: packages,
		Metadata: &client_j5pb.Metadata{
			BuiltAt: timestamppb.Now(),
		},
	}, nil
}

type Package struct {
	Name          string
	Label         string
	Services      []*Service
	StateEntities []*StateEntity
}

func (pkg *Package) ToJ5Proto() (*client_j5pb.Package, error) {

	services := make([]*client_j5pb.Service, 0, len(pkg.Services))
	for _, serviceSrc := range pkg.Services {
		service, err := serviceSrc.ToJ5Proto()
		if err != nil {
			return nil, patherr.Wrap(err, "service", serviceSrc.Name)
		}
		services = append(services, service)
	}

	stateEntities := make([]*client_j5pb.StateEntity, 0, len(pkg.StateEntities))
	for _, entitySrc := range pkg.StateEntities {
		entity, err := entitySrc.ToJ5Proto()
		if err != nil {
			return nil, patherr.Wrap(err, "entity", entitySrc.Name)
		}
		stateEntities = append(stateEntities, entity)

	}

	return &client_j5pb.Package{
		Label:         pkg.Label,
		Name:          pkg.Name,
		Schemas:       map[string]*schema_j5pb.RootSchema{},
		Services:      services,
		StateEntities: stateEntities,
	}, nil

}

type Service struct {
	Package     *Package
	Name        string
	Methods     []*Method
	defaultAuth *auth_j5pb.MethodAuthType
}

func (ss *Service) ToJ5Proto() (*client_j5pb.Service, error) {
	service := &client_j5pb.Service{
		Name:    ss.Name,
		Methods: make([]*client_j5pb.Method, len(ss.Methods)),
	}

	for i, method := range ss.Methods {
		m, err := method.ToJ5Proto()
		if err != nil {
			return nil, err
		}
		service.Methods[i] = m
	}

	return service, nil
}

type SchemaLink interface {
	FullName() string
	Schema() *j5schema.ObjectSchema
	link(schemaSource) error
}

type Request struct {
	Body            *j5schema.ObjectSchema
	PathParameters  []*j5schema.ObjectProperty
	QueryParameters []*j5schema.ObjectProperty

	List *client_j5pb.ListRequest
}

func (rr *Request) ToJ5Proto() *client_j5pb.Method_Request {
	pathParameters := make([]*schema_j5pb.ObjectProperty, 0, len(rr.PathParameters))
	for _, pp := range rr.PathParameters {
		pathParameters = append(pathParameters, pp.ToJ5Proto())
	}

	queryParameters := make([]*schema_j5pb.ObjectProperty, 0, len(rr.QueryParameters))
	for _, qp := range rr.QueryParameters {
		queryParameters = append(queryParameters, qp.ToJ5Proto())
	}

	var body *schema_j5pb.Object
	if rr.Body != nil {
		body = rr.Body.ToJ5Object()
	}

	return &client_j5pb.Method_Request{
		Body:            body,
		PathParameters:  pathParameters,
		QueryParameters: queryParameters,
		List:            rr.List,
	}
}

type Method struct {
	GRPCMethodName string
	HTTPPath       string
	HTTPMethod     client_j5pb.HTTPMethod

	HasBody bool

	Request      *Request
	ResponseBody *j5schema.ObjectSchema
	RawResponse  bool
	Auth         *auth_j5pb.MethodAuthType

	Service *Service
}

func (mm *Method) ToJ5Proto() (*client_j5pb.Method, error) {

	out := &client_j5pb.Method{
		FullGrpcName: fmt.Sprintf("/%s.%s/%s", mm.Service.Package.Name, mm.Service.Name, mm.GRPCMethodName),
		Name:         mm.GRPCMethodName,
		HttpMethod:   mm.HTTPMethod,
		HttpPath:     mm.HTTPPath,
		Request:      mm.Request.ToJ5Proto(),
		Auth:         mm.Auth,
	}
	if mm.ResponseBody != nil {
		out.ResponseBody = mm.ResponseBody.ToJ5Object()
	}
	return out, nil
}

type StateEntity struct {
	Package  *Package // parent
	Name     string
	Commands []*Service
	Query    *Service

	KeysSchema  *j5schema.ObjectSchema
	StateSchema *j5schema.ObjectSchema
	EventSchema *j5schema.ObjectSchema
}

func (entity *StateEntity) ToJ5Proto() (*client_j5pb.StateEntity, error) {

	commands := make([]*client_j5pb.Service, 0, len(entity.Commands))
	for _, command := range entity.Commands {
		service, err := command.ToJ5Proto()
		if err != nil {
			return nil, patherr.Wrap(err, "command", command.Name)
		}
		commands = append(commands, service)
	}

	query := &client_j5pb.Service{}
	if entity.Query != nil {
		var err error
		query, err = entity.Query.ToJ5Proto()
		if err != nil {
			return nil, fmt.Errorf("query %q: %w", entity.Query.Name, err)
		}
	}

	primaryKeys := make([]string, 0)
	for _, prop := range entity.KeysSchema.Properties {
		scalar, ok := prop.Schema.(*j5schema.ScalarSchema)
		if !ok {
			continue
		}
		keyField := scalar.Proto.GetKey()
		if keyField == nil {
			continue
		}

		if keyField.Ext.PrimaryKey {
			primaryKeys = append(primaryKeys, prop.JSONName)
		}
	}

	var eventOneof *j5schema.OneofSchema
	for _, field := range entity.EventSchema.Properties {
		if field.JSONName != "event" {
			continue
		}
		oneofField, ok := field.Schema.(*j5schema.OneofField)
		if !ok {
			return nil, fmt.Errorf("event field is not oneof")
		}
		eventOneof = oneofField.Schema()
		break
	}

	if eventOneof == nil {
		return nil, fmt.Errorf("missing event oneof")
	}

	events := make([]*client_j5pb.StateEvent, 0, len(eventOneof.Properties))
	for _, prop := range eventOneof.Properties {
		objectField, ok := prop.Schema.(*j5schema.ObjectField)
		if !ok {
			return nil, fmt.Errorf("event property %q is not object", prop.JSONName)
		}
		desc := objectField.Schema().Description()

		events = append(events, &client_j5pb.StateEvent{
			Name:        prop.JSONName,
			FullName:    fmt.Sprintf("%s/%s.%s", entity.Package.Name, entity.Name, prop.JSONName),
			Description: desc,
		})
	}

	return &client_j5pb.StateEntity{
		Name:       entity.Name,
		FullName:   fmt.Sprintf("%s/%s", entity.Package.Name, entity.Name),
		SchemaName: entity.StateSchema.FullName(),
		PrimaryKey: primaryKeys,

		QueryService:    query,
		CommandServices: commands,

		Events: events,
	}, nil

}

type StateEvent struct {
	StateEntity *StateEntity // parent
	Name        string
	Schema      *j5schema.ObjectSchema
}

type schemaRef struct {
	schema    *schema_j5pb.RootSchema
	inPackage string
	name      string
}

// collectPackageRefs walks the entire API, returning all client schemas which are
// accessible via a method, event etc.
func collectPackageRefs(api *API) (map[string]*schemaRef, error) {
	// map[
	schemas := make(map[string]*schemaRef)

	var walkRefs func(j5schema.FieldSchema) error
	walkRefRoot := func(ref *j5schema.RefSchema) error {
		if ref.To == nil {
			// should be a reference to another API
			name := ref.Package.Name
			for _, pkg := range api.Packages {
				if pkg.Name == name {
					return fmt.Errorf("unlinked ref %q in linked package %q", name, ref.Schema)
				}
			}

			return nil
		}
		schema := ref.To
		_, ok := schemas[schema.FullName()]
		if ok {
			return nil
		}

		schemas[schema.FullName()] = &schemaRef{
			schema:    schema.ToJ5ClientRoot(),
			inPackage: schema.PackageName(),
			name:      schema.Name(),
		}
		switch st := schema.(type) {
		case *j5schema.ObjectSchema:
			for _, prop := range st.Properties {
				if err := walkRefs(prop.Schema); err != nil {
					return fmt.Errorf("walk %s: %w", st.FullName(), err)
				}
			}
		case *j5schema.OneofSchema:
			for _, prop := range st.Properties {
				if err := walkRefs(prop.Schema); err != nil {
					return fmt.Errorf("walk oneof: %w", err)
				}
			}
		case *j5schema.EnumSchema:
		// do nothing

		default:
			return fmt.Errorf("unsupported ref type %T", st)
		}
		return nil
	}
	walkRefs = func(schema j5schema.FieldSchema) error {

		switch st := schema.(type) {
		case *j5schema.ObjectField:
			if err := walkRefRoot(st.Ref); err != nil {
				return fmt.Errorf("walk object as field: %w", err)
			}

		case *j5schema.OneofField:
			if err := walkRefRoot(st.Ref); err != nil {
				return fmt.Errorf("walk oneof as field: %w", err)
			}

		case *j5schema.EnumField:
			if err := walkRefRoot(st.Ref); err != nil {
				return fmt.Errorf("walk enum as field: %w", err)
			}

		case *j5schema.ArrayField:
			if err := walkRefs(st.Schema); err != nil {
				return fmt.Errorf("walk array: %w", err)
			}

		case *j5schema.MapField:
			if err := walkRefs(st.Schema); err != nil {
				return fmt.Errorf("walk map: %w", err)
			}
		}

		return nil
	}

	walkRootObject := func(schema *j5schema.ObjectSchema) error {
		if schema == nil {
			return nil
		}
		for _, prop := range schema.Properties {
			if err := walkRefs(prop.Schema); err != nil {
				return fmt.Errorf("walk root object: %w", err)
			}
		}
		return nil
	}

	walkMethod := func(method *Method) error {
		if method.Request.Body != nil {
			if err := walkRootObject(method.Request.Body); err != nil {
				return fmt.Errorf("request schema %q: %w", method.Request.Body.FullName(), err)
			}
		}

		for _, prop := range method.Request.PathParameters {
			if err := walkRefs(prop.Schema); err != nil {
				return fmt.Errorf("path parameter %q: %w", prop.JSONName, err)
			}
		}

		for _, prop := range method.Request.QueryParameters {
			if err := walkRefs(prop.Schema); err != nil {
				return fmt.Errorf("path parameter %q: %w", prop.JSONName, err)
			}
		}

		if err := walkRootObject(method.ResponseBody); err != nil {
			return fmt.Errorf("response schema %q: %w", method.ResponseBody.FullName(), err)
		}

		return nil
	}

	for _, pkg := range api.Packages {
		for _, entity := range pkg.StateEntities {

			// add the entity fields directly so they don't get missed through
			// flattening or otherwise being unused.

			if err := walkRootObject(entity.KeysSchema); err != nil {
				return nil, fmt.Errorf("keys schema %q: %w", entity.KeysSchema.FullName(), err)
			}

			if err := walkRootObject(entity.StateSchema); err != nil {
				return nil, fmt.Errorf("state schema %q: %w", entity.StateSchema.FullName(), err)
			}

			if err := walkRootObject(entity.EventSchema); err != nil {
				return nil, fmt.Errorf("event schema %q: %w", entity.EventSchema.FullName(), err)
			}

			if entity.Query != nil {
				for _, method := range entity.Query.Methods {
					if err := walkMethod(method); err != nil {
						return nil, err
					}
				}
			}

			for _, commandService := range entity.Commands {
				for _, method := range commandService.Methods {
					if err := walkMethod(method); err != nil {
						return nil, err
					}
				}
			}
		}

		for _, service := range pkg.Services {
			for _, method := range service.Methods {
				if err := walkMethod(method); err != nil {
					return nil, err
				}
			}
		}
	}

	return schemas, nil
}
