package j5convert

import (
	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5build/internal/sourcewalk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type MessageBuilder struct {
	root       fileContext
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

func (msg *MessageBuilder) addMessage(message *MessageBuilder) {
	msg.commentSet.mergeAt([]int32{3, int32(len(msg.descriptor.NestedType))}, message.commentSet)
	msg.descriptor.NestedType = append(msg.descriptor.NestedType, message.descriptor)
}

func (msg *MessageBuilder) addEnum(enum *EnumBuilder) {
	msg.commentSet.mergeAt([]int32{4, int32(len(msg.descriptor.EnumType))}, enum.commentSet)
	msg.descriptor.EnumType = append(msg.descriptor.EnumType, enum.desc)
}

func convertObject(ww *walkContext, node *sourcewalk.ObjectNode) {

	message := blankMessage(ww.file, node.Name)

	if node.Entity != nil {
		ww.file.ensureImport(j5ExtImport)
		proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Psm, &ext_j5pb.PSMOptions{
			EntityName: node.Entity.Entity,
			EntityPart: node.Entity.Part.Enum(),
		})
	}

	objectType := &ext_j5pb.ObjectMessageOptions{}
	if node.AnyMember != nil {
		objectType.AnyMember = node.AnyMember
	}

	ww.file.ensureImport(j5ExtImport)
	ext := &ext_j5pb.MessageOptions{
		Type: &ext_j5pb.MessageOptions_Object{
			Object: objectType,
		},
	}
	proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Message, ext)

	message.comment([]int32{}, node.Description)

	err := node.RangeProperties(&sourcewalk.PropertyCallbacks{
		SchemaVisitor: walkerSchemaVisitor(ww.inMessage(message)),
		Property: func(node *sourcewalk.PropertyNode) error {

			propertyDesc, err := buildProperty(ww, node)
			if err != nil {
				ww.error(node.Source, err)
			}

			// Take the index (prior to append len == index), not the field number
			locPath := []int32{2, int32(len(message.descriptor.Field))}
			message.comment(locPath, node.Schema.Description)
			message.descriptor.Field = append(message.descriptor.Field, propertyDesc)
			return nil
		},
	})
	if err != nil {
		ww.error(node.Source, err)
	}

	if node.HasNestedSchemas() {
		subContext := ww.inMessage(message)
		if err := node.RangeNestedSchemas(walkerSchemaVisitor(subContext)); err != nil {
			ww.error(node.Source, err)
		}
	}

	ww.parentContext.addMessage(message)
}

func convertOneof(ww *walkContext, node *sourcewalk.OneofNode) {
	schema := node.Schema
	if schema.Name == "" {
		if ww.field == nil {
			ww.errorf(node.Source, "missing object name")
		}
		schema.Name = strcase.ToCamel(ww.field.name)
	}

	message := blankMessage(ww.file, schema.Name)
	message.descriptor.OneofDecl = []*descriptorpb.OneofDescriptorProto{{
		Name: ptr("type"),
	}}
	message.comment([]int32{}, schema.Description)

	oneofType := &ext_j5pb.OneofMessageOptions{}

	ww.file.ensureImport(j5ExtImport)
	ext := &ext_j5pb.MessageOptions{
		Type: &ext_j5pb.MessageOptions_Oneof{
			Oneof: oneofType,
		},
	}
	proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Message, ext)

	err := node.RangeProperties(&sourcewalk.PropertyCallbacks{
		SchemaVisitor: walkerSchemaVisitor(ww.inMessage(message)),
		Property: func(node *sourcewalk.PropertyNode) error {
			schema := node.Schema
			schema.ProtoField = []int32{node.Number}

			propertyDesc, err := buildProperty(ww, node)
			if err != nil {
				ww.error(node.Source, err)
				return nil
			}
			propertyDesc.OneofIndex = ptr(int32(0))

			// Take the index (prior to append len == index), not the field number
			locPath := []int32{2, int32(len(message.descriptor.Field))}
			message.comment(locPath, schema.Description)
			message.descriptor.Field = append(message.descriptor.Field, propertyDesc)
			return nil
		},
	})
	if err != nil {
		ww.error(node.Source, err)
	}

	if node.HasNestedSchemas() {
		subContext := ww.inMessage(message)
		if err := node.RangeNestedSchemas(walkerSchemaVisitor(subContext)); err != nil {
			ww.error(node.Source, err)
		}
	}

	ww.parentContext.addMessage(message)
}
