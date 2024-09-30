package parser

import (
	"fmt"
)

type FragmentKind int

const (
	AssignmentFragment FragmentKind = iota
	BlockHeaderFragment
	BlockCloseFragment
	DescriptionFragment
)

type Fragment interface {
	fmt.GoStringer
	Source() SourceNode
	Kind() FragmentKind
}

type StatementType string

const (
	// Compound Statements, consist of multiple fragments
	BlockStatement       StatementType = "block"
	DeclarationStatement StatementType = "declaration"

	// Fragments which are also Statements
	AssignmentStatement  StatementType = "assignment"
	CommentStatement     StatementType = "comment"
	DescriptionStatement StatementType = "description"
)

type Statement interface {
	StatementType() StatementType
	Source() SourceNode
}

type CloseBlock struct {
	SourceNode
	Token Token
}

var _ Fragment = CloseBlock{}

func (cb CloseBlock) Kind() FragmentKind {
	return BlockCloseFragment
}

func (cb CloseBlock) GoString() string {
	return "<CloseBlock>"
}

type Comment struct {
	Value string
	SourceNode
	Token Token
}

var _ Fragment = Comment{}
var _ Statement = &Comment{}

func (c Comment) GoString() string {
	return fmt.Sprintf("comment(%s)", c.Value)
}

func (c *Comment) StatementType() StatementType {
	return CommentStatement
}

func (c Comment) Kind() FragmentKind {
	return AssignmentFragment
}

type Description struct {
	Value string
	SourceNode
	Tokens []Token
}

var _ Fragment = Description{}
var _ Statement = &Description{}

func (d Description) GoString() string {
	return fmt.Sprintf("description(%s)", d.Value)
}

func (d *Description) StatementType() StatementType {
	return DescriptionStatement
}

func (d Description) Kind() FragmentKind {
	return DescriptionFragment
}

type Assignment struct {
	Key   Reference
	Value Value
	SourceNode
	Append bool // If += was used, otherwise =
}

var _ Statement = &Assignment{}
var _ Fragment = Assignment{}

func (a *Assignment) StatementType() StatementType {
	return AssignmentStatement
}

func (a Assignment) Kind() FragmentKind {
	return AssignmentFragment
}

func (a Assignment) GoString() string {
	if a.Append {
		return fmt.Sprintf("assign(%s += %#v)", a.Key, a.Value)
	}
	return fmt.Sprintf("assign(%s = %#v)", a.Key, a.Value)
}

type BlockHeader struct {
	Type        Reference // all of the name tags, including the first 'type' tag
	Tags        []TagValue
	Qualifiers  []TagValue   // Any of the `:qualifier` tags at the end
	Description *Description // A single | description block
	Open        bool         // 'block' is opened with a {

	SourceNode
}

var _ Fragment = BlockHeader{}

func (bh BlockHeader) Kind() FragmentKind {
	return BlockHeaderFragment
}

func (bh BlockHeader) DescriptionString() string {
	if bh.Description == nil {
		return ""
	}

	return bh.Description.Value

}

func (bs BlockHeader) GoString() string {
	if bs.Open {
		return fmt.Sprintf("block(%s, %#v) <OpenBlock>", bs.Type, bs.Tags)
	}
	return fmt.Sprintf("block(%s, %#v)", bs.Type, bs.Tags)
}

func (bs BlockHeader) RootName() string {
	return bs.Type.String()
}

type Block struct {
	BlockHeader
	Body Body
}

var _ Statement = &Block{}

func (b *Block) StatementType() StatementType {
	return BlockStatement
}

type Declaration struct {
	BlockHeader
}

var _ Statement = &Declaration{}

func (d *Declaration) StatementType() StatementType {
	return DeclarationStatement
}
