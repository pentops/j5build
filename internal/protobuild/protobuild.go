package protobuild

import (
	"fmt"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func BuildFile(source *sourcedef_j5pb.SourceFile) (*descriptorpb.FileDescriptorProto, error) {
	fb := NewFileBuilder(source.Package, source.Path)

	for _, element := range source.Elements {
		if err := fb.AddRoot(element); err != nil {
			return nil, err
		}
	}

	return fb.File(), nil
}

type FileBuilder struct {
	Package string
	Name    string

	fdp *descriptorpb.FileDescriptorProto
}

func NewFileBuilder(pkg string, name string) *FileBuilder {
	return &FileBuilder{
		Package: pkg,
		Name:    name,

		fdp: &descriptorpb.FileDescriptorProto{
			Syntax:  ptr("proto3"),
			Package: ptr(pkg),
			Name:    ptr(name),
			Options: &descriptorpb.FileOptions{},
			SourceCodeInfo: &descriptorpb.SourceCodeInfo{
				Location: []*descriptorpb.SourceCodeInfo_Location{},
			},
		},
	}
}

func ptr[T any](v T) *T {
	return &v
}

func (fb *FileBuilder) ensureImport(importPath string) {
	for _, imp := range fb.fdp.Dependency {
		if imp == importPath {
			return
		}
	}
	fb.fdp.Dependency = append(fb.fdp.Dependency, importPath)
}

func (fb *FileBuilder) File() *descriptorpb.FileDescriptorProto {

	last := int32(1)
	for _, loc := range fb.fdp.SourceCodeInfo.Location {
		last += 2
		loc.Span = []int32{last, 1, 1}
	}

	return fb.fdp
}

func (fb *FileBuilder) AddRoot(schema *sourcedef_j5pb.RootElement) error {
	switch st := schema.Type.(type) {
	case *sourcedef_j5pb.RootElement_Object:
		if st.Object.Def == nil {
			return fmt.Errorf("missing object definition")
		}
		err := doMessage(fb, st.Object.Def)
		if err != nil {
			return errpos.AddContext(err, "object", st.Object.Def.Name)
		}
		return nil
	case *sourcedef_j5pb.RootElement_Enum:
		err := doEnum(fb, st.Enum)
		if err != nil {
			return errpos.AddContext(err, "enum")
		}
		return nil
	case *sourcedef_j5pb.RootElement_Oneof:
		if st.Oneof.Def == nil {
			return fmt.Errorf("missing oneof definition")
		}
		err := doOneof(fb, st.Oneof.Def)
		if err != nil {
			return errpos.AddContext(err, "oneof")
		}
		return nil
	case *sourcedef_j5pb.RootElement_Entity:
		err := fb.AddEntity(st.Entity)
		if err != nil {
			return errpos.AddContext(err, "entity")
		}
		return nil

	default:
		return fmt.Errorf("AddRoot: Unknown %T", schema.Type)
	}
}

func (fb *FileBuilder) AddEntity(entity *sourcedef_j5pb.Entity) error {
	if entity.Keys == nil {
		return fmt.Errorf("missing keys")
	}
	if entity.Data == nil {
		return fmt.Errorf("missing data")
	}
	if entity.Status == nil {
		return fmt.Errorf("missing status")
	}

	entity.Keys.Def.Description = entity.Description

	stateMsg := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "State"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_STATE,
		},
	}
	eventMsg := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Event"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_EVENT,
		},
	}

	if err := doMessage(fb, entity.Keys.Def); err != nil {
		return errpos.AddContext(err, "keys")
	}
	if err := doMessage(fb, stateMsg); err != nil {
		return errpos.AddContext(err, "state")
	}
	if err := doEnum(fb, entity.Status); err != nil {
		return errpos.AddContext(err, "status")
	}
	if err := doMessage(fb, entity.Data.Def); err != nil {
		return errpos.AddContext(err, "data")
	}
	if err := doMessage(fb, eventMsg); err != nil {
		return errpos.AddContext(err, "event")
	}

	return nil
}

func (fb *FileBuilder) addMessage(message *MessageBuilder) {
	idx := int32(len(fb.fdp.MessageType))
	path := []int32{4, idx}

	for _, comment := range message.commentSet {
		fb.fdp.SourceCodeInfo.Location = append(fb.fdp.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path:             append(path, comment.Path...),
			LeadingComments:  comment.LeadingComments,
			TrailingComments: comment.TrailingComments,
		})
	}

	fb.fdp.MessageType = append(fb.fdp.MessageType, message.descriptor)
}

