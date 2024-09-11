package j5convert

import (
	"fmt"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func (ww *walkNode) doTopic(schema *sourcedef_j5pb.Topic) {
	subWalk := ww.subPackageFile("topic")

	switch tt := schema.Type.Type.(type) {
	case *sourcedef_j5pb.TopicType_Publish_:
		subWalk.at("type", "publish").doPublishTopic(schema.Name, tt.Publish)
	case *sourcedef_j5pb.TopicType_Reqres:
		subWalk.at("type", "reqres").doReqResTopic(schema.Name, tt.Reqres)
	case *sourcedef_j5pb.TopicType_Upsert_:
		subWalk.at("type", "upsert").doUpsertTopic(schema.Name, tt.Upsert)
	default:
		ww.errorf("unknown topic type %T", tt)
	}
}

func (ww *walkNode) doPublishTopic(name string, schema *sourcedef_j5pb.TopicType_Publish) {
	name = strcase.ToCamel(name)
	desc := &descriptorpb.ServiceDescriptorProto{
		Name:    ptr(name + "Topic"),
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(desc.Options, messaging_j5pb.E_Service, &messaging_j5pb.ServiceConfig{
		TopicName: ptr(strcase.ToSnake(name)),
		Role: &messaging_j5pb.ServiceConfig_Publish_{
			Publish: &messaging_j5pb.ServiceConfig_Publish{},
		},
	})

	for idx, msg := range schema.Messages {
		objSchema := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sMessage", msg.Name),
			Properties: msg.Fields,
		}
		ww.at("message", fmt.Sprint(idx)).doObject(objSchema)
		rpcDesc := &descriptorpb.MethodDescriptorProto{
			Name:       ptr(msg.Name),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(objSchema.Name),
		}
		desc.Method = append(desc.Method, rpcDesc)
	}

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: desc,
	})
}

func (ww *walkNode) doReqResTopic(name string, schema *sourcedef_j5pb.TopicType_ReqRes) {
	addRequest := func(ww *walkNode, msg *MessageBuilder) {
		msg.descriptor.Field = append(msg.descriptor.Field, &descriptorpb.FieldDescriptorProto{
			Name:     ptr("request"),
			JsonName: ptr("request"),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: ptr(".j5.messaging.v1.RequestMetadata"),
			Number:   ptr(int32(len(msg.descriptor.Field) + 1)),
		})
	}

	name = strcase.ToCamel(name)
	reqObj := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sRequestMessage", name),
		Properties: schema.Request.Fields,
	}
	ww.at("request").doObject(reqObj, addRequest)

	resObj := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sReplyMessage", name),
		Properties: schema.Reply.Fields,
	}
	ww.at("reply").doObject(resObj, addRequest)

	reqDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "RequestTopic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name + "Request"),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(reqObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(reqDesc.Options, messaging_j5pb.E_Service, &messaging_j5pb.ServiceConfig{
		TopicName: ptr(strcase.ToSnake(name) + "_request"),
		Role: &messaging_j5pb.ServiceConfig_Request_{
			Request: &messaging_j5pb.ServiceConfig_Request{},
		},
	})

	resDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "ReplyTopic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name + "Reply"),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(resObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(resDesc.Options, messaging_j5pb.E_Service, &messaging_j5pb.ServiceConfig{
		TopicName: ptr(strcase.ToSnake(name) + "_reply"),
		Role: &messaging_j5pb.ServiceConfig_Reply_{
			Reply: &messaging_j5pb.ServiceConfig_Reply{},
		},
	})

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(messagingReqResImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: reqDesc,
	})
	ww.file.addService(&ServiceBuilder{
		desc: resDesc,
	})

}
func (ww *walkNode) doUpsertTopic(name string, schema *sourcedef_j5pb.TopicType_Upsert) {

	name = strcase.ToCamel(name)
	reqObj := &schema_j5pb.Object{
		Name: fmt.Sprintf("%sMessage", name),
		Properties: append([]*schema_j5pb.ObjectProperty{{
			Name:   "upsert",
			Schema: schemaRefField("j5.messaging.v1", "UpsertMetadata"),
		}}, schema.Message.Fields...),
	}
	ww.at("message").doObject(reqObj)

	reqDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "Topic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(reqObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(reqDesc.Options, messaging_j5pb.E_Service, &messaging_j5pb.ServiceConfig{
		TopicName: ptr(strcase.ToSnake(name)),
		Role: &messaging_j5pb.ServiceConfig_Publish_{
			Publish: &messaging_j5pb.ServiceConfig_Publish{},
		},
	})

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(messagingUpsertImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: reqDesc,
	})

}
