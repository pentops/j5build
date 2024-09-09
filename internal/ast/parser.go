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
	fragments, err := ww.walkFragments()
	if err != nil {
		return &File{
			Errors: ww.errors,
		}, err
	}
	if len(ww.errors) > 0 {
		return &File{
			Errors: ww.errors,
		}, HadErrors
	}

	return fragmentsToFile(fragments)
}

func fragmentsToFile(fragments []Fragment) (*File, error) {

	type walkingBlock struct {
		parent *walkingBlock
		body   *Body
	}

	ff := &File{}

	currentBlock := &walkingBlock{
		parent: nil,
		body:   &ff.Body,
	}

	for _, stmt := range fragments {
		switch s := stmt.(type) {
		case BlockHeader:
			block := &Block{
				BlockHeader: s,
			}
			currentBlock.body.Statements = append(currentBlock.body.Statements, block)

			if !s.Open {
				continue
			}

			newBlock := &walkingBlock{
				parent: currentBlock,
				body:   &block.Body,
			}
			currentBlock = newBlock

		case Assignment:
			currentBlock.body.Statements = append(currentBlock.body.Statements, &s)

		case Description:
			currentBlock.body.Statements = append(currentBlock.body.Statements, &s)

		case CloseBlock:
			if currentBlock.parent == nil {
				pos := s.SourceNode.Position()
				ff.Errors = append(ff.Errors, &errpos.Err{
					Pos: &pos,
					Err: errors.New("unexpected close block"),
				})
				continue
			}
			currentBlock = currentBlock.parent

		case EOF:
			return ff, nil

		default:
			return nil, fmt.Errorf("unexpected fragment type %T", s)
		}
	}

	return ff, nil
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

func (ww *Walker) walkFragments() ([]Fragment, error) {
	fragments := make([]Fragment, 0)
	for {
		fragment, err := ww.nextFragment()
		if err != nil {
			if err := ww.recoverError(err); err != nil {
				return fragments, err
			}
		}
		if fragment == nil {
			continue
		}
		fragments = append(fragments, fragment)
		if fragment.Kind() == EOFFragment {
			break
		}
	}

	return fragments, nil
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

func (ww *Walker) nextFragment() (Fragment, *unexpectedTokenError) {

	switch ww.nextType() {

	case lexer.EOF:
		return EOF{}, nil

	case lexer.EOL:
		// Empty line
		ww.popToken()
		return nil, nil

	case lexer.RBRACE:
		ww.popToken()
		return CloseBlock{}, nil

	case lexer.COMMENT:
		// skip comments for now
		ww.popToken()
		return nil, nil

	case lexer.DESCRIPTION:
		desc := ww.popToken()
		return Description{
			Value: Value{
				token: desc,
				SourceNode: SourceNode{
					Start: desc.Start,
					End:   desc.End,
				},
			},
			SourceNode: SourceNode{
				Start: desc.Start,
				End:   desc.End,
			},
		}, nil
		// Description tokens consume the EOL

	case lexer.IDENT:

		// Read all dot separated idents continuing from the first token
		// a.b.c.d
		ref, err := ww.popReference()
		if err != nil {
			err.context = fmt.Sprintf("after \"%s\"", ref.String())
			return nil, err
		}

		stmt, err := ww.walkStatement(ref)
		if err != nil {
			err.context = fmt.Sprintf("after \"%s\"", ref.String())
			return nil, err
		}
		return stmt, nil

	default:
		return nil, unexpectedToken(ww.popToken(), lexer.IDENT, lexer.COMMENT, lexer.DESCRIPTION, lexer.RBRACE, lexer.EOL)
	}

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

func (ww *Walker) popTag() (TagValue, *unexpectedTokenError) {

	bang := false
	if ww.nextType() == lexer.BANG {
		ww.popToken()
		bang = true
	}

	switch ww.nextType() {
	case lexer.IDENT:

		// Build the name parts
		// <reference> <ident>
		ref, err := ww.popReference()
		if err != nil {
			return TagValue{}, err
		}
		return TagValue{
			Bang:      bang,
			Reference: &ref,
			SourceNode: SourceNode{
				Start: ref.SourceNode.Start,
				End:   ref.SourceNode.End,
			},
		}, nil

	case lexer.STRING:
		refStr, err := ww.popValue()
		if err != nil {
			return TagValue{}, err
		}
		return TagValue{
			Bang:  bang,
			Value: &refStr,
			SourceNode: SourceNode{
				Start: refStr.SourceNode.Start,
				End:   refStr.SourceNode.End,
			},
		}, nil

	default:
		return TagValue{}, unexpectedToken(ww.popToken(), lexer.IDENT, lexer.STRING)
	}

}

func (ww *Walker) walkStatement(ref Reference) (Fragment, *unexpectedTokenError) {
	start := ref.SourceNode.Start

	// Assignments can only take one LHS argument
	if ww.nextType() == lexer.ASSIGN {
		// <reference> = ...
		return ww.walkValueAssign(ref)
	}

	nameParts := []TagValue{}

	hdr := BlockHeader{
		Type: ref,
		Tags: nameParts,
		SourceNode: SourceNode{
			Start: start,
		},
	}

	for ww.nextType().IsTag() {
		tag, err := ww.popTag()
		if err != nil {
			return nil, err
		}
		hdr.Tags = append(hdr.Tags, tag)
	}

	for ww.nextType() == lexer.COLON {
		ww.popToken()
		// <reference>:...
		// Expect a qualifier reference, e.g an object:a.b.C
		qualifier, err := ww.popTag()
		if err != nil {
			return nil, err
		}
		hdr.Qualifiers = append(hdr.Qualifiers, qualifier)
	}

	switch ww.nextType() {

	case lexer.LBRACE:
		hdr.Open = true
		ww.popToken()
		// <reference> { ...
		// This is a block statement
		// <reference> { <body> }
		return hdr, nil

	case lexer.DESCRIPTION:
		// <reference> | description
		// This is a block statement without a body.
		tok := ww.popToken()
		hdr.Description = &tok.Lit
		hdr.End = ww.currentPos()
		return hdr, nil

	case lexer.EOL, lexer.EOF:
		hdr.End = ww.currentPos()
		return hdr, nil

	default:
		return nil, unexpectedToken(ww.popToken(), lexer.LBRACE, lexer.EOL, lexer.DESCRIPTION, lexer.IDENT)
	}
}

func (ww *Walker) endStatement() (*Comment, *unexpectedTokenError) {
	tok := ww.popToken()

	var returnComment *Comment

	if tok.Type == lexer.COMMENT {
		returnComment = &Comment{
			Value: tok.Lit,
			SourceNode: SourceNode{
				Start: tok.Start,
				End:   tok.End,
			},
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

/*
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
}*/
