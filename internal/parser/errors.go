package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
)

var HadErrors = fmt.Errorf("had errors, see Walker.Errors")

func unexpectedToken(tok Token, expected ...TokenType) *unexpectedTokenError {
	return &unexpectedTokenError{
		tok:      tok,
		expected: expected,
	}
}

type unexpectedTokenError struct {
	tok      Token
	expected []TokenType
	context  string
}

func (e *unexpectedTokenError) Error() string {
	if e.context == "" {
		return fmt.Sprintf("%s %s", e.tok.Start, e.msg())
	}
	return fmt.Sprintf("%s %s %s", e.tok.Start, e.msg(), e.context)
}

func (e *unexpectedTokenError) msg() string {
	if len(e.expected) == 1 {
		return fmt.Sprintf("unexpected %s, want %s", e.tok, e.expected[0])
	}
	expectSet := make([]string, len(e.expected))
	for i, e := range e.expected {
		expectSet[i] = e.String()
	}
	return fmt.Sprintf("unexpected %s, want one of %s", e.tok, strings.Join(expectSet, ", "))
}

func (e *unexpectedTokenError) ErrorPosition() *errpos.Position {
	return &errpos.Position{
		Start: errpos.Point{
			Line:   e.tok.Start.Line,
			Column: e.tok.Start.Column,
		},
		End: errpos.Point{
			Line:   e.tok.End.Line,
			Column: e.tok.End.Column,
		},
	}
}

func (e *unexpectedTokenError) WithoutPosition() error {
	return errors.New(e.msg())
}
