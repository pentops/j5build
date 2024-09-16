package sourcewalk

import (
	"fmt"
	"strconv"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type SchemaVisitor interface {
	VisitObject(*ObjectNode) error
	VisitOneof(*OneofNode) error
	VisitEnum(*EnumNode) error
}

type SchemaCallbacks struct {
	Object func(*ObjectNode) error
	Oneof  func(*OneofNode) error
	Enum   func(*EnumNode) error
}

func (fc SchemaCallbacks) VisitObject(on *ObjectNode) error {
	return fc.Object(on)
}

func (fc SchemaCallbacks) VisitOneof(on *OneofNode) error {
	return fc.Oneof(on)
}

func (fc SchemaCallbacks) VisitEnum(en *EnumNode) error {
	return fc.Enum(en)
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
	Schema   *schema_j5pb.Enum
	Source   SourceNode
	NestPath []string
}

type objectLikeNode struct {
	properties []*propertyNode
	children   []*nestedNode // source should have an array of properties at the root.
	Source     SourceNode
	nestPath   []string
	name       string
}

func (on *objectLikeNode) HasNestedSchemas() bool {
	return len(on.children) > 0
}

type nestedNode struct {
	schema sourcedef_j5pb.IsNestedSchema_Type

	// should point to the item inside sourcedef.NestedSchema,
	// i.e. should already contain 'object', 'oneof' or 'enum' in the path.
	source   SourceNode
	nestPath []string
}

func mapNested(source SourceNode, nestName string, nestPath []string, nested []*sourcedef_j5pb.NestedSchema) []*nestedNode {
	out := make([]*nestedNode, 0, len(nested))
	for idx, n := range nested {
		out = append(out, &nestedNode{
			schema:   n.Type,
			source:   source.child(nestName, strconv.Itoa(idx)),
			nestPath: nestPath,
		})
	}
	return out
}

func (node *objectLikeNode) RangeNestedSchemas(visitor SchemaVisitor) error {
	return rangeNestedSchemas(node.children, visitor)
}

func rangeNestedSchemas(children []*nestedNode, visitor SchemaVisitor) error {

	for _, nested := range children {
		switch element := nested.schema.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			source := nested.source  // Points to the object root
			object := element.Object // Matches the source.

			objectNode := &ObjectNode{
				Schema: object.Def,
				objectLikeNode: objectLikeNode{
					Source:   source.child("def"),
					name:     object.Def.Name,
					nestPath: nested.nestPath,
					children: mapNested(
						source, "schemas",
						append(nested.nestPath, object.Def.Name),
						element.Object.Schemas,
					),
					properties: mapProperties(source.child("def"), "properties", object.Def.Properties),
				},
			}
			if err := visitor.VisitObject(objectNode); err != nil {
				return err
			}
		case *sourcedef_j5pb.NestedSchema_Oneof:
			source := nested.source // Points to the oneof root
			oneof := element.Oneof  // Matches the source.

			oneofNode := &OneofNode{
				Schema: oneof.Def,
				objectLikeNode: objectLikeNode{
					Source:   source.child("def"),
					name:     oneof.Def.Name,
					nestPath: nested.nestPath,
					children: mapNested(
						source, "schemas",
						append(nested.nestPath, oneof.Def.Name),
						oneof.Schemas,
					),
					properties: mapProperties(source.child("def"), "properties", oneof.Def.Properties),
				},
			}
			if err := visitor.VisitOneof(oneofNode); err != nil {
				return err
			}

		case *sourcedef_j5pb.NestedSchema_Enum:
			enum := element.Enum
			enumNode := &EnumNode{
				Schema:   enum,
				NestPath: nested.nestPath,
				Source:   nested.source,
			}
			if err := visitor.VisitEnum(enumNode); err != nil {
				return err
			}

		default:
			return walkerErrorf("unknown nexted schema type %T", element)
		}
	}
	return nil
}

func mapProperties(source SourceNode, sourcePath string, properties []*schema_j5pb.ObjectProperty, virtualPrepend ...*schema_j5pb.ObjectProperty) []*propertyNode {
	out := make([]*propertyNode, 0, len(properties))
	fieldNumber := int32(0)
	for _, prop := range virtualPrepend {
		fieldNumber++
		source := source.child(virtualPathNode, prop.Name)
		property := &propertyNode{
			schema: prop,
			source: source,
			number: fieldNumber,
		}
		out = append(out, property)
	}

	for idx, prop := range properties {
		fieldNumber++
		source := source.child(sourcePath, strconv.Itoa(idx))
		property := &propertyNode{
			schema: prop,
			source: source,
			number: fieldNumber,
		}

		out = append(out, property)
	}
	return out
}

func (on *objectLikeNode) RangeProperties(visitor PropertyVisitor) error {
	for _, prop := range on.properties {
		err := prop.accept(visitor)
		if err != nil {
			return fmt.Errorf("at property %s: %w", prop.schema.Name, err)
		}
	}
	return nil
}
