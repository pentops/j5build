package schema

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type TagType int

const (
	_noTag TagType = iota

	// Set the scalar value at Path to the value of Tag. Oneofs are allowed at
	// the leaf, the default value of the property matching the tag is set.
	TagTypeScalar

	// The leaf node at Path + Tag must be a container. Used on 'type select'
	// fields
	TagTypeTypeSelect

	// Leaf can be either type.
	// If it is a container, it must have a property matching the given name.
	// Then the container is included in the search path for attributes and
	// blocks.
	// If it is a scalar, the value is set, the search path does not change.
	// If it is a SplitRef scalar, the value is set, and if there are any
	// remaining blocks in the item they are added to the search path.
	TagTypeQualifier

	_lastType
)

type StringCase int

const (
	// No change to the case of the string
	StringCaseNone StringCase = iota

	StringCaseScreamingSnake
	StringCaseLowerCamel

	_lastCase
)

var stringCaseStrings = map[StringCase]string{
	StringCaseNone:           "lEavE aS-iS",
	StringCaseScreamingSnake: "SCREAMING_SNAKE",
	StringCaseLowerCamel:     "lowerCamel",
}

type Tag struct {
	Path []string

	IsBlock bool

	StringCase StringCase

	SplitRef [][]string
}

func (t *Tag) Validate(tagType TagType) error {
	if tagType >= _lastType || tagType <= _noTag {
		return fmt.Errorf("invalid TagType: %d", tagType)
	}
	if t.StringCase > _lastCase {
		return fmt.Errorf("invalid StringCase: %d", t.StringCase)
	}

	if tagType == TagTypeTypeSelect {
		if len(t.SplitRef) > 0 {
			return fmt.Errorf("SplitRef not valid for TypeSelect")
		}
	} else {
		if len(t.Path) == 0 && len(t.SplitRef) == 0 {
			return fmt.Errorf("Path or SplitRef are required")
		}
		if t.IsBlock && tagType == TagTypeScalar {
			return fmt.Errorf("IsBlock not valid for Scalar")
		}
	}
	return nil
}

func (t Tag) GoString() string {
	sb := &strings.Builder{}
	sb.WriteString("Tag(")
	split := false
	if len(t.Path) > 0 {
		sb.WriteString("Path: ")
		sb.WriteString(strings.Join(t.Path, "."))
		split = true
	}

	if t.StringCase != StringCaseNone {
		if split {
			sb.WriteString(", ")
		}
		sb.WriteString("StringCase: ")
		sb.WriteString(stringCaseStrings[t.StringCase])
		split = true

	}
	if len(t.SplitRef) > 0 {
		if split {
			sb.WriteString(", ")
		}
		sb.WriteString("SplitRef(")
		for idx, split := range t.SplitRef {
			if idx > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(strings.Join(split, "."))
		}
		sb.WriteString(")")
	}
	sb.WriteString(")")

	return sb.String()
}

type specSource string

const (
	specSourceAuto   specSource = "reflect"
	specSourceSchema specSource = "global"
)

type ChildSpec struct {
	Path         PathSpec
	IsContainer  bool
	IsScalar     bool
	IsCollection bool
}

func (cs ChildSpec) TagString() string {
	prefix := []rune{'-', '-', '-'}
	if cs.IsContainer {
		prefix[0] = 'C'
	}
	if cs.IsScalar {
		prefix[1] = 'S'
	}
	if cs.IsCollection {
		prefix[2] = 'A'
	}
	return string(prefix)
}

type PathSpec []string

func (sp PathSpec) GoString() string {
	return fmt.Sprintf("PathSpec(%s)", strings.Join(sp, "."))
}

// Defines customizations for a 'type', these should be set in the schema
type BlockSpec struct {
	DebugName string // Prints as context to the user

	source specSource // Set by the parser, notes on how the spec came to be
	schema string     // Set by the parser

	Description PathSpec // Field to place the description in

	Children map[string]ChildSpec

	Name       *Tag
	TypeSelect *Tag

	Qualifier *Tag // A qualifier maps to a new child block at this field

	// A list of paths to include when searching for blocks
	//IncludeNestedContext []string

	OnlyDefined bool // Only allows blocks and attributes explicitly defined in Spec, otherwise merges all available in the schema

	// Callback to run after closing the block, to run validation, automatic
	// cleanup etc.
	RunAfter BlockHook
}

type BlockHookFunc func(j5reflect.ContainerField) error

func (bh BlockHookFunc) RunHook(cf j5reflect.ContainerField) error {
	return bh(cf)
}

type BlockHook interface {
	RunHook(j5reflect.ContainerField) error
}

func (bs *BlockSpec) ErrName() string {
	if bs.DebugName != "" {
		return fmt.Sprintf("%s from %s as %q", bs.schema, bs.source, bs.DebugName)
	}
	return fmt.Sprintf("%s from %s", bs.schema, bs.source)
}

func (bs *BlockSpec) Validate() error {
	if bs == nil {
		// Nil is fine, allows for aliases without specification
		return nil
	}
	if bs.Name != nil {
		err := bs.Name.Validate(TagTypeScalar)
		if err != nil {
			return fmt.Errorf("name: %s", err)
		}
	}

	if bs.TypeSelect != nil {
		err := bs.TypeSelect.Validate(TagTypeTypeSelect)
		if err != nil {
			return fmt.Errorf("typeSelect: %s", err)
		}
	}

	if bs.Qualifier != nil {
		err := bs.Qualifier.Validate(TagTypeQualifier)
		if err != nil {
			return fmt.Errorf("qualifier: %w", err)
		}
	}
	return nil
}

type SchemaSet struct {
	givenSpecs  map[string]*BlockSpec
	cachedSpecs map[string]*BlockSpec
}

type ConversionSpec struct {
	GlobalDefs map[string]*BlockSpec
}

func (sc *ConversionSpec) Validate() error {
	for name, spec := range sc.GlobalDefs {
		err := spec.Validate()
		if err != nil {
			return fmt.Errorf("GlobalDefs[%q]: %w", name, err)
		}
	}
	return nil
}

func (ss *SchemaSet) _buildSpec(node j5reflect.PropertySet) (*BlockSpec, error) {
	schemaName := node.SchemaName()
	blockSpec := ss.givenSpecs[schemaName]
	if blockSpec == nil {
		blockSpec = &BlockSpec{
			source: specSourceAuto,
		}
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
			items := field.Array.Items
			switch itemSchema := items.Type.(type) {
			case *schema_j5pb.Field_Object:
				name := objectName(itemSchema.Object)
				if name != "" {
					spec.IsCollection = true
					spec.IsContainer = true
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

func (ss *SchemaSet) wrapContainer(node j5reflect.PropertySet, path []string, loc *sourcedef_j5pb.SourceLocation) (*containerField, error) {
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

func objectName(obj *schema_j5pb.ObjectField) string {
	var name string
	if ref := obj.GetRef(); ref != nil {
		name = ref.Schema
	} else if inline := obj.GetObject(); obj != nil {
		name = inline.Name
	}
	return strcase.ToLowerCamel(name)
}
