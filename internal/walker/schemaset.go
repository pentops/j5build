package walker

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/internal/reflwrap"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
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
	TagTypeBlock

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

	StringCase StringCase

	SplitRef [][]string

	// When append or replace, walk this path to find the new context node
	SubContext []string
}

func (t *Tag) Validate(tagType TagType) error {
	if tagType >= _lastType || tagType <= _noTag {
		return fmt.Errorf("invalid TagType: %d", tagType)
	}
	if t.StringCase > _lastCase {
		return fmt.Errorf("invalid StringCase: %d", t.StringCase)
	}
	if len(t.Path) == 0 && len(t.SplitRef) == 0 {
		return fmt.Errorf("Path or SplitRef are required")
	}

	if tagType == TagTypeBlock {
		if len(t.SplitRef) > 0 {
			return fmt.Errorf("SplitRef not valid for block types")
		}
	}

	return nil
}

func (t Tag) GoString() string {
	sb := &strings.Builder{}
	sb.WriteString("Tag(")
	sb.WriteString("Path: ")
	sb.WriteString(strings.Join(t.Path, "."))

	if t.StringCase != StringCaseNone {
		sb.WriteString(", ")
		sb.WriteString("StringCase: ")
		sb.WriteString(stringCaseStrings[t.StringCase])
	}
	if len(t.SplitRef) > 0 {
		sb.WriteString(", ")
		sb.WriteString("SplitRef: ")
		sb.WriteString(fmt.Sprintf("%#v", t.SplitRef))
	}
	if len(t.SubContext) > 0 {
		sb.WriteString(", ")
		sb.WriteString("SubContext: ")
		sb.WriteString(fmt.Sprintf("%#v", t.SubContext))
	}
	sb.WriteString(")")

	return sb.String()
}

type specSource string

const (
	specSourceAuto   specSource = "reflect"
	specSourceSchema specSource = "global"
)

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
	Location    PathSpec // Field to place the source location info

	Attributes map[string]PathSpec
	Blocks     map[string]PathSpec

	Name       *Tag
	TypeSelect *Tag

	Qualifier *Tag // A qualifier maps to a new child block at this field

	// A list of paths to include when searching for blocks
	//IncludeNestedContext []string

	OnlyDefinedBlocks bool // Only allows blocks explicitly defined in Spec, otherwise merges all available in the schema
}

func (bs *BlockSpec) errName() string {
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
		err := bs.TypeSelect.Validate(TagTypeBlock)
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

func (ss *SchemaSet) _buildSpec(node reflwrap.ContainerField) (*BlockSpec, error) {
	schemaName := node.SchemaName()
	spec := ss.givenSpecs[schemaName]
	if spec == nil {
		spec = &BlockSpec{
			source: specSourceAuto,
		}
	} else {
		spec.source = specSourceSchema
	}
	spec.schema = schemaName

	//for blockName, block := range spec.Blocks {
	//	}

	if spec.OnlyDefinedBlocks {
		return spec, nil
	}

	if spec.Blocks == nil {
		spec.Blocks = map[string]PathSpec{}
	}

	if spec.Attributes == nil {
		spec.Attributes = map[string]PathSpec{}
	}

	foundContainers := map[string]PathSpec{}
	foundScalars := map[string]PathSpec{}

	err := node.RangeProperties(func(prop j5reflect.Property) error {
		schema := prop.Schema()
		switch field := schema.Schema.Type.(type) {
		case *schema_j5pb.Field_Object:
			name := objectName(field.Object)
			if name != "" {
				foundContainers[name] = PathSpec{name}
			}

		case *schema_j5pb.Field_String_:
			if schema.Name == "name" && spec.Name == nil {
				spec.Name = &Tag{
					Path: []string{"name"},
				}
			}
			if schema.Name == "description" && len(spec.Description) == 0 {
				spec.Description = []string{"description"}
			}

			foundScalars[schema.Name] = []string{schema.Name}

		case *schema_j5pb.Field_Array:
			items := field.Array.Items
			switch itemSchema := items.Type.(type) {
			case *schema_j5pb.Field_Object:
				name := objectName(itemSchema.Object)
				if name != "" {
					foundContainers[name] = PathSpec{name}
				}
			}

		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	for name, block := range foundContainers {
		if _, ok := spec.Blocks[name]; !ok {
			spec.Blocks[name] = block
		}
	}

	for name, attr := range foundScalars {
		if _, ok := spec.Attributes[name]; !ok {
			spec.Attributes[name] = attr
		}
	}

	return spec, nil
}

func (ss *SchemaSet) blockSpec(node reflwrap.ContainerField) (*BlockSpec, error) {
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
