package schema

import (
	"fmt"
	"sort"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"golang.org/x/exp/maps"
)

type containerSet []containerField

type Container interface {
	Path() []string
	Spec() BlockSpec
	Name() string
}

func (bs containerSet) schemaNames() []string {
	names := make([]string, 0, len(bs))
	for _, block := range bs {
		names = append(names, block.schemaName)
	}
	return names
}

func (bs containerSet) allChildFields() map[string]*schema_j5pb.Field {
	children := map[string]*schema_j5pb.Field{}
	for _, blockSchema := range bs {
		_ = blockSchema.container.RangePropertySchemas(func(name string, required bool, schema *schema_j5pb.Field) error {
			if _, ok := children[name]; !ok {
				children[name] = schema
			}
			return nil
		})
		for name, path := range blockSchema.spec.Aliases {
			schema, err := blockSchema.container.ContainerSchema().WalkToProperty(path...)
			if err != nil {
				continue
			}
			if _, ok := children[name]; !ok {
				children[name] = schema.ToJ5Field()
			}
		}
	}
	return children
}

func (bs containerSet) listChildren() []string {
	fields := bs.allChildFields()
	fieldNames := maps.Keys(fields)
	sort.Strings(fieldNames)
	return fieldNames
}

func (bs containerSet) listAttributes() []string {
	fields := bs.allChildFields()
	fieldNames := []string{}

	for name, field := range fields {
		if schemaCan(field.GetType()).canAttribute {
			fieldNames = append(fieldNames, name)
		}
	}

	sort.Strings(fieldNames)
	return fieldNames
}

func (bs containerSet) listBlocks() []string {
	fields := bs.allChildFields()
	fieldNames := []string{}

	for name, field := range fields {
		if schemaCan(field.GetType()).canBlock {
			fieldNames = append(fieldNames, name)
		}
	}
	sort.Strings(fieldNames)
	return fieldNames
}

type schemaFlags struct {
	canAttribute bool
	canBlock     bool
}

func (sf schemaFlags) GoString() string {
	return fmt.Sprintf("schema: Attr %v, Block: %v}", sf.canAttribute, sf.canBlock)
}

func schemaCan(st schema_j5pb.IsField_Type) schemaFlags {
	switch st.(type) {
	case *schema_j5pb.Field_Bool,
		*schema_j5pb.Field_Bytes,
		*schema_j5pb.Field_String_,
		*schema_j5pb.Field_Date,
		*schema_j5pb.Field_Timestamp,
		*schema_j5pb.Field_Decimal,
		*schema_j5pb.Field_Float,
		*schema_j5pb.Field_Integer,
		*schema_j5pb.Field_Key:
		return schemaFlags{canAttribute: true, canBlock: false}

	case *schema_j5pb.Field_Any:
		return schemaFlags{canAttribute: false, canBlock: false}

	case *schema_j5pb.Field_Array:
		return schemaFlags{canAttribute: false, canBlock: true}

	case *schema_j5pb.Field_Object:
		return schemaFlags{canAttribute: false, canBlock: true}

	case *schema_j5pb.Field_Map:
		return schemaFlags{canAttribute: false, canBlock: true}

	case *schema_j5pb.Field_Enum:
		return schemaFlags{canAttribute: true, canBlock: false}

	case *schema_j5pb.Field_Oneof:
		return schemaFlags{canAttribute: false, canBlock: true}
	default:
		return schemaFlags{}
	}
}
