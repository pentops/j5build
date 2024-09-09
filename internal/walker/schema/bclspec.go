package schema

import (
	"fmt"
	"strings"

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

type Tag struct {
	Path     []string
	BangPath []string

	IsBlock bool
}

func (t *Tag) Validate(tagType TagType) error {
	if tagType >= _lastType || tagType <= _noTag {
		return fmt.Errorf("invalid TagType: %d", tagType)
	}

	if tagType == TagTypeTypeSelect {
	} else {
		if t.IsBlock && tagType == TagTypeScalar {
			return fmt.Errorf("Tag IsBlock not valid for Scalar")
		}
	}
	return nil
}

func (t Tag) GoString() string {
	sb := &strings.Builder{}
	sb.WriteString("Tag(")
	if len(t.Path) > 0 {
		sb.WriteString("Path: ")
		sb.WriteString(strings.Join(t.Path, "."))
	}

	sb.WriteString(")")

	return sb.String()
}

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

	source      specSource // Set by the parser, notes on how the spec came to be
	schema      string     // Set by the parser
	Description PathSpec   // Field to place the description in

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

	ScalarSplit *ScalarSplit
}

type ScalarSplit struct {
	Delimiter   *string
	RightToLeft bool

	Required  []PathSpec
	Optional  []PathSpec
	Remainder *PathSpec
}

type BlockHookFunc func(j5reflect.ContainerField) error

func (bh BlockHookFunc) RunHook(cf j5reflect.ContainerField) error {
	return bh(cf)
}

type BlockHook interface {
	RunHook(j5reflect.ContainerField) error
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
