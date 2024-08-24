package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type File struct {
	Package string
	Imports []Import
	BlockStatement
}

type Comment struct {
	Text string
}

type SourceNode struct {
	Start   errpos.Position
	End     errpos.Position
	Comment *Comment
}

func (sn SourceNode) Source() SourceNode {
	return sn
}

type Import struct {
	Path  string
	Alias string
}

type Body struct {
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
type Reference []Ident

func (r Reference) AsStringValue() Value {
	return Value{
		SourceNode: SourceNode{
			Start: r[0].Start,
			End:   r[len(r)-1].End,
		},
		token: lexer.Token{
			Start: r[0].Start,
			End:   r[len(r)-1].End,
			Type:  lexer.STRING,
			Lit:   r.String(),
		},
	}

}

func (r Reference) String() string {
	return strings.Join(r.Strings(), ".")
}

func (r Reference) Strings() []string {
	out := make([]string, len(r))
	for i, part := range r {
		out[i] = part.Value
	}
	return out
}

func ReferencesToStrings(refs []Reference) []string {
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = ref.String()
	}
	return out
}

// AsValue converts the reference to a Value type, which is used when it is on
// the RHS of an assignment or directive.
func (r Reference) AsValue() Value {
	return Value{
		SourceNode: SourceNode{
			Start: r[0].Start,
			End:   r[len(r)-1].End,
		},
		token: lexer.Token{
			Type:  lexer.IDENT,
			Start: r[0].Start,
			End:   r[len(r)-1].End,
			Lit:   r.String(),
		},
	}
}

func (r Reference) GoString() string {
	return fmt.Sprintf("reference(%s)", r)
}

type BlockHeader struct {
	Name        []Reference // all of the name tags, including the first 'type' tag
	Qualifiers  []Reference // Any of the `:qualifier` tags at the end
	Description *Value      // A single | description block

	Export bool
	SourceNode
}

// ScanTags scans the tags after the first 'type' tag, all elements must be
// single Ident, not joined references. (for other cases parse it directly)
func (bs BlockHeader) ScanTags(into ...*string) error {
	wantLen := 1 + len(into)
	// idx 0 is the type
	if len(bs.Name) != wantLen {
		return fmt.Errorf("expected %d tags, got %v", wantLen, bs.Name) //ast.ReferencesToStrings(tags))
	}

	for idx, dest := range into {
		tag := bs.Name[1+idx]
		if len(tag) != 1 {
			return fmt.Errorf("expected single tag, got %v", tag)
		}
		str := tag[0].String()
		*dest = str
	}

	return nil
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
	statement
}

type Statement interface {
	fmt.GoStringer
	Source() SourceNode
	isStatement()
}

type statement struct{}

func (s statement) isStatement() {}

type Assignment struct {
	Key   Reference
	Value Value
	SourceNode
	statement
}

func (a Assignment) GoString() string {
	return fmt.Sprintf("assign(%s = %#v)", a.Key, a.Value)
}

type Directive struct {
	Key   Reference
	Value *Value
	SourceNode
	statement
}

func (d Directive) GoString() string {
	return fmt.Sprintf("directive(%s %#v)", d.Key, d.Value)
}

type TypeError struct {
	Expected string
	Got      string
}

func (te *TypeError) Error() string {
	return fmt.Sprintf("expected a %s, got %s", te.Expected, te.Got)
}

type Value struct {
	token lexer.Token
	SourceNode
}

func (v Value) GoString() string {
	return fmt.Sprintf("value(%s:%s)", v.token.Type, v.token.Lit)
}

func (v Value) AsString() (string, error) {
	if v.token.Type != lexer.STRING && v.token.Type != lexer.DESCRIPTION && v.token.Type != lexer.IDENT {

		return "", &TypeError{
			Expected: "string",
			Got:      v.token.String(),
		}
	}
	return v.token.Lit, nil
}

func (v Value) AsBoolean() (bool, error) {
	if v.token.Type != lexer.BOOL {
		return false, &TypeError{
			Expected: "bool",
			Got:      v.token.String(),
		}
	}
	return v.token.Lit == "true", nil
}

func (v Value) AsUint(size int) (uint64, error) {
	if v.token.Type != lexer.INT {
		return 0, &TypeError{
			Expected: fmt.Sprintf("uint%d", size),
			Got:      v.token.String(),
		}
	}
	parsed, err := strconv.ParseUint(v.token.Lit, 10, size)
	return parsed, err

}

func (v Value) AsInt(size int) (int64, error) {
	if v.token.Type != lexer.INT {
		return 0, &TypeError{
			Expected: fmt.Sprintf("int%d", size),
			Got:      v.token.String(),
		}
	}
	parsed, err := strconv.ParseInt(v.token.Lit, 10, size)
	return parsed, err
}

func (v Value) AsFloat(size int) (float64, error) {
	switch v.token.Type {
	case lexer.INT:
		parsed, err := strconv.ParseFloat(v.token.Lit, size)
		return parsed, err
	case lexer.DECIMAL:
		parsed, err := strconv.ParseFloat(v.token.Lit, size)
		return parsed, err
	default:
		return 0, &TypeError{
			Expected: fmt.Sprintf("float%d", size),
			Got:      v.token.String(),
		}
	}
}
