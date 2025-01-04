package j5convert

import (
	"errors"
	"fmt"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/iancoleman/strcase"
	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/list/v1/list_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/id62"
	"github.com/pentops/j5build/internal/bcl/sourcewalk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func buildProperty(ww *conversionVisitor, node *sourcewalk.PropertyNode) (*descriptorpb.FieldDescriptorProto, error) {

	if node.Schema.Schema == nil {
		return nil, fmt.Errorf("missing schema")
	}

	desc, err := buildField(ww, node.Field)
	if err != nil {
		return nil, err
	}

	required := node.Schema.Required
	if ext := proto.GetExtension(desc.Options, ext_j5pb.E_Key).(*ext_j5pb.PSMKeyFieldOptions); ext != nil {
		if ext.PrimaryKey {
			// even if not explicitly set, a primary key is required, we don't support partial primary keys.
			required = true
		}
	}

	if required {
		ext := proto.GetExtension(desc.Options, validate.E_Field).(*validate.FieldConstraints)
		if ext == nil {
			ext = &validate.FieldConstraints{}
		}
		ww.file.ensureImport(bufValidateImport)
		ext.Required = gl.Ptr(true)
		proto.SetExtension(desc.Options, validate.E_Field, ext)
		ww.file.ensureImport(j5ExtImport)
	}

	if node.Schema.ExplicitlyOptional {
		if required {
			return nil, fmt.Errorf("cannot be both required and optional")
		}
		desc.Proto3Optional = gl.Ptr(true)
	}

	protoFieldName := strcase.ToSnake(node.Schema.Name)
	desc.Name = gl.Ptr(protoFieldName)
	desc.JsonName = gl.Ptr(node.Schema.Name)
	desc.Number = gl.Ptr(node.Number)
	return desc, nil
}

