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

			objNode, err := newVirtualObjectNode(source, nil, fmt.Sprintf("%sMessage", msg.Name), msg.Fields)
			if err != nil {
				return err
			}

			if err := visitor.VisitObject(objNode); err != nil {
				return err
			}

			methods = append(methods, &TopicMethodNode{
				Source:  source,
				Name:    msg.Name,
				Request: objNode.NameInPackage(),
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

		requestNode, err := newVirtualObjectNode(source, nil, fmt.Sprintf("%sRequestMessage", tn.schema.Name), tt.Reqres.Request.Fields,
			&schema_j5pb.ObjectProperty{
				Name:   "request",
				Schema: schemaRefField("j5.messaging.v1", "RequestMetadata"),
			},
		)
		if err != nil {
			return err
		}
		err = visitor.VisitObject(requestNode)
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
				Request: requestNode.NameInPackage(),
			}},
		})
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}

		replyNode, err := newVirtualObjectNode(source, nil, fmt.Sprintf("%sReplyMessage", tn.schema.Name), tt.Reqres.Reply.Fields,
			&schema_j5pb.ObjectProperty{
				Name:   "request",
				Schema: schemaRefField("j5.messaging.v1", "RequestMetadata"),
			},
		)

		if err != nil {
			return err
		}
		err = visitor.VisitObject(replyNode)
		if err != nil {
			return fmt.Errorf("replyNode object: %w", err)
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
				Request: replyNode.NameInPackage(),
			}},
		})
		if err != nil {
			return fmt.Errorf("reply: %w", err)
		}
		return nil

	case *sourcedef_j5pb.TopicType_Upsert_:

		source := tn.source.child("type", "upsert")

		upsert := tt.Upsert

		messageNode, err := newVirtualObjectNode(source, nil, fmt.Sprintf("%sMessage", tn.schema.Name), upsert.Message.Fields,
			&schema_j5pb.ObjectProperty{
				Name:   "upsert",
				Schema: schemaRefField("j5.messaging.v1", "UpsertMetadata"),
			},
		)
		if err != nil {
			return err
		}

		err = visitor.VisitObject(messageNode)
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
				Name:    tn.schema.Name,
				Request: messageNode.NameInPackage(),
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
