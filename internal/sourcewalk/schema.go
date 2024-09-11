package sourcewalk

import (
	"fmt"
	"strconv"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type SchemaVisitor interface {
	VisitObject(*ObjectNode)
	VisitOneof(*OneofNode)
	VisitEnum(*EnumNode)
}

type SchemaCallbacks struct {
	Object func(*ObjectNode)
	Oneof  func(*OneofNode)
	Enum   func(*EnumNode)
}

func (fc SchemaCallbacks) VisitObject(on *ObjectNode) {
	fc.Object(on)
}

func (fc SchemaCallbacks) VisitOneof(on *OneofNode) {
	fc.Oneof(on)
}

func (fc SchemaCallbacks) VisitEnum(en *EnumNode) {
	fc.Enum(en)
}

type ObjectNode struct {
	Schema *schema_j5pb.Object
	objectLikeNode
}

type OneofNode struct {
	Schema *schema_j5pb.Oneof
	objectLikeNode
}

type EnumNode struct {
	Schema *schema_j5pb.Enum
	Source SourceNode
}

type objectLikeNode struct {
	properties []*propertyNode
	children   []*nestedNode
	Source     SourceNode
}

func (on *objectLikeNode) HasNestedSchemas() bool {
	return len(on.children) > 0
}

type nestedNode struct {
	schema *sourcedef_j5pb.NestedSchema
	source SourceNode
}

func mapNested(source SourceNode, nested []*sourcedef_j5pb.NestedSchema) []*nestedNode {
	out := make([]*nestedNode, 0, len(nested))
	for idx, n := range nested {
		out = append(out, &nestedNode{
			schema: n,
			source: source.child(strconv.Itoa(idx)),
		})
	}
	return out
}

func (node *objectLikeNode) RangeNestedSchemas(visitor SchemaVisitor) {
	for idx, nested := range node.children {
		switch element := nested.schema.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			source := nested.source.child("object")
			object := element.Object.Def
			objectNode := &ObjectNode{
				Schema: object,
				objectLikeNode: objectLikeNode{
					Source:     source.child("def"),
					children:   mapNested(source.child("schemas"), element.Object.Schemas),
					properties: mapProperties(source.child("def", "properties"), element.Object.Def.Properties),
				},
			}
			visitor.VisitObject(objectNode)
		case *sourcedef_j5pb.NestedSchema_Oneof:
			source := nested.source.child("oneof")
			oneof := element.Oneof.Def
			oneofNode := &OneofNode{
				Schema: oneof,
				objectLikeNode: objectLikeNode{
					Source:     source.child("def"),
					children:   mapNested(source.child("schemas"), element.Oneof.Schemas),
					properties: mapProperties(source.child("def", "properties"), element.Oneof.Def.Properties),
				},
			}
			visitor.VisitOneof(oneofNode)

		case *sourcedef_j5pb.NestedSchema_Enum:
			enum := element.Enum
			enumNode := &EnumNode{
				Schema: enum,
				Source: node.Source.child("elements", strconv.Itoa(idx), "enum"),
			}
			visitor.VisitEnum(enumNode)

		default:
			panic(fmt.Sprintf("unhandled nested schema type %T", element))
		}
	}
}

func mapProperties(source SourceNode, properties []*schema_j5pb.ObjectProperty, virtualPrepend ...*schema_j5pb.ObjectProperty) []*propertyNode {
	out := make([]*propertyNode, 0, len(properties))
	fieldNumber := int32(0)
	for _, prop := range virtualPrepend {
		fieldNumber++
		source := source.maybeChild("_virtual", prop.Name)
		property := &propertyNode{
			schema: proto.Clone(prop).(*schema_j5pb.ObjectProperty),
			source: source,
			number: fieldNumber,
		}
		out = append(out, property)
	}

	for idx, prop := range properties {
		fieldNumber++
		source := source.child(strconv.Itoa(idx))
		property := &propertyNode{
			schema: proto.Clone(prop).(*schema_j5pb.ObjectProperty),
			source: source,
			number: fieldNumber,
		}

		out = append(out, property)
	}
	return out
}

