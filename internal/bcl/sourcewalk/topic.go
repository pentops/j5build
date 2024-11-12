package sourcewalk

import (
	"fmt"
	"strconv"

	"github.com/iancoleman/strcase"
	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type TopicFileNode struct {
	topics []*topicRef
}

type topicRef struct {
	schema *sourcedef_j5pb.Topic
	source SourceNode
}

type TopicNode struct {
	Name          string
	Source        SourceNode
	ServiceConfig *messaging_j5pb.ServiceConfig
	Methods       []*TopicMethodNode
}

type TopicMethodNode struct {
	Source  SourceNode
	Name    string
	Request string
}

type TopicFileVisitor interface {
	VisitObject(*ObjectNode) error
	VisitTopic(*TopicNode) error
}

type TopicFileCallbacks struct {
	Object func(*ObjectNode) error
	Topic  func(*TopicNode) error
}

func (tc TopicFileCallbacks) VisitObject(on *ObjectNode) error {
	return tc.Object(on)
}

func (tc TopicFileCallbacks) VisitTopic(tn *TopicNode) error {
	return tc.Topic(tn)
}

func (fn *TopicFileNode) Accept(visitor TopicFileVisitor) error {
	for _, topic := range fn.topics {
		if err := topic.accept(visitor); err != nil {
			return fmt.Errorf("at topic %s: %w", topic.schema.Name, err)
		}
	}
	return nil
}

func (tn *topicRef) accept(visitor TopicFileVisitor) error {
	switch tt := tn.schema.Type.Type.(type) {
	case *sourcedef_j5pb.TopicType_Publish_:
		source := tn.source.child("type", "publish")
		return acceptTopic(source, topicNode{
			name:    tn.schema.Name,
			methods: tt.Publish.Messages,
			serviceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: gl.Ptr(strcase.ToSnake(tn.schema.Name)),
				Role: &messaging_j5pb.ServiceConfig_Publish_{
					Publish: &messaging_j5pb.ServiceConfig_Publish{},
				},
			},
		}, visitor)

	case *sourcedef_j5pb.TopicType_Reqres:
		source := tn.source.child("type", "reqres")
		return acceptMultiReqResTopic(source, tn.schema.Name, tt.Reqres, visitor)

	case *sourcedef_j5pb.TopicType_Upsert_:
		source := tn.source.child("type", "upsert")
		name := tn.schema.Name
		if tt.Upsert.Message.Name == nil {
			tt.Upsert.Message.Name = &name
		}
		return acceptTopic(source, topicNode{
			name: name,
			methods: []*sourcedef_j5pb.TopicMethod{
				tt.Upsert.Message,
			},
			serviceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: gl.Ptr(strcase.ToSnake(name)),
				Role: &messaging_j5pb.ServiceConfig_Publish_{
					Publish: &messaging_j5pb.ServiceConfig_Publish{},
				},
			},
			prependFields: []*schema_j5pb.ObjectProperty{{
				Name:     "upsert",
				Schema:   schemaRefField("j5.messaging.v1", "UpsertMetadata"),
				Required: true,
			}},
		}, visitor)

	default:
		return walkerErrorf("unknown topic type %T", tt)
	}
}

type topicNode struct {
	name          string
	serviceConfig *messaging_j5pb.ServiceConfig
	methods       []*sourcedef_j5pb.TopicMethod
	prependFields []*schema_j5pb.ObjectProperty
}

func acceptTopic(source SourceNode, topic topicNode, visitor TopicFileVisitor) error {
	methods := make([]*TopicMethodNode, 0)
	for idx, method := range topic.methods {
		source := source.child("messages", strconv.Itoa(idx))

		var methodName string
		if method.Name != nil {
			methodName = *method.Name
		} else if len(topic.methods) == 1 {
			name := topic.name
			methodName = name
		} else {
			return walkerErrorf("method name is required (multiple methods specified)")
		}
		messageNode, err := newVirtualObjectNode(source, nil, fmt.Sprintf("%sMessage", methodName), method.Fields, topic.prependFields...)
		if err != nil {
			return err
		}

		if err := visitor.VisitObject(messageNode); err != nil {
			return err
		}

		methods = append(methods, &TopicMethodNode{
			Source:  source,
			Name:    methodName,
			Request: messageNode.NameInPackage(),
		})
	}

	return visitor.VisitTopic(&TopicNode{
		Source:        source,
		Methods:       methods,
		Name:          fmt.Sprintf("%sTopic", strcase.ToCamel(topic.name)),
		ServiceConfig: topic.serviceConfig,
	})
}

func acceptMultiReqResTopic(source SourceNode, name string, topic *sourcedef_j5pb.TopicType_ReqRes, visitor TopicFileVisitor) error {
	if err := acceptTopic(source, topicNode{
		name:    fmt.Sprintf("%sRequest", name),
		methods: topic.Request,
		serviceConfig: &messaging_j5pb.ServiceConfig{
			TopicName: gl.Ptr(strcase.ToSnake(name)),
			Role: &messaging_j5pb.ServiceConfig_Request_{
				Request: &messaging_j5pb.ServiceConfig_Request{},
			},
		},
		prependFields: []*schema_j5pb.ObjectProperty{{
			Name:     "request",
			Schema:   schemaRefField("j5.messaging.v1", "RequestMetadata"),
			Required: true,
		}},
	}, visitor); err != nil {
		return fmt.Errorf("req: %w", err)
	}

	if err := acceptTopic(source, topicNode{
		name:    fmt.Sprintf("%sReply", name),
		methods: topic.Reply,
		serviceConfig: &messaging_j5pb.ServiceConfig{
			TopicName: gl.Ptr(strcase.ToSnake(name)),
			Role: &messaging_j5pb.ServiceConfig_Reply_{
				Reply: &messaging_j5pb.ServiceConfig_Reply{},
			},
		},
		prependFields: []*schema_j5pb.ObjectProperty{{
			Name:     "request",
			Schema:   schemaRefField("j5.messaging.v1", "RequestMetadata"),
			Required: true,
		}},
	}, visitor); err != nil {
		return fmt.Errorf("req: %w", err)
	}

	return nil
}
