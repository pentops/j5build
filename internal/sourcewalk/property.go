package sourcewalk

import (
	"fmt"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PropertyVisitor interface {
	SchemaVisitor
	VisitProperty(*PropertyNode) error
}

type PropertyCallbacks struct {
	SchemaVisitor
	Property func(*PropertyNode) error
}

func (pc PropertyCallbacks) VisitProperty(pn *PropertyNode) error {
	return pc.Property(pn)
}

type PropertyNode struct {
	Schema *schema_j5pb.ObjectProperty
	Source SourceNode
	Number int32
	Field  FieldNode
}

type FieldNode struct {
	Source SourceNode
	Schema schema_j5pb.IsField_Type

	// for Object, Oneof or Enum, inline schemas are converted to refs, and refs
	// are passed through.
	// Check Items.Ref for arrays and maps!
	Ref *RefNode

	// for Array or Map, the FieldNode for the item / map value type
	Items *FieldNode
}

type RefNode struct {
	*schema_j5pb.Ref
	Source SourceNode
	Inline bool // Ref was converted from inline schema

	InlineObject *ObjectNode
	InlineOneof  *OneofNode
	InlineEnum   *EnumNode
}

type propertyNode struct {
	schema *schema_j5pb.ObjectProperty
	source SourceNode
	number int32
}

func (pn *propertyNode) accept(visitor PropertyVisitor) error {

	propNode := &PropertyNode{
		Schema: pn.schema,
		Source: pn.source,
		Number: pn.number,
	}

	source := pn.source.child("schema")

	fieldNode, err := buildFieldNode(source, pn.schema.Schema, visitor)
	if err != nil {
		return err
	}

	propNode.Field = *fieldNode

	return visitor.VisitProperty(propNode)

}

func buildFieldNode(source SourceNode, pn *schema_j5pb.Field, visitor PropertyVisitor) (*FieldNode, error) {
	if pn == nil || pn.Type == nil {
		return nil, fmt.Errorf("field type is nil")
	}
	switch pt := pn.Type.(type) {
	case *schema_j5pb.Field_Array:
		items := pt.Array.Items
		itemsNode, err := buildFieldNode(source.child("array", "items"), items, visitor)
		if err != nil {
			return nil, err
		}

		return &FieldNode{
			Source: source.child("array"),
			Schema: pt,
			Items:  itemsNode,
		}, nil

	case *schema_j5pb.Field_Map:
		values := pt.Map.ItemSchema
		itemsNode, err := buildFieldNode(source.child("map", "itemSchema"), values, visitor)
		if err != nil {
			return nil, err
		}

		return &FieldNode{
			Source: source.child("map"),
			Schema: pt,
			Items:  itemsNode,
		}, nil

	case *schema_j5pb.Field_Object:
		ref, err := replaceNestedObject(source.child("object"), pt.Object, visitor)
		if err != nil {
			return nil, err
		}
		return &FieldNode{
			Source: source.child("object"),
			Schema: pt,
			Ref:    ref,
		}, nil

	case *schema_j5pb.Field_Oneof:
		ref, err := replaceNestedOneof(source.child("oneof"), pt.Oneof, visitor)
		if err != nil {
			return nil, err
		}
		return &FieldNode{
			Source: source.child("oneof"),
			Schema: pt,
			Ref:    ref,
		}, nil

	case *schema_j5pb.Field_Enum:
		ref, err := replaceNestedEnum(source.child("enum"), pt.Enum, visitor)
		if err != nil {
			return nil, err
		}
		return &FieldNode{
			Source: source.child("enum"),
			Schema: pt,
			Ref:    ref,
		}, nil

	default:
		tn := pn.ProtoReflect()
		var name string
		tn.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			name = string(fd.Name())
			return false
		})

		return &FieldNode{
			Source: source.child(name),
			Schema: pt,
		}, nil
	}

}

func replaceNestedObject(source SourceNode, field *schema_j5pb.ObjectField, visitor SchemaVisitor) (*RefNode, error) {
	switch st := field.Schema.(type) {
	case *schema_j5pb.ObjectField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil

	case *schema_j5pb.ObjectField_Object:
		objectNode := &ObjectNode{
			Schema: st.Object,
			objectLikeNode: objectLikeNode{
				name:       st.Object.Name,
				Source:     source.child("object"),
				properties: mapProperties(source.child("object"), "properties", st.Object.Properties),
			},
		}
		if err := visitor.VisitObject(objectNode); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: st.Object.Name,
			},
			Source:       source.child("object"),
			Inline:       true,
			InlineObject: objectNode,
		}, nil

	default:
		return nil, fmt.Errorf("unhandled object schema type %T", st)
	}
}

func replaceNestedOneof(source SourceNode, field *schema_j5pb.OneofField, visitor SchemaVisitor) (*RefNode, error) {
	switch st := field.Schema.(type) {
	case *schema_j5pb.OneofField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil
	case *schema_j5pb.OneofField_Oneof:
		oneofNode := &OneofNode{
			Schema: st.Oneof,
			objectLikeNode: objectLikeNode{
				name:       st.Oneof.Name,
				Source:     source.child("oneof"),
				properties: mapProperties(source.child("oneof"), "properties", st.Oneof.Properties),
			},
		}
		if err := visitor.VisitOneof(oneofNode); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: st.Oneof.Name,
			},
			Source:      source.child("oneof"),
			Inline:      true,
			InlineOneof: oneofNode,
		}, nil
	default:
		return nil, fmt.Errorf("unhandled oneof schema type %T", st)
	}
}

func replaceNestedEnum(source SourceNode, field *schema_j5pb.EnumField, visitor SchemaVisitor) (*RefNode, error) {
	switch st := field.Schema.(type) {
	case *schema_j5pb.EnumField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil
	case *schema_j5pb.EnumField_Enum:
		enumNode := &EnumNode{
			Schema: st.Enum,
			Source: source.child("enum"),
		}
		if err := visitor.VisitEnum(enumNode); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: st.Enum.Name,
			},
			Source:     source.child("enum"),
			Inline:     true,
			InlineEnum: enumNode,
		}, nil

	default:
		return nil, fmt.Errorf("unhandled enum schema type %T", st)
	}

}
