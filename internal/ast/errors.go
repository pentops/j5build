package ast

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

func tokenErrf(tok lexer.Token, format string, args ...interface{}) error {
	return &errpos.Err{
		Pos: &tok.Start,
		Err: fmt.Errorf(format, args...),
	}
}

func unexpectedToken(tok lexer.Token, expected ...lexer.TokenType) error {
	return &unexpectedTokenError{
		tok:      tok,
		expected: expected,
	}
}

type unexpectedTokenError struct {
	tok      lexer.Token
	expected []lexer.TokenType
}

func (e *unexpectedTokenError) Error() string {
	return fmt.Sprintf("%s %s", e.tok.Start, e.msg())
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
	pos := e.tok.Start
	return &pos
}

func (e *unexpectedTokenError) WithoutPosition() error {
	return errors.New(e.msg())
}
