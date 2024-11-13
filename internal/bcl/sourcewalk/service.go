package sourcewalk

import (
	"fmt"
	"path"
	"strconv"

	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type ServiceFileNode struct {
	services []*serviceBuilder
}

type ServiceNode struct {
	// Schema  *sourcedef_j5pb.Service
	Source  SourceNode
	Methods []*ServiceMethodNode

	Name string

	ServiceOptions *ext_j5pb.ServiceOptions
}

type ServiceMethodNode struct {
	Source       SourceNode
	InputType    string
	OutputType   string
	Schema       *sourcedef_j5pb.APIMethod
	ResolvedPath string
}

type ServiceFileVisitor interface {
	VisitObject(*ObjectNode) error
	VisitService(*ServiceNode) error
}

type ServiceFileCallbacks struct {
	Object  func(*ObjectNode) error
	Service func(*ServiceNode) error
}

func (mc ServiceFileCallbacks) VisitObject(on *ObjectNode) error {
	return mc.Object(on)
}

func (mc ServiceFileCallbacks) VisitService(sn *ServiceNode) error {
	return mc.Service(sn)
}

func (sf *ServiceFileNode) Accept(visitor ServiceFileVisitor) error {
	for idx, service := range sf.services {
		err := service.accept(visitor)
		var name string
		if service.schema != nil && service.schema.Name != nil {
			name = *service.schema.Name
		} else {
			name = strconv.Itoa(idx)
		}
		if err != nil {
			return fmt.Errorf("at service %s: %w", name, err)
		}
	}
	return nil
}

type serviceBuilder struct {
	schema *sourcedef_j5pb.Service
	source SourceNode
}

func newServiceRef(source SourceNode, schema *sourcedef_j5pb.Service) (*serviceBuilder, error) {
	return &serviceBuilder{
		source: source,
		schema: schema,
	}, nil
}

func (sn *serviceBuilder) accept(visitor ServiceFileVisitor) error {
	methods := make([]*ServiceMethodNode, 0, len(sn.schema.Methods))

	for idx, method := range sn.schema.Methods {
		source := sn.source.child("methods", strconv.Itoa(idx))

		request := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sRequest", method.Name),
			Properties: method.Request.Properties,
		}

		requestNode, err := newObjectSchemaNode(source.child("request"), nil, request)
		if err != nil {
			return fmt.Errorf("method %s request: %w", method.Name, err)
		}

		if err := visitor.VisitObject(requestNode); err != nil {
			return fmt.Errorf("method %s request: %w", method.Name, err)
		}

		var outputType string
		if method.Response == nil {
			outputType = "google.api.HttpBody"
		} else {
			response := &schema_j5pb.Object{
				Name:       fmt.Sprintf("%sResponse", method.Name),
				Properties: method.Response.Properties,
			}

			responseNode, err := newObjectSchemaNode(source.child("response"), nil, response)
			if err != nil {
				return fmt.Errorf("method %s response: %w", method.Name, err)
			}

			if err := visitor.VisitObject(responseNode); err != nil {
				return fmt.Errorf("method %s response: %w", method.Name, err)
			}

			outputType = response.Name
		}

		resolvedPath := method.HttpPath
		if sn.schema.BasePath != nil {
			resolvedPath = path.Join(*sn.schema.BasePath, resolvedPath)
		}

		methods = append(methods, &ServiceMethodNode{
			Source:       source.child("request"),
			InputType:    request.Name,
			OutputType:   outputType,
			Schema:       method,
			ResolvedPath: resolvedPath,
		})

	}
	if sn.schema.Name == nil {
		return fmt.Errorf("missing service name")
	}
	serviceNode := &ServiceNode{
		Source:         sn.source,
		Methods:        methods,
		Name:           *sn.schema.Name + "Service",
		ServiceOptions: sn.schema.Options,
	}

	return visitor.VisitService(serviceNode)
}
