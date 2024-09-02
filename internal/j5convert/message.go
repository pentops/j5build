package j5convert

import (
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MessageBuilder struct {
	root       rootContext
	isOneof    bool
	descriptor *descriptorpb.DescriptorProto
	commentSet
}

func blankMessage(root rootContext, name string) *MessageBuilder {
	message := &MessageBuilder{
		root: root,
		descriptor: &descriptorpb.DescriptorProto{
			Name:    ptr(name),
			Options: &descriptorpb.MessageOptions{},
		},
	}

	return message
}

func blankOneof(root rootContext, name string) *MessageBuilder {
	message := blankMessage(root, name)
	message.isOneof = true
	return message
}

func (msg *MessageBuilder) setDescription(str string) {
	msg.comment([]int32{}, str)
}

func (msg *MessageBuilder) entityType(name string, part schema_j5pb.EntityPart) {
	msg.root.ensureImport(j5ExtImport)
	proto.SetExtension(msg.descriptor.Options, ext_j5pb.E_Psm, &ext_j5pb.PSMOptions{
		EntityName: name,
	})
}

func (msg *MessageBuilder) addMessage(message *MessageBuilder) {
	msg.descriptor.NestedType = append(msg.descriptor.NestedType, message.descriptor)
}

func (msg *MessageBuilder) addEnum(enum *EnumBuilder) {
	msg.descriptor.EnumType = append(msg.descriptor.EnumType, enum.desc)
}

func (msg *MessageBuilder) schemaRefField() *schema_j5pb.Field {
	return schemaRefField("", *msg.descriptor.Name)
}

func schemaRefField(pkg, desc string) *schema_j5pb.Field {
	return &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Ref{
					Ref: &schema_j5pb.Ref{
						Package: pkg,
						Schema:  desc,
					},
				},
			},
		},
	}
}