func buildField(ww *conversionVisitor, node sourcewalk.FieldNode) (*descriptorpb.FieldDescriptorProto, error) {

	desc := &descriptorpb.FieldDescriptorProto{
		Options: &descriptorpb.FieldOptions{},
	}

	switch st := node.Schema.(type) {
	case *schema_j5pb.Field_Map:
		if st.Map.ItemSchema == nil {
			return nil, errors.New("missing map item schema")
		}

		itemDesc, err := buildField(ww, *node.Items)
		if err != nil {
			return nil, err
		}

		keyDesc := &descriptorpb.FieldDescriptorProto{}

		desc := &descriptorpb.FieldDescriptorProto{}

		desc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()

		_ = keyDesc
		_ = itemDesc
		panic("Map not implemented")

	case *schema_j5pb.Field_Array:

		if st.Array.Items == nil {
			return nil, errors.New("missing array items")
		}

		desc, err := buildField(ww, *node.Items)
		if err != nil {
			return nil, err
		}

		desc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

		if st.Array.Ext != nil {
			proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_Array{
					Array: &ext_j5pb.ArrayField{
						SingleForm: st.Array.Ext.SingleForm,
					},
				},
			})
		}

		ww.setJ5Ext(node.Source, desc.Options, "array", st.Array.Ext)

		if st.Array.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_Repeated{
					Repeated: &validate.RepeatedRules{
						MinItems: st.Array.Rules.MinItems,
						MaxItems: st.Array.Rules.MaxItems,
						Unique:   st.Array.Rules.UniqueItems,
					},
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
			ww.file.ensureImport(bufValidateImport)
		}

		return desc, nil

	case *schema_j5pb.Field_Object:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()

		typeRef, err := ww.resolveType(node.Ref) //.Package, ref.Schema)
		if err != nil {
			return nil, err
		}

		desc.TypeName = typeRef.protoTypeName()
		if typeRef.MessageRef == nil {
			return nil, fmt.Errorf("type %s is not a message (for object)", *desc.TypeName)
		}

		ext := ww.setJ5Ext(node.Source, desc.Options, "object", st.Object.Ext)

		if st.Object.Flatten {
			ext.Type.(*ext_j5pb.FieldOptions_Object).Object.Flatten = true
		}

		if st.Object.Rules != nil {
			rules := &validate.FieldConstraints{}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
			ww.file.ensureImport(bufValidateImport)
		}

		return desc, nil

	case *schema_j5pb.Field_Oneof:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()

		typeRef, err := ww.resolveType(node.Ref)
		if err != nil {
			return nil, err
		}
		desc.TypeName = typeRef.protoTypeName()
		if typeRef.MessageRef == nil {
			return nil, fmt.Errorf("type %s is not a message (for oneof)", *desc.TypeName)
		}

		ww.setJ5Ext(node.Source, desc.Options, "oneof", st.Oneof.Ext)

		if st.Oneof.Rules != nil {
			rules := &validate.FieldConstraints{}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
			ww.file.ensureImport(bufValidateImport)
		}
		return desc, nil

	case *schema_j5pb.Field_Enum:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		var enumRef *EnumRef

		typeRef, err := ww.resolveType(node.Ref)
		if err != nil {
			return nil, err
		}
		desc.TypeName = typeRef.protoTypeName()
		if typeRef.EnumRef == nil {
			return nil, fmt.Errorf("type %s is not an enum", *desc.TypeName)
		}
		enumRef = typeRef.EnumRef

		ww.setJ5Ext(node.Source, desc.Options, "enum", st.Enum.Ext)

		enumRules := &validate.EnumRules{
			DefinedOnly: gl.Ptr(true),
		}

		if st.Enum.Rules != nil {
			enumRules.In, err = enumRef.mapValues(st.Enum.Rules.In)
			if err != nil {
				return nil, err
			}
			enumRules.NotIn, err = enumRef.mapValues(st.Enum.Rules.NotIn)
			if err != nil {
				return nil, err
			}
		}

		rules := &validate.FieldConstraints{
			Type: &validate.FieldConstraints_Enum{
				Enum: enumRules,
			},
		}
		ww.file.ensureImport(bufValidateImport)
		proto.SetExtension(desc.Options, validate.E_Field, rules)
		return desc, nil

	case *schema_j5pb.Field_Bool:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()

		ww.setJ5Ext(node.Source, desc.Options, "bool", st.Bool.Ext)

		if st.Bool.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_Bool{
					Bool: &validate.BoolRules{
						Const: st.Bool.Rules.Const,
					},
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
		}
		return desc, nil

	case *schema_j5pb.Field_Bytes:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()

		ww.setJ5Ext(node.Source, desc.Options, "bytes", st.Bytes.Ext)

		if st.Bytes.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_Bytes{
					Bytes: &validate.BytesRules{
						MinLen: st.Bytes.Rules.MinLength,
						MaxLen: st.Bytes.Rules.MaxLength,
					},
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
		}
		return desc, nil

	case *schema_j5pb.Field_Date:
		ww.file.ensureImport(j5DateImport)
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = gl.Ptr(".j5.types.date.v1.Date")

		ww.setJ5Ext(node.Source, desc.Options, "date", st.Date.Ext)

		return desc, nil

	case *schema_j5pb.Field_Decimal:
		ww.file.ensureImport(j5DecimalImport)
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = gl.Ptr(".j5.types.decimal.v1.Decimal")

		ww.setJ5Ext(node.Source, desc.Options, "decimal", st.Decimal.Ext)

		return desc, nil

	case *schema_j5pb.Field_Float:
		if st.Float.Rules != nil {
			return nil, fmt.Errorf("TODO: float rules not implemented")
		}
		switch st.Float.Format {
		case schema_j5pb.FloatField_FORMAT_FLOAT32:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()

		case schema_j5pb.FloatField_FORMAT_FLOAT64:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
		default:
			return nil, fmt.Errorf("unknown float format %T", st.Float.Format)
		}

		ww.setJ5Ext(node.Source, desc.Options, "float", st.Float.Ext)

		return desc, nil

	case *schema_j5pb.Field_Integer:
		if st.Integer.Rules != nil {
			return nil, fmt.Errorf("TODO: integer rules not implemented")
		}
		switch st.Integer.Format {
		case schema_j5pb.IntegerField_FORMAT_INT32:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
		case schema_j5pb.IntegerField_FORMAT_INT64:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		case schema_j5pb.IntegerField_FORMAT_UINT32:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum()
		case schema_j5pb.IntegerField_FORMAT_UINT64:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()
		default:
			return nil, fmt.Errorf("unknown integer format %v", st.Integer.Format)
		}

		ww.setJ5Ext(node.Source, desc.Options, "integer", st.Integer.Ext)

		return desc, nil

	case *schema_j5pb.Field_Key:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		ww.file.ensureImport(j5ExtImport)

		if st.Key.Entity != nil {
			entityExt := &ext_j5pb.PSMKeyFieldOptions{}
			switch et := st.Key.Entity.Type.(type) {
			case *schema_j5pb.EntityKey_PrimaryKey:
				if et.PrimaryKey { // May be explicitly false to self-document
					entityExt.PrimaryKey = true
				}
			case *schema_j5pb.EntityKey_ForeignKey:
				entityExt.ForeignKey = et.ForeignKey
			}
			proto.SetExtension(desc.Options, ext_j5pb.E_Key, entityExt)
		}

		ww.setJ5Ext(node.Source, desc.Options, "key", st.Key.Ext)

		if st.Key.ListRules != nil {
			var fkt list_j5pb.IsForeignKeyRules_Type

			if st.Key.Format == nil {
				fkt = &list_j5pb.ForeignKeyRules_UniqueString{
					UniqueString: st.Key.ListRules,
				}
			} else {
				switch st.Key.Format.Type.(type) {
				case *schema_j5pb.KeyFormat_Id62:
					fkt = &list_j5pb.ForeignKeyRules_Id62{
						Id62: st.Key.ListRules,
					}
				case *schema_j5pb.KeyFormat_Uuid:
					fkt = &list_j5pb.ForeignKeyRules_Uuid{
						Uuid: st.Key.ListRules,
					}
				case *schema_j5pb.KeyFormat_Custom_:
					fkt = &list_j5pb.ForeignKeyRules_UniqueString{
						UniqueString: st.Key.ListRules,
					}
				default:
					return nil, fmt.Errorf("unknown key format %T", st.Key.Format.Type)
				}
			}

			ww.file.ensureImport(j5ListAnnotationsImport)
			proto.SetExtension(desc.Options, list_j5pb.E_Field, &list_j5pb.FieldConstraint{
				Type: &list_j5pb.FieldConstraint_String_{
					String_: &list_j5pb.StringRules{
						WellKnown: &list_j5pb.StringRules_ForeignKey{
							ForeignKey: &list_j5pb.ForeignKeyRules{
								Type: fkt,
							},
						},
					},
				},
			})
		}

		stringRules := &validate.StringRules{}

		if st.Key.Format != nil {
			switch ff := st.Key.Format.Type.(type) {
			case *schema_j5pb.KeyFormat_Uuid:
				stringRules.WellKnown = &validate.StringRules_Uuid{
					Uuid: true,
				}

			case *schema_j5pb.KeyFormat_Id62:
				stringRules.Pattern = gl.Ptr(id62.PatternString)

			case *schema_j5pb.KeyFormat_Custom_:
				stringRules.Pattern = &ff.Custom.Pattern

			case *schema_j5pb.KeyFormat_Informal_:

			default:
				return nil, fmt.Errorf("unknown key format %T", st.Key.Format.Type)
			}
			ww.file.ensureImport(bufValidateImport)
			proto.SetExtension(desc.Options, validate.E_Field, &validate.FieldConstraints{
				Type: &validate.FieldConstraints_String_{
					String_: stringRules,
				},
			})

		}
		return desc, nil

	case *schema_j5pb.Field_String_:
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()

		ww.setJ5Ext(node.Source, desc.Options, "string", st.String_.Ext)

		if st.String_.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_String_{
					String_: &validate.StringRules{
						MinLen:  st.String_.Rules.MinLength,
						MaxLen:  st.String_.Rules.MaxLength,
						Pattern: st.String_.Rules.Pattern,
					},
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
		}
		return desc, nil

	case *schema_j5pb.Field_Timestamp:

		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = gl.Ptr(".google.protobuf.Timestamp")
		ww.file.ensureImport(pbTimestamp)

		ww.setJ5Ext(node.Source, desc.Options, "timestamp", st.Timestamp.Ext)

		if st.Timestamp.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_Timestamp{
					Timestamp: &validate.TimestampRules{},
					// None Implemented.
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
		}

		return desc, nil
	case *schema_j5pb.Field_Any:

		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = gl.Ptr(".j5.types.any.v1.Any")
		ww.file.ensureImport(j5AnyImport)

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Any{
				Any: &ext_j5pb.AnyField{
					OnlyDefined: st.Any.OnlyDefined,
					Types:       st.Any.Types,
				},
			},
		})

		return desc, nil
	default:
		return nil, fmt.Errorf("unknown schema type %T", st)
	}

}

