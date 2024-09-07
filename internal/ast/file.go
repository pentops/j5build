package ast

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type File struct {
	//Package string

	Body Body

	Errors errpos.Errors
}

type Error struct {
	Err error
	Pos errpos.Position
}

type TypeError struct {
	Expected string
	Got      string
}

func (te *TypeError) Error() string {
	return fmt.Sprintf("expected a %s, got %s", te.Expected, te.Got)
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
	Statements []Statement
}
