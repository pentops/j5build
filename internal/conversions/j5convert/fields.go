package j5convert

import (
	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/id62"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func (ww *walkNode) buildField(schema *schema_j5pb.Field) *descriptorpb.FieldDescriptorProto {

	desc := &descriptorpb.FieldDescriptorProto{
		Options: &descriptorpb.FieldOptions{},
	}

	switch st := schema.Type.(type) {
	case *schema_j5pb.Field_Map:
		ww := ww.at("map")
		if st.Map.ItemSchema == nil {
			ww.errorf("missing map item schema")
			return nil
		}

		desc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()

		panic("Map not implemented")

	case *schema_j5pb.Field_Array:
		ww := ww.at("array")

		if st.Array.Items == nil {
			ww.errorf("missing array items")
			return nil
		}

		desc := ww.at("items").buildField(st.Array.Items)
		if desc == nil {
			return nil
		}

		desc.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Array{
				Array: st.Array.Ext,
			},
		})

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

		return desc

	case *schema_j5pb.Field_Object:
		ww := ww.at("object")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()

		switch where := st.Object.Schema.(type) {
		case *schema_j5pb.ObjectField_Ref:
			typeRef, err := ww.resolveType(where.Ref.Package, where.Ref.Schema)
			if err != nil {
				ww.at("ref").error(err)
				return nil
			}

			desc.TypeName = typeRef.protoTypeName()
			if typeRef.MessageRef == nil {
				ww.at("ref").errorf("type %s is not a message (for oneof)", *desc.TypeName)
				return nil
			}

			//msgRef = typeRef.MessageRef
		case *schema_j5pb.ObjectField_Object:
			// object is inline

			ww.at("object").doObject(where.Object)
			desc.TypeName = ptr(where.Object.Name)

		}

		if st.Object.Flatten {
			proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_Message{
					Message: &ext_j5pb.MessageFieldOptions{
						Flatten: true,
					},
				},
			})
		}
		if st.Object.Ext != nil {
			proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_Object{
					Object: st.Object.Ext,
				},
			})
		}

		if st.Object.Rules != nil {
			rules := &validate.FieldConstraints{}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
			ww.file.ensureImport(bufValidateImport)
			ww.errorf("TODO: object rules not implemented")
		}

		return desc

	case *schema_j5pb.Field_Oneof:
		ww := ww.at("oneof")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		//var msgRef *MessageRef

		switch where := st.Oneof.Schema.(type) {
		case *schema_j5pb.OneofField_Ref:
			typeRef, err := ww.resolveType(where.Ref.Package, where.Ref.Schema)
			if err != nil {
				ww.at("ref").error(err)
				return nil
			}
			desc.TypeName = typeRef.protoTypeName()
			if typeRef.MessageRef == nil {
				ww.errorf("type %s is not a message (for oneof)", *desc.TypeName)
			}
			//msgRef = typeRef.MessageRef

		case *schema_j5pb.OneofField_Oneof:
			// oneof is inline
			ww.errorf("TODO: inline oneof not implemented")
			return nil
		}

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Oneof{
				Oneof: st.Oneof.Ext,
			},
		})

		if st.Oneof.Rules != nil {
			rules := &validate.FieldConstraints{}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
			ww.file.ensureImport(bufValidateImport)
			ww.errorf("TODO: oneof rules not implemented")
			return nil
		}
		return desc

	case *schema_j5pb.Field_Enum:
		ww := ww.at("enum")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		var enumRef *EnumRef
		switch where := st.Enum.Schema.(type) {

		case *schema_j5pb.EnumField_Ref:
			typeRef, err := ww.resolveType(where.Ref.Package, where.Ref.Schema)
			if err != nil {
				ww.at("ref").error(err)
				return nil
			}
			desc.TypeName = typeRef.protoTypeName()
			if typeRef.EnumRef == nil {
				ww.errorf("type %s is not an enum", *desc.TypeName)
				return nil
			}
			enumRef = typeRef.EnumRef

		case *schema_j5pb.EnumField_Enum:
			// enum is inline
			ww.at("enum").doEnum(where.Enum)
			desc.TypeName = ptr(where.Enum.Name)
		}

		if st.Enum.Ext != nil {
			proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_Enum{
					Enum: st.Enum.Ext,
				},
			})
		}

		enumRules := &validate.EnumRules{
			DefinedOnly: ptr(true),
		}
		var err error

		if st.Enum.Rules != nil {
			enumRules.In, err = enumRef.mapValues(st.Enum.Rules.In)
			if err != nil {
				ww.error(err)
				return nil
			}
			enumRules.NotIn, err = enumRef.mapValues(st.Enum.Rules.NotIn)
			if err != nil {
				ww.error(err)
				return nil
			}
		}

		rules := &validate.FieldConstraints{
			Type: &validate.FieldConstraints_Enum{
				Enum: enumRules,
			},
		}
		proto.SetExtension(desc.Options, validate.E_Field, rules)
		return desc

	case *schema_j5pb.Field_Bool:
		//ww := ww.at("bool")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Bool{
				Bool: st.Bool.Ext,
			},
		})

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
		return desc

	case *schema_j5pb.Field_Bytes:
		//ww := ww.at("bytes")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Bytes{
				Bytes: st.Bytes.Ext,
			},
		})

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
		return desc

	case *schema_j5pb.Field_Date:
		ww := ww.at("date")
		ww.file.ensureImport(j5DateImport)
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = ptr(".j5.types.date.v1.Date")
		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Date{
				Date: st.Date.Ext,
			},
		})
		return desc

	case *schema_j5pb.Field_Decimal:
		ww := ww.at("decimal")
		ww.file.ensureImport(j5DecimalImport)
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = ptr(".j5.types.decimal.v1.Decimal")
		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Decimal{
				Decimal: st.Decimal.Ext,
			},
		})
		return desc

	case *schema_j5pb.Field_Float:
		ww := ww.at("float")
		if st.Float.Rules != nil {
			ww.errorf("TODO: float rules not implemented")
		}
		switch st.Float.Format {
		case schema_j5pb.FloatField_FORMAT_FLOAT32:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()

		case schema_j5pb.FloatField_FORMAT_FLOAT64:
			desc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
		default:
			ww.errorf("unknown float format %T", st.Float.Format)
			return nil
		}

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Float{
				Float: st.Float.Ext,
			},
		})
		return desc

	case *schema_j5pb.Field_Integer:
		ww := ww.at("integer")
		if st.Integer.Rules != nil {
			ww.errorf("TODO: integer rules not implemented")
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
			ww.errorf("unknown integer format %v", st.Integer.Format)
			return nil
		}

		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Integer{
				Integer: st.Integer.Ext,
			},
		})
		return desc

	case *schema_j5pb.Field_Key:
		ww := ww.at("key")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		ww.file.ensureImport(j5ExtImport)

		if st.Key.Ext != nil {
			if st.Key.Ext.PrimaryKey {
				proto.SetExtension(desc.Options, ext_j5pb.E_Key, &ext_j5pb.PSMKeyFieldOptions{
					PrimaryKey: true,
				})
			}
		}
		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Key{
				Key: st.Key.Ext,
			},
		})

		stringRules := &validate.StringRules{}

		if st.Key.Format != nil {
			switch st.Key.Format.Type.(type) {
			case *schema_j5pb.KeyFormat_Uuid:
				stringRules.WellKnown = &validate.StringRules_Uuid{
					Uuid: true,
				}

			case *schema_j5pb.KeyFormat_Id62:
				stringRules.Pattern = ptr(id62.PatternString)
			default:
				ww.errorf("unknown key format %T", st.Key.Format.Type)
				return nil
			}
			ww.file.ensureImport(bufValidateImport)
			proto.SetExtension(desc.Options, validate.E_Field, &validate.FieldConstraints{
				Type: &validate.FieldConstraints_String_{
					String_: stringRules,
				},
			})

		}
		return desc

	case *schema_j5pb.Field_String_:
		//ww := ww.at("string")
		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()

		if st.String_.Ext != nil {
			proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_String_{
					String_: st.String_.Ext,
				},
			})

		}
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
		return desc

	case *schema_j5pb.Field_Timestamp:
		ww := ww.at("timestamp")

		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = ptr(".google.protobuf.Timestamp")
		ww.file.ensureImport(pbTimestamp)
		proto.SetExtension(desc.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
			Type: &ext_j5pb.FieldOptions_Timestamp{
				Timestamp: st.Timestamp.Ext,
			},
		})

		if st.Timestamp.Rules != nil {
			rules := &validate.FieldConstraints{
				Type: &validate.FieldConstraints_Timestamp{
					Timestamp: &validate.TimestampRules{},
					// None Implemented.
				},
			}
			proto.SetExtension(desc.Options, validate.E_Field, rules)
		}

		return desc
	case *schema_j5pb.Field_Any:

		desc.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		desc.TypeName = ptr(".google.protobuf.Any")
		ww.file.ensureImport(pbAnyImport)
		/*
			proto.SetExtension(field.Options, ext_j5pb.E_Field, &ext_j5pb.FieldOptions{
				Type: &ext_j5pb.FieldOptions_Any{},
			})*/

		return desc
	default:
		ww.errorf("unknown schema type %T", schema.Type)
		return nil
	}

}
