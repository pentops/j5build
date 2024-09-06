package ast

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type File struct {
	//Package string

	Body Body

	Errors errpos.Errors
}

/*
	func (f *File) Imports() []*ImportStatement {
		stmts := make([]*ImportStatement, 0, len(f.Body.Statements))
		for _, stmt := range f.Body.Statements {
			if is, ok := stmt.(*ImportStatement); ok {
				stmts = append(stmts, is)
			}

		}
		return stmts
	}
*/
func (f *File) Statements() []Statement {
	return f.Body.Statements
}

type Error struct {
	Err error
	Pos errpos.Position
}

type Comment struct {
	Text string
}

type SourceNode struct {
	Start   lexer.Position
	End     lexer.Position
	Comment *Comment
}

func (sn SourceNode) Position() errpos.Position {
	return errpos.Position{
		Start: sn.Start,
		End:   sn.End,
	}
}

func (sn SourceNode) Source() SourceNode {
	return sn
}

type Body struct {
	IsRoot     bool
	Includes   []Reference
	Statements []Statement
}

// Ident is a simple name used when declaring a type, or as parts of a
// reference.
type Ident struct {
	Value string
	SourceNode
}

func (i Ident) String() string {
	return i.Value
}

func (i Ident) GoString() string {
	return fmt.Sprintf("ident(%s)", i.Value)
}

func (i Ident) AsStringValue() Value {
	return Value{
		SourceNode: i.SourceNode,
		token: lexer.Token{
			Start: i.Start,
			End:   i.End,
			Type:  lexer.STRING,
			Lit:   i.Value,
		},
	}
}

// Reference is a dot separates set of Idents
type Reference struct {
	Idents []Ident
	SourceNode
	unknownValue
}

func NewReference(idents []Ident) Reference {
	return Reference{
		unknownValue: unknownValue{
			typeName: "reference",
		},
		Idents: idents,
		SourceNode: SourceNode{
			Start: idents[0].Start,
			End:   idents[len(idents)-1].End,
		},
	}
}

func (r Reference) GoString() string {
	return fmt.Sprintf("reference(%s)", r)
}

func (r Reference) String() string {
	return strings.Join(r.Strings(), ".")
}

func (r Reference) AsString() (string, error) {
	return r.String(), nil
}

func (r Reference) Strings() []string {
	out := make([]string, len(r.Idents))
	for i, part := range r.Idents {
		out[i] = part.Value
	}
	return out
}

type BlockHeader struct {
	Name        []Reference // all of the name tags, including the first 'type' tag
	Qualifiers  []Reference // Any of the `:qualifier` tags at the end
	Description *Value      // A single | description block

	Export bool
	SourceNode
}

func (bh BlockHeader) DescriptionString() string {
	if bh.Description == nil {
		return ""
	}

	return bh.Description.token.Lit

}

func (bs BlockHeader) GoString() string {
	return fmt.Sprintf("block(%s)", bs.Name)
}

func (bs BlockHeader) RootName() string {
	if len(bs.Name) == 0 {
		return ""
	}
	return bs.Name[0].String()
}

func (bs BlockHeader) NamePart(idx int) (string, bool) {
	if idx >= len(bs.Name) {
		return "", false
	}
	return bs.Name[idx].String(), true
}

type BlockStatement struct {
	BlockHeader
	Body Body
}

func (bs BlockStatement) GoString() string {
	return fmt.Sprintf("block(%s)", bs.Name)
}

func (bs BlockStatement) Kind() StatementKind {
	return StatementKindBlock
}

var _ Statement = BlockStatement{}

type TypeError struct {
	Expected string
	Got      string
}

func (te *TypeError) Error() string {
	return fmt.Sprintf("expected a %s, got %s", te.Expected, te.Got)
}

type StatementKind int

const (
	StatementKindEOF StatementKind = iota
	StatementKindAssignment
	StatementKindBlockHeader
	StatementKindBlockClose
	//StatementKindImport
	//StatementKindPackage

	StatementKindBlock // allows a whole 'block' to act as a statement

)

type Statement interface {
	fmt.GoStringer
	Source() SourceNode
	Kind() StatementKind
}

/*
type ImportStatement struct {
	// when true, the import was specified as a quoted file path, rather than a package, for compatibility with proto imports
	IsFile bool
	Path   string
	Alias  string
	SourceNode
}

func (is ImportStatement) Kind() StatementKind {
	return StatementKindImport
}

func (is ImportStatement) GoString() string {
	if is.IsFile {
		return fmt.Sprintf("import(%q)", is.Path)
	}
	if is.Alias != "" {
		return fmt.Sprintf("import(%s as %s)", is.Path, is.Alias)
	}
	return fmt.Sprintf("import(%s)", is.Path)
}

var _ Statement = ImportStatement{}

type PackageStatement struct {
	Name string
	SourceNode
}

var _ Statement = PackageStatement{}

func (ps PackageStatement) Kind() StatementKind {
	return StatementKindPackage
}

func (ps PackageStatement) GoString() string {
	return fmt.Sprintf("package(%s)", ps.Name)
}
*/

type Assignment struct {
	Key   Reference
	Value Value
	SourceNode
}

func (a Assignment) Kind() StatementKind {
	return StatementKindAssignment
}

func (a Assignment) GoString() string {
	return fmt.Sprintf("assign(%s = %#v)", a.Key, a.Value)
}

/*
type Directive struct {
	Key   Reference
	Value *Value
	SourceNode
}

func (d Directive) Kind() StatementKind {
	return StatementKindDirective
}

func (d Directive) GoString() string {
	return fmt.Sprintf("directive(%s %#v)", d.Key, d.Value)
}*/
