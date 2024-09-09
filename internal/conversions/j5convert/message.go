package j5convert

import (
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MessageBuilder struct {
	root       fileContext
	isOneof    bool
	descriptor *descriptorpb.DescriptorProto
	commentSet
}

func blankMessage(root fileContext, name string) *MessageBuilder {
	message := &MessageBuilder{
		root: root,
		descriptor: &descriptorpb.DescriptorProto{
			Name:    ptr(name),
			Options: &descriptorpb.MessageOptions{},
		},
	}

	return message
}

func blankOneof(root fileContext, name string) *MessageBuilder {
	message := blankMessage(root, name)
	message.isOneof = true
	proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Message, &ext_j5pb.MessageOptions{
		IsOneofWrapper: true,
	})
	return message
}

func (msg *MessageBuilder) addMessage(message *MessageBuilder) {
	msg.commentSet.mergeAt([]int32{3, int32(len(msg.descriptor.NestedType))}, message.commentSet)
	msg.descriptor.NestedType = append(msg.descriptor.NestedType, message.descriptor)
}

func (msg *MessageBuilder) addEnum(enum *EnumBuilder) {
	msg.commentSet.mergeAt([]int32{4, int32(len(msg.descriptor.EnumType))}, enum.commentSet)
	msg.descriptor.EnumType = append(msg.descriptor.EnumType, enum.desc)
}