func (fb *FileBuilder) addEnum(enum *EnumBuilder) {
	idx := int32(len(fb.fdp.EnumType))
	path := []int32{5, idx}

	for _, comment := range enum.commentSet {
		fb.fdp.SourceCodeInfo.Location = append(fb.fdp.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path:             append(path, comment.Path...),
			LeadingComments:  comment.LeadingComments,
			TrailingComments: comment.TrailingComments,
		})
	}

	fb.fdp.EnumType = append(fb.fdp.EnumType, enum.desc)
}

type SchemaCollection interface {
	AddSchema(schema *schema_j5pb.RootSchema) error
}

type parentFile interface {
	ensureImport(string)
	addMessage(*MessageBuilder)
	addEnum(*EnumBuilder)
}

type MessageBuilder struct {
	Parent     parentFile
	descriptor *descriptorpb.DescriptorProto
	commentSet
}

type commentSet []*descriptorpb.SourceCodeInfo_Location

func (cs *commentSet) comment(path []int32, description string) {
	*cs = append(*cs, sourceLoc(path, description))
}

func doMessage(parent parentFile, schema *schema_j5pb.Object) error {
	message := &MessageBuilder{
		Parent: parent,
		descriptor: &descriptorpb.DescriptorProto{
			Name:    ptr(schema.Name),
			Options: &descriptorpb.MessageOptions{},
		},
	}

	if schema.Entity != nil {
		parent.ensureImport(j5ExtImport)
		proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Psm, &ext_j5pb.PSMOptions{
			EntityName: schema.Entity.Entity,
		})

	}
	message.comment([]int32{}, schema.Description)

	for _, prop := range schema.Properties {
		//prop.ProtoField = []int32{int32(idx) + 1}
		if err := message.addProperty(prop); err != nil {
			return errpos.AddContext(err, prop.Name)
		}
	}

	parent.addMessage(message)

	return nil
}

type FieldBuilder struct {
	msg      *MessageBuilder
	desc     *descriptorpb.FieldDescriptorProto
	comments *descriptorpb.SourceCodeInfo
}

func (msg *MessageBuilder) addProperty(prop *schema_j5pb.ObjectProperty) error {
	if len(prop.ProtoField) == 0 {
		var props []*schema_j5pb.ObjectProperty
		switch st := prop.Schema.Type.(type) {
		case *schema_j5pb.Field_Object:
			props = st.Object.GetObject().Properties
		case *schema_j5pb.Field_Oneof:
			props = st.Oneof.GetOneof().Properties
		default:
			return fmt.Errorf("AddProperty: Invalid ObjectPRoperty.Schema.Type for 'unnumbered' field %T", prop.Schema.Type)
		}
		for _, p := range props {
			if err := msg.addProperty(p); err != nil {
				return err
			}
		}
		return nil
	}

	fb := &FieldBuilder{
		msg:      msg,
		comments: &descriptorpb.SourceCodeInfo{},
	}
	if prop.Schema == nil {
		fmt.Printf("Field: \n%s\n", prototext.Format(prop))

		return fmt.Errorf("missing schema/type")
	}
	err := fb.build(prop.Schema)
	if err != nil {

		return err
	}

	protoFieldName := strcase.ToSnake(prop.Name)
	fb.desc.Name = ptr(protoFieldName)

	// TODO: handle nested and flattened
	if len(prop.ProtoField) != 1 {
		return fmt.Errorf("unexpected number of proto fields %d", len(prop.ProtoField))
	}
	fb.desc.Number = ptr(prop.ProtoField[0])
	msg.comment([]int32{2, *fb.desc.Number}, prop.Description)
	msg.descriptor.Field = append(msg.descriptor.Field, fb.desc)

	return nil
}

func sourceLoc(path []int32, description string) *descriptorpb.SourceCodeInfo_Location {
	loc := &descriptorpb.SourceCodeInfo_Location{
		Path: path,
	}

	if description != "" {
		loc.LeadingComments = ptr(" " + description + "\n")
	}

	return loc
}

const (
	bufValidateImport = "buf/validate/validate.proto"
	j5ExtImport       = "j5/ext/v1/annotations.proto"
)

