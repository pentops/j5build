package sourcewalk

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

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

type EnumNode struct {
	Schema *schema_j5pb.Enum
	rootType
}

func newEnumNode(source SourceNode, parent parentNode, schema *schema_j5pb.Enum) (*EnumNode, error) {
	return &EnumNode{
		Schema:   schema,
		rootType: newRoot(source, parent, schema.Name),
	}, nil
}

type ObjectNode struct {
	Name        string
	Description string
	Entity      *schema_j5pb.EntityObject

	rootType
	propertySet
	nestedSet
}

func newVirtualObjectNode(
	source SourceNode,
	parent parentNode,
	name string,
	properties []*schema_j5pb.ObjectProperty,
	virtual ...*schema_j5pb.ObjectProperty,
) (*ObjectNode, error) {

	schema := &schema_j5pb.Object{
		Name: name,
	}
	root := newRoot(source, parent, schema.Name)
	return &ObjectNode{
		Name:        schema.Name,
		Description: schema.Description,
		Entity:      schema.Entity,
		rootType:    root,
		propertySet: propertySet{
			properties: mapProperties(source, []string{}, root, properties, virtual),
		},
	}, nil
}

func newObjectSchemaNode(source SourceNode, parent parentNode, schema *schema_j5pb.Object, virtual ...*schema_j5pb.ObjectProperty) (*ObjectNode, error) {
	root := newRoot(source, parent, schema.Name)
	return &ObjectNode{
		Name:        schema.Name,
		Description: schema.Description,
		Entity:      schema.Entity,
		rootType:    root,
		propertySet: propertySet{
			properties: mapProperties(source, []string{"properties"}, root, schema.Properties, virtual),
		},
	}, nil
}

func newObjectNode(source SourceNode, parent parentNode, wrapper *sourcedef_j5pb.Object) (*ObjectNode, error) {
	node, err := newObjectSchemaNode(source.child("def"), parent, wrapper.Def)
	if err != nil {
		return nil, err
	}

	node.nestedSet = mapNested(
		source,
		node,
		wrapper.Schemas,
	)

	return node, nil
}

type OneofNode struct {
	Schema *schema_j5pb.Oneof
	rootType
	propertySet
	nestedSet
}

func newOneofSchemaNode(source SourceNode, parent parentNode, schema *schema_j5pb.Oneof, virtual ...*schema_j5pb.ObjectProperty) (*OneofNode, error) {
	root := newRoot(source, parent, schema.Name)
	oneofNode := &OneofNode{
		Schema:   schema,
		rootType: root,
		propertySet: propertySet{
			properties: mapProperties(source, []string{"properties"}, root, schema.Properties, virtual),
		},
	}
	return oneofNode, nil
}

func newOneofNode(source SourceNode, parent parentNode, wrapper *sourcedef_j5pb.Oneof) (*OneofNode, error) {
	node, err := newOneofSchemaNode(source.child("def"), parent, wrapper.Def)
	if err != nil {
		return nil, err
	}

	node.nestedSet = mapNested(source, node, wrapper.Schemas)

	return node, nil
}

type parentNode interface {
	NestPath() []string
	NameInPackage() string
}

type rootType struct {
	Source   SourceNode
	name     string
	nestPath []string
}

func newRoot(source SourceNode, parent parentNode, name string) rootType {
	var nestPath []string
	if parent != nil {
		nestPath = parent.NestPath()
	}
	return rootType{
		Source:   source,
		name:     name,
		nestPath: nestPath,
	}
}

func (on rootType) NestPath() []string {
	if len(on.nestPath) == 0 {
		return []string{on.name}
	}
	return append(slices.Clone(on.nestPath), on.name)
}

func (on rootType) NameInPackage() string {
	if on.nestPath == nil {
		return on.name
	}
	return fmt.Sprintf("%s.%s", strings.Join(on.nestPath, "."), on.name)
}

type nestedNode struct {
	schema sourcedef_j5pb.IsNestedSchema_Type

	// should point to the item inside sourcedef.NestedSchema,
	// i.e. should already contain 'object', 'oneof' or 'enum' in the path.
	source SourceNode
	//nestPath []string
}

func mapNested(source SourceNode, parent parentNode, nested []*sourcedef_j5pb.NestedSchema) nestedSet {
	out := make([]*nestedNode, 0, len(nested))
	for idx, n := range nested {
		out = append(out, &nestedNode{
			schema: n.Type,
			source: source.child("schemas", strconv.Itoa(idx)),
		})
	}
	return nestedSet{
		children: out,
		parent:   parent,
	}
}

type nestedSet struct {
	children []*nestedNode // source should have an array of properties at the root.
	parent   parentNode
}

func (on *nestedSet) HasNestedSchemas() bool {
	return len(on.children) > 0
}

func (node *nestedSet) RangeNestedSchemas(visitor SchemaVisitor) error {

	for _, nested := range node.children {
		switch element := nested.schema.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			source := nested.source // Points to the object root

			objectNode, err := newObjectNode(source, node.parent, element.Object)
			if err != nil {
				return err
			}

			if err := visitor.VisitObject(objectNode); err != nil {
				return err
			}

		case *sourcedef_j5pb.NestedSchema_Oneof:
			source := nested.source // Points to the oneof root
			oneof := element.Oneof  // Matches the source.

			oneofNode, err := newOneofNode(source, node.parent, oneof)
			if err != nil {
				return err
			}

			if err := visitor.VisitOneof(oneofNode); err != nil {
				return err
			}

		case *sourcedef_j5pb.NestedSchema_Enum:
			enum := element.Enum
			enumNode, err := newEnumNode(nested.source, node.parent, enum)
			if err != nil {
				return err
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

func mapProperties(source SourceNode, sourcePath []string, parent parentNode, properties []*schema_j5pb.ObjectProperty, virtualPrepend []*schema_j5pb.ObjectProperty) []*propertyNode {
	out := make([]*propertyNode, 0, len(properties))
	fieldNumber := int32(0)
	for _, prop := range virtualPrepend {
		fieldNumber++
		source := source.child(virtualPathNode, prop.Name)
		property := &propertyNode{
			schema: prop,
			source: source,
			number: fieldNumber,
			parent: parent,
		}
		out = append(out, property)
	}

	if len(properties) == 0 {
		return out
	}

	propSource := source.child(sourcePath...)
	for idx, prop := range properties {
		fieldNumber++
		source := propSource.child(strconv.Itoa(idx))
		property := &propertyNode{
			schema: prop,
			source: source,
			number: fieldNumber,
			parent: parent,
		}

		out = append(out, property)
	}
	return out
}

type propertySet struct {
	properties []*propertyNode
}

func (on *propertySet) RangeProperties(visitor PropertyVisitor) error {
	for _, prop := range on.properties {
		err := prop.accept(visitor)
		if err != nil {
			return fmt.Errorf("at property %s: %w", prop.schema.Name, err)
		}
	}
	return nil
}
