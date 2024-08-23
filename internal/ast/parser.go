package ast

import (
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

type Walker struct {
	tokens []lexer.Token
	offset int
}

func Walk(tokens []lexer.Token) (*File, error) {
	ww := &Walker{
		tokens: tokens,
	}

	out, err := ww.walkFile()
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	return out, nil
}

func (w *Walker) currentPos() errpos.Position {
	if w.offset == 0 {
		return errpos.Position{}
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
	file := &File{}
	for {
		switch ww.nextType() {
		case lexer.EOF:
			return file, nil

		case lexer.PACKAGE:
			ww.popToken()
			value, err := ww.popReference()
			if err != nil {
				return file, err
			}
			file.Package = value.String()
			if err := ww.popEOL(); err != nil {
				return nil, err
			}
			continue

		case lexer.IMPORT:
			ww.popToken()
			packagePath, err := ww.popReference()
			if err != nil {
				return file, err
			}
			decl := Import{
				Path: packagePath.String(),
			}

			if ww.nextType() == lexer.IDENT {
				tok := ww.popToken()
				if tok.Lit != "as" {
					return file, unexpectedToken(tok, lexer.IDENT)
				}

				ident, err := ww.popIdent()
				if err != nil {
					return file, err
				}
				decl.Alias = ident.Value
			}

			file.Imports = append(file.Imports, decl)

		default:
			err := ww.nextStatement(&file.Body)
			if err != nil {
				return file, err
			}
		}
	}
}

// WalkBody walks up until, but not including, a closing }
func (ww *Walker) walkBody(block *BlockStatement) error {
	block.Body = Body{}

	for {
		switch ww.nextType() {
		case lexer.RBRACE:
			// Do not consume, that is the job of the block statement
			return nil

		case lexer.EXPORT:
			ww.popToken()
			block.Export = true
			_, err := ww.endStatement()
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

		default:
			err := ww.nextStatement(&block.Body)
			if err != nil {
				return err
			}
		}
	}
}

func (ww *Walker) nextStatement(body *Body) error {

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
			return errpos.AddContext(err, ref.String())
		}

		stmt, err := ww.walkStatement(ref)
		if err != nil {
			return errpos.AddContext(err, ref.String())
		}
		body.Statements = append(body.Statements, stmt)
		return nil

	default:
		return unexpectedToken(ww.popToken(), lexer.IDENT, lexer.COMMENT)
	}

}

func (ww *Walker) popEOL() error {
	tok := ww.popToken()

	if tok.Type == lexer.COMMENT {
		return ww.popEOL()
	}

	if tok.Type != lexer.EOL && tok.Type != lexer.EOF {
		return unexpectedToken(tok, lexer.EOL)
	}
	return nil
}

func (ww *Walker) popType(tt lexer.TokenType) (lexer.Token, error) {
	tok := ww.popToken()
	if tok.Type != tt {
		return lexer.Token{}, unexpectedToken(tok, tt)
	}
	return tok, nil
}

func (ww *Walker) popValue() (Value, error) {
	token := ww.popToken()
	if token.Type.IsLiteral() {
		return Value{
			token: token,
		}, nil
	}

	return Value{}, tokenErrf(token, "unexpected token %s, expected a literal or value", token)
}

func (ww *Walker) popIdent() (Ident, error) {
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
func (ww *Walker) popReference() (Reference, error) {
	ref := make(Reference, 0)
	for {
		ident, err := ww.popIdent()
		if err != nil {
			return ref, fmt.Errorf("%w after \"%s.\"", err, ref.String())
		}
		ref = append(ref, ident)
		if ww.nextType() != lexer.DOT {
			return ref, nil
		}
		ww.popToken()
	}

}

func (ww *Walker) walkStatement(ref Reference) (Statement, error) {
	start := ref[0].Start

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
		return nil, unexpectedToken(ww.popToken(), lexer.LBRACE, lexer.EOL)
	}
}

func (ww *Walker) endStatement() (*Comment, error) {
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

func (ww *Walker) walkValueAssign(ref Reference) (Assignment, error) {

	assign := Assignment{
		Key: ref,
		SourceNode: SourceNode{
			Start: ref[0].Start,
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

func (ww *Walker) walkBlockStatement(block *BlockStatement) error {

	_, err := ww.popType(lexer.LBRACE)
	if err != nil {
		return err
	}

	comment, err := ww.endStatement()
	if err != nil {
		return err
	}
	if comment != nil {
		block.Comment = comment
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