func (on *objectLikeNode) RangeProperties(visitor PropertyVisitor) {
	for _, prop := range on.properties {
		prop.accept(visitor)
	}
}

type propertyNode struct {
	schema *schema_j5pb.ObjectProperty
	source SourceNode
	number int32
}

func (pn *propertyNode) accept(visitor PropertyVisitor) {

	propNode := &PropertyNode{
		Schema: pn.schema,
		Source: pn.source,
		Number: pn.number,
	}

	source := pn.source.child("schema")

	switch pt := pn.schema.Schema.Type.(type) {
	case *schema_j5pb.Field_Array:
		items := pt.Array.Items
		replaceNestedSchemas(source.child("array"), items, visitor)
		propNode.Field = FieldNode{
			Source: source.child("array"),
			Schema: pt,
		}

	case *schema_j5pb.Field_Map:
		values := pt.Map.ItemSchema
		replaceNestedSchemas(source.child("map"), values, visitor)
		propNode.Field = FieldNode{
			Source: source.child("map"),
			Schema: pt,
		}

	case *schema_j5pb.Field_Object:
		replaceNestedObject(source.child("object"), pt.Object, visitor)
		propNode.Field = FieldNode{
			Source: source.child("object"),
			Schema: pt,
		}

	case *schema_j5pb.Field_Oneof:
		replaceNestedOneof(source.child("oneof"), pt.Oneof, visitor)
		propNode.Field = FieldNode{
			Source: source.child("oneof"),
			Schema: pt,
		}

	case *schema_j5pb.Field_Enum:
		replaceNestedEnum(source.child("enum"), pt.Enum, visitor)
		propNode.Field = FieldNode{
			Source: source.child("enum"),
			Schema: pt,
		}

	default:
		tn := pn.schema.Schema.ProtoReflect()
		var name string
		tn.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			name = string(fd.Name())
			return false
		})

		propNode.Field = FieldNode{
			Source: source.child(name),
			Schema: pt,
		}
	}
	visitor.VisitProperty(propNode)
}

func replaceNestedSchemas(source SourceNode, schema *schema_j5pb.Field, visitor SchemaVisitor) {
	switch pt := schema.Type.(type) {
	case *schema_j5pb.Field_Object:
		replaceNestedObject(source.child("object"), pt.Object, visitor)

	case *schema_j5pb.Field_Oneof:
		replaceNestedOneof(source.child("oneof"), pt.Oneof, visitor)

	case *schema_j5pb.Field_Enum:
		replaceNestedEnum(source.child("enum"), pt.Enum, visitor)
	}
}

func replaceNestedObject(source SourceNode, field *schema_j5pb.ObjectField, visitor SchemaVisitor) {
	schema := field.GetObject()
	if schema != nil {
		visitor.VisitObject(&ObjectNode{
			Schema: schema,
			objectLikeNode: objectLikeNode{
				Source:     source.child("object"),
				properties: mapProperties(source.child("object", "properties"), schema.Properties),
			},
		})
		field.Schema = &schema_j5pb.ObjectField_Ref{
			Ref: &schema_j5pb.Ref{
				Schema: schema.Name,
			},
		}
	}
}

func replaceNestedOneof(source SourceNode, field *schema_j5pb.OneofField, visitor SchemaVisitor) {
	schema := field.GetOneof()
	if schema != nil {
		visitor.VisitOneof(&OneofNode{
			Schema: schema,
			objectLikeNode: objectLikeNode{
				Source:     source.child("oneof"),
				properties: mapProperties(source.child("oneof", "properties"), schema.Properties),
			},
		})
		field.Schema = &schema_j5pb.OneofField_Ref{
			Ref: &schema_j5pb.Ref{
				Schema: schema.Name,
			},
		}
	}
}

func replaceNestedEnum(source SourceNode, field *schema_j5pb.EnumField, visitor SchemaVisitor) {

	schema := field.GetEnum()
	if schema != nil {
		visitor.VisitEnum(&EnumNode{
			Schema: schema,
			Source: source.child("enum"),
		})
		field.Schema = &schema_j5pb.EnumField_Ref{
			Ref: &schema_j5pb.Ref{
				Schema: schema.Name,
			},
		}
	}

}