func (fb *FieldBuilder) build(schema *schema_j5pb.Field) error {

	field := &descriptorpb.FieldDescriptorProto{
		Options: &descriptorpb.FieldOptions{},
	}
	fb.desc = field

	switch st := schema.Type.(type) {
	case *schema_j5pb.Field_Any:
	case *schema_j5pb.Field_Array:

		if st.Array.Items == nil {
			fmt.Printf("Field: \n%s\n", prototext.Format(schema))

			return fmt.Errorf("missing schema/type")
		}
		err := fb.build(st.Array.Items)
		if err != nil {
			return errpos.AddContext(err, "array items")
		}
		fb.desc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		return nil

	case *schema_j5pb.Field_Boolean:
		field.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()

	case *schema_j5pb.Field_Bytes:
	case *schema_j5pb.Field_Date:
	case *schema_j5pb.Field_Decimal:
	case *schema_j5pb.Field_Enum:
		field.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		switch where := st.Enum.Schema.(type) {
		case *schema_j5pb.EnumField_Ref:
			if where.Ref.Package != "" {
				field.TypeName = ptr(fmt.Sprintf(".%s.%s", where.Ref.Package, where.Ref.Schema))
			} else {
				field.TypeName = ptr(where.Ref.Schema)
			}
		case *schema_j5pb.EnumField_Enum:
			// enum is inline
		}
	case *schema_j5pb.Field_Float:
	case *schema_j5pb.Field_Integer:
	case *schema_j5pb.Field_Key:
		field.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		if st.Key.Format != nil {
			switch st.Key.Format.Type.(type) {
			case *schema_j5pb.KeyFormat_Uuid:
				fb.msg.Parent.ensureImport(bufValidateImport)
				proto.SetExtension(field.Options, validate.E_Field, &validate.FieldConstraints{
					Type: &validate.FieldConstraints_String_{
						String_: &validate.StringRules{
							WellKnown: &validate.StringRules_Uuid{
								Uuid: true,
							},
						},
					},
				})

			default:
				return fmt.Errorf("unknown key format %T", st.Key.Format.Type)
			}

		}
		fb.msg.Parent.ensureImport(j5ExtImport)
		proto.SetExtension(field.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Key{
				Key: &ext_j5pb.KeyTypeFieldOptions{},
			},
		})

		if st.Key.Primary {
			proto.SetExtension(field.Options, ext_j5pb.E_Key, &ext_j5pb.KeyFieldOptions{
				PrimaryKey: true,
			})
		}

	case *schema_j5pb.Field_Map:
	case *schema_j5pb.Field_Object:
	case *schema_j5pb.Field_Oneof:
	case *schema_j5pb.Field_String_:
		field.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()

	case *schema_j5pb.Field_Timestamp:
	default:
		return fmt.Errorf("unknown schema type %T", schema.Type)
	}

	return nil

}

type EnumBuilder struct {
	desc *descriptorpb.EnumDescriptorProto
	commentSet
}

func doEnum(parent parentFile, schema *schema_j5pb.Enum) error {
	enum := &descriptorpb.EnumDescriptorProto{
		Name: ptr(schema.Name),
	}

	eb := &EnumBuilder{
		desc: enum,
	}

	eb.comment([]int32{}, schema.Description)

	for _, value := range schema.Options {
		enumValue := &descriptorpb.EnumValueDescriptorProto{
			Name:   ptr(fmt.Sprintf("%s%s", schema.Prefix, value.Name)),
			Number: ptr(value.Number),
		}
		enum.Value = append(enum.Value, enumValue)

		eb.comment([]int32{2, int32(value.Number)}, value.Description)

	}

	parent.addEnum(eb)
	return nil
}

func doOneof(parent parentFile, schema *schema_j5pb.Oneof) error {
	message := &MessageBuilder{
		Parent: parent,
		descriptor: &descriptorpb.DescriptorProto{
			Name:    ptr(schema.Name),
			Options: &descriptorpb.MessageOptions{},
		},
	}

	message.comment([]int32{}, schema.Description)

	for idx, prop := range schema.Properties {
		prop.ProtoField = []int32{int32(idx) + 1}
		if err := message.addProperty(prop); err != nil {
			return errpos.AddContext(err, prop.Name)
		}
	}

	parent.addMessage(message)

	return nil

}
