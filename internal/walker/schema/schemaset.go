package schema

import (
	"fmt"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type specSource string

const (
	specSourceAuto   specSource = "reflect"
	specSourceSchema specSource = "global"
)

func (bs *BlockSpec) ErrName() string {
	if bs.DebugName != "" {
		return fmt.Sprintf("%s from %s as %q", bs.schema, bs.source, bs.DebugName)
	}
	return fmt.Sprintf("%s from %s", bs.schema, bs.source)
}

type SchemaSet struct {
	givenSpecs  map[string]*BlockSpec
	cachedSpecs map[string]*BlockSpec
}

func convertTag(tag *bcl_j5pb.Tag) *Tag {
	if tag == nil {
		return nil
	}
	tt := &Tag{
		Path:    tag.Path.Path,
		IsBlock: tag.IsBlock,
	}
	if tag.BangBool != nil {
		tt.BangPath = tag.BangBool.Path
	}
	return tt
}

func convertBlocks(given []*bcl_j5pb.Block) (map[string]*BlockSpec, error) {
	givenBlocks := map[string]*BlockSpec{}
	for _, src := range given {
		children := map[string]ChildSpec{}
		for _, child := range src.Children {
			children[child.Name] = ChildSpec{
				Path:         PathSpec(child.Path.Path),
				IsContainer:  child.IsContainer,
				IsScalar:     child.IsScalar,
				IsCollection: child.IsCollection,
			}
		}

		block := &BlockSpec{
			Name:        convertTag(src.Name),
			TypeSelect:  convertTag(src.TypeSelect),
			Qualifier:   convertTag(src.Qualifier),
			OnlyDefined: src.OnlyExplicit,
			Children:    children,
		}
		if src.Description != nil {
			block.Description = src.Description.Path
		}

		if src.ScalarSplit != nil {
			ss := src.ScalarSplit
			block.ScalarSplit = &ScalarSplit{
				Delimiter:   ss.Delimiter,
				RightToLeft: ss.RightToLeft,
			}
			for _, required := range ss.RequiredFields {
				block.ScalarSplit.Required = append(block.ScalarSplit.Required, PathSpec(required.Path))
			}
			for _, optional := range ss.OptionalFields {
				block.ScalarSplit.Optional = append(block.ScalarSplit.Optional, PathSpec(optional.Path))
			}
			if ss.RemainderField != nil {
				ps := PathSpec(ss.RemainderField.Path)
				block.ScalarSplit.Remainder = &ps
			}
		}

		if err := block.Validate(); err != nil {
			return nil, fmt.Errorf("invalid block spec for %s: %s", src.SchemaName, err)
		}

		givenBlocks[src.SchemaName] = block
	}
	return givenBlocks, nil
}

func NewSchemaSet(given *bcl_j5pb.Schema) (*SchemaSet, error) {
	givenBlocks, err := convertBlocks(given.Blocks)
	if err != nil {
		return nil, err
	}

	return &SchemaSet{
		givenSpecs:  givenBlocks,
		cachedSpecs: map[string]*BlockSpec{},
	}, nil
}

func (ss *SchemaSet) _buildSpec(node j5reflect.PropertySet) (*BlockSpec, error) {
	schemaName := node.SchemaName()
	blockSpec := ss.givenSpecs[schemaName]
	if blockSpec == nil {
		blockSpec = &BlockSpec{}
		blockSpec.source = specSourceAuto
	} else {
		blockSpec.source = specSourceSchema
	}
	blockSpec.schema = schemaName

	if blockSpec.OnlyDefined {
		return blockSpec, nil
	}

	if blockSpec.Children == nil {
		blockSpec.Children = map[string]ChildSpec{}
	}

	err := node.RangeProperties(func(prop j5reflect.Property) error {

		schema := prop.Schema().ToJ5Proto()
		name := schema.Name
		spec := ChildSpec{
			Path: PathSpec{schema.Name},
		}

		switch field := schema.Schema.Type.(type) {
		case *schema_j5pb.Field_Object:
			spec.IsContainer = true

		case *schema_j5pb.Field_Oneof:
			spec.IsContainer = true

		case *schema_j5pb.Field_String_:
			if name == "name" && blockSpec.Name == nil {
				blockSpec.Name = &Tag{
					Path: []string{"name"},
				}
			}
			if name == "description" && len(blockSpec.Description) == 0 {
				blockSpec.Description = []string{"description"}
			}

			spec.IsScalar = true

		case *schema_j5pb.Field_Bool,
			*schema_j5pb.Field_Integer,
			*schema_j5pb.Field_Float,
			*schema_j5pb.Field_Key,
			*schema_j5pb.Field_Enum,
			*schema_j5pb.Field_Bytes,
			*schema_j5pb.Field_Date,
			*schema_j5pb.Field_Timestamp,
			*schema_j5pb.Field_Decimal:

			spec.IsScalar = true

		case *schema_j5pb.Field_Array:
			if field.Array != nil && field.Array.Ext != nil && field.Array.Ext.SingleForm != nil {
				spec.IsScalar = true
				name = *field.Array.Ext.SingleForm
			}
			spec.IsCollection = true

			items := field.Array.Items
			switch itemSchema := items.Type.(type) {
			case *schema_j5pb.Field_Object:
				spec.IsContainer = true
				if name == "" {
					name = arrayName(itemSchema.Object)
				}
			}

		default:
			return fmt.Errorf("unimplemented schema type: %T", field)
		}

		if _, ok := blockSpec.Children[name]; !ok {
			blockSpec.Children[name] = spec
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return blockSpec, nil
}

func (ss *SchemaSet) wrapContainer(node j5reflect.PropertySet, path []string, loc *bcl_j5pb.SourceLocation) (*containerField, error) {
	spec, err := ss.blockSpec(node)
	if err != nil {
		return nil, err
	}

	return &containerField{
		schemaName: node.SchemaName(),
		container:  node,
		spec:       *spec,
		path:       path,
		location:   loc,
	}, nil

}

func (ss *SchemaSet) blockSpec(node j5reflect.PropertySet) (*BlockSpec, error) {
	schemaName := node.SchemaName()

	var err error
	spec, ok := ss.cachedSpecs[schemaName]
	if !ok {
		spec, err = ss._buildSpec(node)
		if err != nil {
			return nil, err
		}
		ss.cachedSpecs[schemaName] = spec
	}

	return spec, nil
}

func arrayName(obj *schema_j5pb.ObjectField) string {

	var name string
	if ref := obj.GetRef(); ref != nil {
		name = ref.Schema
	} else if inline := obj.GetObject(); obj != nil {
		name = inline.Name
	}
	return strcase.ToLowerCamel(name)
}
