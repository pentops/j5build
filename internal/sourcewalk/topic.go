package sourcewalk

import (
	"fmt"
	"strconv"

	"github.com/iancoleman/strcase"
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

		methods := make([]*TopicMethodNode, 0)
		for idx, msg := range tt.Publish.Messages {
			source := source.child("messages", strconv.Itoa(idx))
			objSchema := &schema_j5pb.Object{
				Name:       fmt.Sprintf("%sMessage", msg.Name),
				Properties: msg.Fields,
			}

			if err := visitor.VisitObject(&ObjectNode{
				Schema: objSchema,
				objectLikeNode: objectLikeNode{
					Source:     source,
					properties: mapProperties(source, "fields", msg.Fields),
				},
			}); err != nil {
				return err
			}

			methods = append(methods, &TopicMethodNode{
				Source:  source,
				Name:    msg.Name,
				Request: objSchema.Name,
			})
		}

		return visitor.VisitTopic(&TopicNode{
			Source:  source,
			Methods: methods,
			Name:    fmt.Sprintf("%sTopic", strcase.ToCamel(tn.schema.Name)),
			ServiceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: ptr(strcase.ToSnake(tn.schema.Name)),
				Role: &messaging_j5pb.ServiceConfig_Publish_{
					Publish: &messaging_j5pb.ServiceConfig_Publish{},
				},
			},
		})

	case *sourcedef_j5pb.TopicType_Reqres:
		source := tn.source.child("type", "reqres")

		request := &schema_j5pb.Object{
			Name: fmt.Sprintf("%sRequestMessage", tn.schema.Name),
		}
		requestProperties := mapProperties(
			source.child("request"), "fields",
			request.Properties,
			&schema_j5pb.ObjectProperty{
				Name:   "request",
				Schema: schemaRefField("j5.messaging.v1", "RequestMetadata"),
			})

		err := visitor.VisitObject(&ObjectNode{
			Schema: request,
			objectLikeNode: objectLikeNode{
				Source:     source.child("request"),
				properties: requestProperties,
			},
		})

		if err != nil {
			return fmt.Errorf("request object: %w", err)
		}

		err = visitor.VisitTopic(&TopicNode{
			Source: source,
			Name:   fmt.Sprintf("%sRequestTopic", strcase.ToCamel(tn.schema.Name)),
			ServiceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: ptr(strcase.ToSnake(tn.schema.Name)),
				Role: &messaging_j5pb.ServiceConfig_Request_{
					Request: &messaging_j5pb.ServiceConfig_Request{},
				},
			},
			Methods: []*TopicMethodNode{{
				Source:  source,
				Name:    fmt.Sprintf("%sRequest", tn.schema.Name),
				Request: request.Name,
			}},
		})
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}

		reply := &schema_j5pb.Object{
			Name: fmt.Sprintf("%sReplyMessage", tn.schema.Name),
		}

		replyProperties := mapProperties(
			source.child("reply"), "fields",
			reply.Properties,
			&schema_j5pb.ObjectProperty{
				Name:   "request",
				Schema: schemaRefField("j5.messaging.v1", "RequestMetadata"),
			})

		err = visitor.VisitObject(&ObjectNode{
			Schema: reply,
			objectLikeNode: objectLikeNode{
				Source:     source.child("reply"),
				properties: replyProperties,
			},
		})
		if err != nil {
			return fmt.Errorf("requireply object: %w", err)
		}

		err = visitor.VisitTopic(&TopicNode{
			Source: source,
			Name:   fmt.Sprintf("%sReplyTopic", strcase.ToCamel(tn.schema.Name)),
			ServiceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: ptr(strcase.ToSnake(tn.schema.Name)),
				Role: &messaging_j5pb.ServiceConfig_Reply_{
					Reply: &messaging_j5pb.ServiceConfig_Reply{},
				},
			},
			Methods: []*TopicMethodNode{{
				Source:  source.child("reply"),
				Name:    fmt.Sprintf("%sReply", tn.schema.Name),
				Request: reply.Name,
			}},
		})
		if err != nil {
			return fmt.Errorf("reply: %w", err)
		}
		return nil

	case *sourcedef_j5pb.TopicType_Upsert_:

		source := tn.source.child("type", "upsert")

		upsert := tt.Upsert

		properties := mapProperties(
			source.child("message"), "fields",
			upsert.Message.Fields,
			&schema_j5pb.ObjectProperty{
				Name:   "upsert",
				Schema: schemaRefField("j5.messaging.v1", "UpsertMetadata"),
			})

		upsertSchema := &schema_j5pb.Object{
			Name: fmt.Sprintf("%sMessage", tn.schema.Name),
			//Properties: append([]*schema_j5pb.ObjectProperty{metadata}, upsert.Message.Fields...),
		}

		err := visitor.VisitObject(&ObjectNode{
			Schema: upsertSchema,
			objectLikeNode: objectLikeNode{
				Source:     source.child("message"),
				properties: properties,
			},
		})
		if err != nil {
			return fmt.Errorf("messagre: %w", err)
		}

		err = visitor.VisitTopic(&TopicNode{
			Source: source,
			Name:   fmt.Sprintf("%sTopic", strcase.ToCamel(tn.schema.Name)),
			ServiceConfig: &messaging_j5pb.ServiceConfig{
				TopicName: ptr(strcase.ToSnake(tn.schema.Name)),
				Role: &messaging_j5pb.ServiceConfig_Publish_{
					Publish: &messaging_j5pb.ServiceConfig_Publish{},
				},
			},
			Methods: []*TopicMethodNode{{
				Source:  source,
				Name:    fmt.Sprintf("%sUpsert", tn.schema.Name),
				Request: upsertSchema.Name,
			}},
		})
		if err != nil {
			return err
		}
		return nil

	default:
		return walkerErrorf("unknown topic type %T", tt)
	}
}
