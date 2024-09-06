package ast

import (
	"errors"
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type Position = errpos.Point

func ParseFile(input string, failFast bool) (*File, error) {
	l := lexer.NewLexer(input)

	tokens, ok, err := l.AllTokens(failFast)
	if err != nil {
		return nil, fmt.Errorf("unexpected lexer error: %w", err)
	}
	if !ok {
		return nil, errpos.AddSource(l.Errors, input)

	}
	tree, err := Walk(tokens, failFast)
	if err != nil {
		if err == HadErrors {
			return tree, errpos.AddSource(tree.Errors, input)
		}
		return tree, fmt.Errorf("unexpected walk error: %w", err)
	}

	return tree, nil
}

var HadErrors = fmt.Errorf("had errors, see Walker.Errors")

type Walker struct {
	tokens   []lexer.Token
	offset   int
	failFast bool
	eof      bool

	errors errpos.Errors
}

func (w *Walker) addError(err *unexpectedTokenError) {
	w.errors = append(w.errors, &errpos.Err{
		Pos: err.ErrorPosition(),
		Err: errors.New(err.msg()),
	})
}

func Walk(tokens []lexer.Token, failFast bool) (*File, error) {
	ww := &Walker{
		tokens:   tokens,
		failFast: failFast,
	}
	out, err := ww.walkFile()
	out.Errors = ww.errors
	if err != nil {
		return out, err
	}

	if len(ww.errors) > 0 {
		return out, HadErrors
	}
	return out, nil
}

func (w *Walker) currentPos() lexer.Position {
	if w.offset == 0 {
		return lexer.Position{}
	}
	return w.tokens[w.offset-1].End
}

func (ww *Walker) popToken() lexer.Token {
	if ww.offset >= len(ww.tokens) {
		eofToken := ww.tokens[len(ww.tokens)-1]
		if eofToken.Type == lexer.EOF {
			return eofToken
		}
		// Indicates a bug but let's go with it
		return lexer.Token{
			Type:  lexer.EOF,
			Lit:   "",
			Start: eofToken.End,
			End:   eofToken.End,
		}
	}
	tok := ww.tokens[ww.offset]
	ww.offset++
	return tok
}

func (ww *Walker) nextType() lexer.TokenType {
	if ww.offset >= len(ww.tokens) {
		return lexer.EOF
	}
	return ww.tokens[ww.offset].Type
}

func (ww *Walker) walkFile() (*File, error) {
	file := &File{
		Body: Body{
			IsRoot: true,
		},
	}
	for {
		err := ww.nextFileStatement(file)
		if err != nil {
			if err := ww.recoverError(err); err != nil {
				return file, err
			}

		}
		if ww.eof {
			break
		}
	}
	return file, nil
}

func (ww *Walker) recoverError(err *unexpectedTokenError) error {
	ww.addError(err)

	if ww.failFast {
		return HadErrors

	}

	// Skip to the next EOL
	for {
		fmt.Printf("skip %s\n", ww.nextType())
		if ww.nextType() == lexer.EOL || ww.nextType() == lexer.EOF {
			ww.popToken()
			break
		}
		ww.popToken()
	}
	return nil
}

func (ww *Walker) nextFileStatement(file *File) *unexpectedTokenError {
	switch ww.nextType() {
	case lexer.EOF:
		ww.eof = true
		return nil
		/*
			case lexer.PACKAGE:
				tok := ww.popToken()
				value, err := ww.popReference()
				if err != nil {
					return err
				}
				decl := PackageStatement{
					Name: value.String(),
					SourceNode: SourceNode{
						Start: tok.Start,
						End:   value.End,
					},
				}
				if err := ww.popEOL(); err != nil {
					return err
				}
				file.Body.Statements = append(file.Body.Statements, decl)
				return nil

			case lexer.IMPORT:
				ww.popToken()
				decl := &ImportStatement{}

				switch ww.nextType() {
				case lexer.STRING:
					decl.Path = ww.popToken().Lit
					decl.IsFile = true
					comment, err := ww.endStatement()
					if err != nil {
						return err
					}
					if comment != nil {
						decl.Comment = comment
					}
					file.Body.Statements = append(file.Body.Statements, decl)

					return nil

				case lexer.IDENT:
					packagePath, err := ww.popReference()
					if err != nil {
						return err
					}
					decl.Path = strings.Join(packagePath.Strings(), ".")
				}

				if ww.nextType() == lexer.IDENT {
					tok := ww.popToken()
					if tok.Lit != "as" {
						return unexpectedToken(tok, lexer.IDENT)
					}

					ident, err := ww.popIdent()
					if err != nil {
						return err
					}
					decl.Alias = ident.Value
				}
				comment, err := ww.endStatement()
				if err != nil {
					return err
				}
				if comment != nil {
					decl.Comment = comment
				}

				file.Body.Statements = append(file.Body.Statements, decl)

				return nil
		*/
	default:
		err := ww.nextStatement(&file.Body)
		if err != nil {
			return err
		}
		return nil
	}
}

// WalkBody walks up until, but not including, a closing }
func (ww *Walker) walkBody(block *BlockStatement) *unexpectedTokenError {
	block.Body = Body{}
	//var err *unexpectedTokenError

	for {
		switch ww.nextType() {
		case lexer.RBRACE:
			// Do not consume, that is the job of the block statement
			return nil
			/*
				case lexer.EXPORT:
					ww.popToken()
					block.Export = true
					_, err = ww.endStatement()
					if err != nil {
						return err
					}

				case lexer.INCLUDE:
					ww.popToken()
					ref, err := ww.popReference()
					if err != nil {
						return err
					}
					block.Body.Includes = append(block.Body.Includes, ref)
					_, err = ww.endStatement()
					if err != nil {
						return err
					}
			*/
		default:
			err := ww.nextStatement(&block.Body)
			if err != nil {
				return err
			}
		}
	}
}

func (ww *Walker) nextStatement(body *Body) *unexpectedTokenError {

	switch ww.nextType() {

	case lexer.EOL:
		// Empty line
		ww.popToken()
		return nil

	case lexer.COMMENT:
		// skip comments for now
		ww.popToken()
		return nil

	case lexer.IDENT:

		// Read all dot separated idents continuing from the first token
		// a.b.c.d
		ref, err := ww.popReference()
		if err != nil {
			err.context = fmt.Sprintf("after \"%s\"", ref.String())
			return err
		}

		stmt, err := ww.walkStatement(ref)
		if err != nil {
			err.context = fmt.Sprintf("after \"%s\"", ref.String())
			return err
		}
		body.Statements = append(body.Statements, stmt)
		return nil

	default:
		return unexpectedToken(ww.popToken(), lexer.IDENT, lexer.COMMENT, lexer.EOL)
	}

}

func (ww *Walker) popEOL() *unexpectedTokenError {
	tok := ww.popToken()

	if tok.Type == lexer.COMMENT {
		return ww.popEOL()
	}

	if tok.Type != lexer.EOL && tok.Type != lexer.EOF {
		return unexpectedToken(tok, lexer.EOL)
	}
	return nil
}

func (ww *Walker) popType(tt lexer.TokenType) (lexer.Token, *unexpectedTokenError) {
	tok := ww.popToken()
	if tok.Type != tt {
		return lexer.Token{}, unexpectedToken(tok, tt)
	}
	return tok, nil
}

func (ww *Walker) popValue() (Value, *unexpectedTokenError) {
	if ww.nextType().IsLiteral() {
		token := ww.popToken()
		return Value{
			token: token,
			SourceNode: SourceNode{
				Start: token.Start,
				End:   token.End,
			},
		}, nil
	}

	if ww.nextType() == lexer.LBRACK {
		opener := ww.popToken()

		if ww.nextType() == lexer.RBRACK {
			ww.popToken()
			return Value{
				array: []Value{},
				SourceNode: SourceNode{
					Start: opener.Start,
					End:   ww.currentPos(),
				},
			}, nil
		}

		values := make([]Value, 0)
		for {

			value, err := ww.popValue()
			if err != nil {
				return Value{}, err
			}

			values = append(values, value)

			if ww.nextType() == lexer.COMMA {
				ww.popToken()
				continue
			}
			if ww.nextType() == lexer.RBRACK {
				ww.popToken()
				break
			}
			return Value{}, unexpectedToken(ww.popToken(), lexer.COMMA, lexer.RBRACK)
		}
		return Value{
			array: values,
			SourceNode: SourceNode{
				Start: opener.Start,
				End:   ww.currentPos(),
			},
		}, nil
	}

	return Value{}, unexpectedToken(ww.popToken(), lexer.AnyLiteral, lexer.LBRACK)
}

func (ww *Walker) popIdent() (Ident, *unexpectedTokenError) {
	tok, err := ww.popType(lexer.IDENT)
	if err != nil {
		return Ident{}, err
	}
	return Ident{
		Value: tok.Lit,
		SourceNode: SourceNode{
			Start: tok.Start,
			End:   tok.End,
		},
	}, nil
}

// popReference reads all dot separated idents, dot, ident etc
func (ww *Walker) popReference() (Reference, *unexpectedTokenError) {
	ref := make([]Ident, 0)
	for {
		ident, err := ww.popIdent()
		if err != nil {
			rr := NewReference(ref)
			err.context = fmt.Sprintf("after \"%s.\"", rr.String())
			return rr, err
		}
		ref = append(ref, ident)
		if ww.nextType() != lexer.DOT {
			return NewReference(ref), nil
		}
		ww.popToken()
	}

}

func (ww *Walker) walkStatement(ref Reference) (Statement, *unexpectedTokenError) {
	start := ref.SourceNode.Start

	// Assignments can only take one LHS argument
	if ww.nextType() == lexer.ASSIGN {
		// <reference> = ...
		return ww.walkValueAssign(ref)
	}

	nameParts := []Reference{ref}

	for ww.nextType() == lexer.IDENT {
		// Build the name parts
		// <reference> <ident>
		ref, err := ww.popReference()
		if err != nil {
			return nil, err
		}
		nameParts = append(nameParts, ref)
	}

	block := BlockStatement{
		BlockHeader: BlockHeader{
			Name: nameParts,
			SourceNode: SourceNode{
				Start: start,
			},
		},
	}

	for ww.nextType() == lexer.COLON {
		ww.popToken()
		// <reference>:...
		// Expect a qualifier reference, e.g an object:a.b.C
		qualifier, err := ww.popReference()
		if err != nil {
			return nil, err
		}
		block.Qualifiers = append(block.Qualifiers, qualifier)
	}

	switch ww.nextType() {

	case lexer.LBRACE:
		// <reference> { ...
		// This is a block statement
		// <reference> { <body> }
		if err := ww.walkBlockStatement(&block); err != nil {
			return nil, err
		}
		block.End = ww.currentPos()
		return block, nil

	case lexer.DESCRIPTION:
		// <reference> | description
		// This is a block statement without a body.
		tok := ww.popToken()
		block.Description = &Value{token: tok, SourceNode: SourceNode{Start: tok.Start, End: tok.End}}
		block.End = ww.currentPos()
		return block, nil

	case lexer.EOL, lexer.EOF:
		block.End = ww.currentPos()
		return block, nil

	default:
		return nil, unexpectedToken(ww.popToken(), lexer.LBRACE, lexer.EOL, lexer.DESCRIPTION, lexer.IDENT)
	}
}

func (ww *Walker) endStatement() (*Comment, *unexpectedTokenError) {
	tok := ww.popToken()

	var returnComment *Comment

	if tok.Type == lexer.COMMENT {
		returnComment = &Comment{
			Text: tok.Lit,
		}
		tok = ww.popToken()
	}

	if tok.Type == lexer.EOL || tok.Type == lexer.EOF {
		return returnComment, nil
	}

	return returnComment, unexpectedToken(tok, lexer.COMMENT, lexer.EOL)
}

func (ww *Walker) walkValueAssign(ref Reference) (Assignment, *unexpectedTokenError) {

	assign := Assignment{
		Key: ref,
		SourceNode: SourceNode{
			Start: ref.SourceNode.Start,
		},
	}

	_, err := ww.popType(lexer.ASSIGN)
	if err != nil {
		return assign, err
	}

	value, err := ww.popValue()
	if err != nil {
		return assign, err
	}

	assign.Value = value
	assign.End = value.End

	comment, err := ww.endStatement()
	if err != nil {
		return assign, err
	}
	if comment != nil {
		assign.Comment = comment
	}

	return assign, nil
}

func (ww *Walker) walkBlockStatement(block *BlockStatement) *unexpectedTokenError {

	_, err := ww.popType(lexer.LBRACE)
	if err != nil {
		return err
	}

	empty := false
	if ww.nextType() == lexer.RBRACE {
		// Empty block
		ww.popToken()
		empty = true
	}

	comment, err := ww.endStatement()
	if err != nil {
		return err
	}
	if comment != nil {
		block.Comment = comment
	}
	if empty {
		return nil
	}

	if ww.nextType() == lexer.DESCRIPTION {
		desc := ww.popToken()
		block.Description = &Value{
			token: desc,
			SourceNode: SourceNode{
				Start: desc.Start,
				End:   desc.End,
			},
		}
		// Description tokens consume the EOL
	}

	err = ww.walkBody(block)
	if err != nil {
		return err
	}

	_, err = ww.popType(lexer.RBRACE)
	if err != nil {
		return err
	}

	block.End = ww.currentPos()

	// Trailing Comments are not supported.

	return nil
}
