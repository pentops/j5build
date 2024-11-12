package j5convert

import (
	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/bcl/sourcewalk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func convertFile(ww *walkContext, src *sourcedef_j5pb.SourceFile) error {
	file := sourcewalk.NewRoot(src)
	return file.RangeRootElements(sourcewalk.FileCallbacks{
		SchemaCallbacks: sourcewalk.SchemaCallbacks{
			Object: func(on *sourcewalk.ObjectNode) error {
				convertObject(ww, on)
				return nil
			},
			Oneof: func(on *sourcewalk.OneofNode) error {
				convertOneof(ww, on)
				return nil
			},
			Enum: func(en *sourcewalk.EnumNode) error {
				convertEnum(ww, en)
				return nil
			},
		},
		TopicFile: func(tn *sourcewalk.TopicFileNode) error {
			subWalk := ww.subPackageFile("topic")
			return convertTopic(subWalk, tn)
		},
		ServiceFile: func(sn *sourcewalk.ServiceFileNode) error {
			subWalk := ww.subPackageFile("service")
			return convertService(subWalk, sn)
		},
	})
}

func walkerSchemaVisitor(ww *walkContext) sourcewalk.SchemaVisitor {
	return &sourcewalk.SchemaCallbacks{
		Object: func(on *sourcewalk.ObjectNode) error {
			convertObject(ww, on)
			return nil
		},
		Oneof: func(on *sourcewalk.OneofNode) error {
			convertOneof(ww, on)
			return nil
		},
		Enum: func(en *sourcewalk.EnumNode) error {
			convertEnum(ww, en)
			return nil
		},
	}
}
func convertTopic(ww *walkContext, tn *sourcewalk.TopicFileNode) error {
	return tn.Accept(sourcewalk.TopicFileCallbacks{
		Topic: func(tn *sourcewalk.TopicNode) error {
			convertTopicNode(ww, tn)
			return nil
		},
		Object: func(on *sourcewalk.ObjectNode) error {
			convertObject(ww, on)
			return nil
		},
	})
}

func convertTopicNode(ww *walkContext, tn *sourcewalk.TopicNode) {
	desc := &descriptorpb.ServiceDescriptorProto{
		Name:    gl.Ptr(tn.Name),
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(desc.Options, messaging_j5pb.E_Service, tn.ServiceConfig)

	for _, method := range tn.Methods {
		rpcDesc := &descriptorpb.MethodDescriptorProto{
			Name:       gl.Ptr(method.Name),
			OutputType: gl.Ptr(googleProtoEmptyType),
			InputType:  gl.Ptr(method.Request),
		}
		desc.Method = append(desc.Method, rpcDesc)
	}

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: desc,
	})
}