// Copies the J5 extension object to the equivalent protoreflect extension type
// by field names.
func (ww *conversionVisitor) setJ5Ext(node sourcewalk.SourceNode, dest *descriptorpb.FieldOptions, fieldType protoreflect.Name, j5Ext proto.Message) *ext_j5pb.FieldOptions {

	// Options in the *proto* representation.
	extOptions := &ext_j5pb.FieldOptions{}
	extOptionsRefl := extOptions.ProtoReflect()

	// The proto extension is a oneof to each field type, which should match the
	// specified type.

	typeField := extOptionsRefl.Descriptor().Fields().ByName(fieldType)
	if typeField == nil {
		ww.addErrorf(node, "Field %s does not have a type field", fieldType)
		return nil
	}

	extTypedRefl := extOptionsRefl.Mutable(typeField).Message()
	if extTypedRefl == nil {
		ww.addErrorf(node, "Field %s type field is not a message", fieldType)
		return nil
	}

	// The J5 extension should already be typed. It should have the same fields
	// as the Proto extension.
	j5ExtRefl := j5Ext.ProtoReflect()
	if j5ExtRefl.IsValid() {
		j5ExtFields := j5ExtRefl.Descriptor().Fields()

		// Copy each field from the J5 extension to the Proto extension.
		err := RangeField(j5ExtRefl, func(fd protoreflect.FieldDescriptor, v protoreflect.Value) error {
			destField := j5ExtFields.ByName(fd.Name())
			if destField == nil {
				return fmt.Errorf("No equivalent for %s in %s", fd.FullName(), j5ExtRefl.Descriptor().FullName())
			}

			if destField.Kind() != fd.Kind() {
				return fmt.Errorf("Field %s has different kind in %s", fd.FullName(), j5ExtRefl.Descriptor().FullName())
			}

			extTypedRefl.Set(fd, j5ExtRefl.Get(destField))
			return nil
		})
		if err != nil {
			ww.addErrorf(node, "Error copying J5 extension to Proto extension: %v", err)
			return nil
		}
	}

	ww.file.ensureImport(j5ExtImport)
	// Set the extension, even if no fields were set, as this indicates the J5
	// type.
	proto.SetExtension(dest, ext_j5pb.E_Field, extOptions)

	return extOptions
}

func RangeField(pt protoreflect.Message, f func(protoreflect.FieldDescriptor, protoreflect.Value) error) error {
	var err error
	pt.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		err = f(fd, v)
		return err == nil
	})
	return err
}
