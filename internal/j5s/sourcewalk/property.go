package sourcewalk

import (
	"fmt"

	"github.com/iancoleman/strcase"
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
	parent parentNode
	Field  FieldNode
}

func (pn *PropertyNode) NameInPackage() string {
	return fmt.Sprintf("%s.%s", pn.parent.NameInPackage(), pn.Schema.Name)
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
	parent parentNode
	number int32
}

func (pn *propertyNode) accept(visitor PropertyVisitor) error {

	propNode := &PropertyNode{
		Schema: pn.schema,
		Source: pn.source,
		Number: pn.number,
		parent: pn.parent,
	}

	source := pn.source.child("schema")

	defaultNestingName := strcase.ToCamel(pn.schema.Name)
	fieldNode, err := buildFieldNode(source, pn.parent, defaultNestingName, pn.schema.Schema, visitor)
	if err != nil {
		return err
	}

	propNode.Field = *fieldNode

	return visitor.VisitProperty(propNode)

}

func buildFieldNode(source SourceNode, parent parentNode, defaultNestingName string, pn *schema_j5pb.Field, visitor PropertyVisitor) (*FieldNode, error) {
	if pn == nil || pn.Type == nil {
		return nil, fmt.Errorf("field type is nil")
	}
	switch pt := pn.Type.(type) {
	case *schema_j5pb.Field_Array:
		items := pt.Array.Items
		itemsNode, err := buildFieldNode(source.child("array", "items"), parent, defaultNestingName, items, visitor)
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
		itemsNode, err := buildFieldNode(source.child("map", "itemSchema"), parent, defaultNestingName, values, visitor)
		if err != nil {
			return nil, err
		}

		return &FieldNode{
			Source: source.child("map"),
			Schema: pt,
			Items:  itemsNode,
		}, nil

	case *schema_j5pb.Field_Object:
		ref, err := replaceNestedObject(source.child("object"), parent, defaultNestingName, pt.Object, visitor)
		if err != nil {
			return nil, err
		}
		return &FieldNode{
			Source: source.child("object"),
			Schema: pt,
			Ref:    ref,
		}, nil

	case *schema_j5pb.Field_Oneof:
		ref, err := replaceNestedOneof(source.child("oneof"), parent, defaultNestingName, pt.Oneof, visitor)
		if err != nil {
			return nil, err
		}
		return &FieldNode{
			Source: source.child("oneof"),
			Schema: pt,
			Ref:    ref,
		}, nil

	case *schema_j5pb.Field_Enum:
		ref, err := replaceNestedEnum(source.child("enum"), parent, defaultNestingName, pt.Enum, visitor)
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

func replaceNestedObject(source SourceNode, parent parentNode, defaultName string, field *schema_j5pb.ObjectField, visitor SchemaVisitor) (*RefNode, error) {
	if field.Schema == nil {
		field.Schema = &schema_j5pb.ObjectField_Object{
			Object: &schema_j5pb.Object{},
		}
	}
	switch st := field.Schema.(type) {
	case *schema_j5pb.ObjectField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil

	case *schema_j5pb.ObjectField_Object:
		if st.Object.Name == "" {
			st.Object.Name = defaultName
		}
		node, err := newObjectSchemaNode(source.child("object"), parent, st.Object)
		if err != nil {
			return nil, err
		}
		if err := visitor.VisitObject(node); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: node.NameInPackage(),
			},
			Source:       source.child("object"),
			Inline:       true,
			InlineObject: node,
		}, nil

	default:
		return nil, fmt.Errorf("unhandled object schema type %T", st)
	}
}

func replaceNestedOneof(source SourceNode, parent parentNode, defaultName string, field *schema_j5pb.OneofField, visitor SchemaVisitor) (*RefNode, error) {
	if field.Schema == nil {
		field.Schema = &schema_j5pb.OneofField_Oneof{
			Oneof: &schema_j5pb.Oneof{},
		}
	}
	switch st := field.Schema.(type) {
	case *schema_j5pb.OneofField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil
	case *schema_j5pb.OneofField_Oneof:
		if st.Oneof.Name == "" {
			st.Oneof.Name = defaultName
		}
		node, err := newOneofSchemaNode(source.child("oneof"), parent, st.Oneof)
		if err != nil {
			return nil, err
		}
		if err := visitor.VisitOneof(node); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: node.NameInPackage(),
			},
			Source:      source.child("oneof"),
			Inline:      true,
			InlineOneof: node,
		}, nil
	default:
		return nil, fmt.Errorf("unhandled oneof schema type %T", st)
	}
}

func replaceNestedEnum(source SourceNode, parent parentNode, defaultName string, field *schema_j5pb.EnumField, visitor SchemaVisitor) (*RefNode, error) {
	switch st := field.Schema.(type) {
	case *schema_j5pb.EnumField_Ref:
		return &RefNode{
			Ref:    st.Ref,
			Source: source.child("ref"),
		}, nil
	case *schema_j5pb.EnumField_Enum:
		if st.Enum.Name == "" {
			st.Enum.Name = defaultName
		}
		node, err := newEnumNode(source.child("enum"), parent, st.Enum)
		if err != nil {
			return nil, err
		}
		if err := visitor.VisitEnum(node); err != nil {
			return nil, err
		}
		return &RefNode{
			Ref: &schema_j5pb.Ref{
				Schema: st.Enum.Name,
			},
			Source:     source.child("enum"),
			Inline:     true,
			InlineEnum: node,
		}, nil

	default:
		return nil, fmt.Errorf("unhandled enum schema type %T", st)
	}

}
