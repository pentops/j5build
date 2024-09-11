package sourcewalk

import (
	"fmt"
	"strconv"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type ServiceFileNode struct {
	services []*serviceRef
}

type serviceRef struct {
	schema *sourcedef_j5pb.Service
	source SourceNode
}

type ServiceNode struct {
	Schema  *sourcedef_j5pb.Service
	Source  SourceNode
	Methods []*ServiceMethodNode
}

type ServiceMethodNode struct {
	Source     SourceNode
	InputType  string
	OutputType string
	Schema     *sourcedef_j5pb.Method
}

type ServiceFileVisitor interface {
	VisitObject(*ObjectNode)
	VisitService(*ServiceNode)
}

type ServiceFileCallbacks struct {
	Object  func(*ObjectNode)
	Service func(*ServiceNode)
}

func (mc ServiceFileCallbacks) VisitObject(on *ObjectNode) {
	mc.Object(on)
}

func (mc ServiceFileCallbacks) VisitService(sn *ServiceNode) {
	mc.Service(sn)
}

func (sf *ServiceFileNode) Accept(visitor ServiceFileVisitor) {
	for _, service := range sf.services {
		service.accept(visitor)
	}
}

func (sn *serviceRef) accept(visitor ServiceFileVisitor) {
	methods := make([]*ServiceMethodNode, 0, len(sn.schema.Methods))

	for idx, method := range sn.schema.Methods {
		source := sn.source.child("methods", strconv.Itoa(idx))

		request := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sRequest", method.Name),
			Properties: method.Request.Properties,
		}

		visitor.VisitObject(&ObjectNode{
			Schema: request,
			objectLikeNode: objectLikeNode{
				Source:     source.child("request"),
				properties: mapProperties(source.child("request", "properties"), method.Request.Properties),
			},
		})

		var outputType string
		if method.Response == nil {
			outputType = "google.api.HttpBody"
		} else {
			response := &schema_j5pb.Object{
				Name:       fmt.Sprintf("%sResponse", method.Name),
				Properties: method.Response.Properties,
			}
			visitor.VisitObject(&ObjectNode{
				Schema: response,
				objectLikeNode: objectLikeNode{
					Source:     source.child("response"),
					properties: mapProperties(source.child("response", "properties"), method.Response.Properties),
				},
			})

			outputType = response.Name
		}

		methods = append(methods, &ServiceMethodNode{
			Source:     source.child("request"),
			InputType:  request.Name,
			OutputType: outputType,
			Schema:     method,
		})

		visitor.VisitService(&ServiceNode{
			Schema:  sn.schema,
			Source:  sn.source,
			Methods: methods,
		})
	}
}
