package ast

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/internal/lexer"
)

// Ident is a simple name used when declaring a type, or as parts of a
// reference.
type Ident struct {
	Token lexer.Token
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
