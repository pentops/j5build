package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
)

func ParseFile(input string, failFast bool) (*File, error) {
	l := NewLexer(input)

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

type Walker struct {
	tokens   []Token
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

func Walk(tokens []Token, failFast bool) (*File, error) {
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

	if len(fragments) == 0 {
		// empty, but fine
		return ff, nil
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

		case Comment:
			continue

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

		default:
			return nil, fmt.Errorf("unexpected fragment type %T", s)
		}
	}

	if currentBlock.parent != nil {
		lastFragment := fragments[len(fragments)-1]
		pos := lastFragment.Source().Position()
		ff.Errors = append(ff.Errors, &errpos.Err{
			Pos: &pos,
			Err: errors.New("unclosed block at EOF"),
		})
	}

	if len(ff.Errors) > 0 {
		return ff, HadErrors
	}

	return ff, nil
}

func (w *Walker) currentPos() Position {
	if w.offset == 0 {
		return Position{}
	}
	return w.tokens[w.offset-1].End
}

func (ww *Walker) popToken() Token {
	if ww.offset >= len(ww.tokens) {
		eofToken := ww.tokens[len(ww.tokens)-1]
		if eofToken.Type == EOF {
			return eofToken
		}
		// Indicates a bug but let's go with it
		return Token{
			Type:  EOF,
			Lit:   "",
			Start: eofToken.End,
			End:   eofToken.End,
		}
	}
	tok := ww.tokens[ww.offset]
	ww.offset++
	return tok
}

func (ww *Walker) nextType() TokenType {
	return ww.peekType(0)
}
func (ww *Walker) peekType(offset int) TokenType {
	if ww.offset+offset >= len(ww.tokens) {
		return EOF
	}
	return ww.tokens[ww.offset+offset].Type
}

func (ww *Walker) walkFragments() ([]Fragment, error) {
	fragments := make([]Fragment, 0)
	for {
		if ww.nextType() == EOF {
			break
		}
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
		if ww.nextType() == EOL || ww.nextType() == EOF {
			ww.popToken()
			break
		}
		ww.popToken()
	}
	return nil
}

func (ww *Walker) nextFragment() (Fragment, *unexpectedTokenError) {

	switch ww.nextType() {

	case EOF:
		ww.popToken()
		return nil, nil

	case EOL:
		// Empty line
		ww.popToken()
		return nil, nil

	case RBRACE:
		tok := ww.popToken()
		return CloseBlock{
			Token: tok,
			SourceNode: SourceNode{
				Start: tok.Start,
				End:   tok.End,
			},
		}, nil

	case COMMENT, BLOCK_COMMENT:
		tok := ww.popToken()
		return Comment{
			Token: tok,
			Value: tok.Lit,
			SourceNode: SourceNode{
				Start: tok.Start,
				End:   tok.End,
			},
		}, nil

	case DESCRIPTION:
		stmt, err := ww.popDescription()
		if err != nil {
			return nil, err
		}
		return stmt, nil

	case IDENT, BOOL: // bool looks like an ident.
		stmt, err := ww.walkStatement()
		if err != nil {
			return nil, err
		}
		return stmt, nil

	default:
		return nil, unexpectedToken(ww.popToken(), IDENT, COMMENT, DESCRIPTION, RBRACE, EOL)
	}

}

func (ww *Walker) popType(tt TokenType) (Token, *unexpectedTokenError) {
	tok := ww.popToken()
	if tok.Type != tt {
		return Token{}, unexpectedToken(tok, tt)
	}
	return tok, nil
}

func (ww *Walker) popValue() (Value, *unexpectedTokenError) {
	if ww.nextType() == IDENT {
		ref, err := ww.popReference()
		if err != nil {
			return Value{}, err
		}
		return Value{
			token: Token{
				Type:  STRING,
				Lit:   ref.String(),
				Start: ref.SourceNode.Start,
				End:   ref.SourceNode.End,
			},
			SourceNode: SourceNode{
				Start: ref.SourceNode.Start,
				End:   ref.SourceNode.End,
			},
		}, nil
	}
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

	if ww.nextType() == LBRACK {
		opener := ww.popToken()

		if ww.nextType() == RBRACK {
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

			if ww.nextType() == COMMA {
				ww.popToken()
				continue
			}
			if ww.nextType() == RBRACK {
				ww.popToken()
				break
			}
			return Value{}, unexpectedToken(ww.popToken(), COMMA, RBRACK)
		}
		return Value{
			array: values,
			SourceNode: SourceNode{
				Start: opener.Start,
				End:   ww.currentPos(),
			},
		}, nil
	}

	return Value{}, unexpectedToken(ww.popToken(), AnyLiteral, LBRACK)
}

func (ww *Walker) popIdent() (Ident, *unexpectedTokenError) {
	tok, ok := ww.popToken().AsIdent()
	if !ok {
		return Ident{}, unexpectedToken(tok, IDENT)
	}

	return Ident{
		Token: tok,
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
		if ww.nextType() != DOT {
			return NewReference(ref), nil
		}
		ww.popToken()
	}

}

func (ww *Walker) popDescription() (Description, *unexpectedTokenError) {
	tokens := make([]Token, 0)
	lines := make([]string, 0)

	for {
		tok := ww.popToken()
		tokens = append(tokens, tok)
		lines = append(lines, tok.Lit)

		// one EOL, next is Description,
		//	i.e. the next token is a joined description line.
		if ww.peekType(0) != EOL || ww.peekType(1) != DESCRIPTION {
			break
		}
		ww.popToken() // pop EOL.
		// Next is Description, back to the loop.
	}
	return Description{
		Tokens: tokens,
		Value:  strings.Join(lines, "\n"),
		SourceNode: SourceNode{
			Start: tokens[0].Start,
			End:   tokens[len(tokens)-1].End,
		},
	}, nil

}

func (ww *Walker) popTag() (TagValue, *unexpectedTokenError) {

	var mark = TagMarkNone
	var markToken Token
	switch ww.nextType() {
	case BANG:
		tok := ww.popToken()
		mark = TagMarkBang
		markToken = tok
	case QUESTION:
		tok := ww.popToken()
		mark = TagMarkQuestion
		markToken = tok
	}

	switch ww.nextType() {
	case IDENT, BOOL:

		// Build the name parts
		// <reference> <ident>
		ref, err := ww.popReference()
		if err != nil {
			return TagValue{}, err
		}
		return TagValue{
			Mark:      mark,
			MarkToken: markToken,
			Reference: &ref,
			SourceNode: SourceNode{
				Start: ref.SourceNode.Start,
				End:   ref.SourceNode.End,
			},
		}, nil

	case STRING:
		refStr, err := ww.popValue()
		if err != nil {
			return TagValue{}, err
		}
		return TagValue{
			Mark:      mark,
			MarkToken: markToken,
			Value:     &refStr,
			SourceNode: SourceNode{
				Start: refStr.SourceNode.Start,
				End:   refStr.SourceNode.End,
			},
		}, nil

	default:

		return TagValue{}, unexpectedToken(ww.popToken(), IDENT, BOOL, STRING)
	}

}

func (ww *Walker) walkStatement() (Fragment, *unexpectedTokenError) {

	// Read all dot separated idents continuing from the first token
	// a.b.c.d
	ref, err := ww.popReference()
	if err != nil {
		err.context = fmt.Sprintf("after \"%s\"", ref.String())
		return nil, err
	}

	start := ref.SourceNode.Start

	// Assignments can only take one LHS argument
	if ww.nextType() == ASSIGN {
		// <reference> = ...
		return ww.walkValueAssign(ref)
	}

	if ww.nextType() == PLUS {
		// +=
		ww.popToken() // Pop +
		if ww.nextType() != ASSIGN {
			return nil, unexpectedToken(ww.popToken(), ASSIGN)
		}

		assign, err := ww.walkValueAssign(ref)
		if err != nil {
			return nil, err
		}
		assign.Append = true
		return assign, nil

	}

	nameParts := []TagValue{}

	hdr := BlockHeader{
		Type: ref,
		Tags: nameParts,
		SourceNode: SourceNode{
			Start: start,
		},
	}

	for ww.nextType().CanStartTag() {
		tag, err := ww.popTag()
		if err != nil {
			return nil, err
		}
		hdr.Tags = append(hdr.Tags, tag)
	}

	for ww.nextType() == COLON {
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

	case LBRACE:
		hdr.Open = true
		ww.popToken()
		hdr.End = ww.currentPos()
		// <reference> { ...
		// This is a block statement
		// <reference> { <body> }

		comment, err := ww.endStatement()
		if err != nil {
			return hdr, err
		}
		if comment != nil {
			hdr.Comment = comment
		}
		return hdr, nil

	case DESCRIPTION:
		// <reference> | description
		// This is a block statement without a body.
		tok := ww.popToken()
		desc := Description{
			Tokens: []Token{tok},
			Value:  tok.Lit,
			SourceNode: SourceNode{
				Start: tok.Start,
				End:   tok.End,
			},
		}
		hdr.Description = &desc
		hdr.End = ww.currentPos()
		return hdr, nil

	case COMMENT:
		comment, err := ww.endStatement()
		if err != nil {
			return hdr, err
		}
		if comment != nil {
			hdr.Comment = comment
		}
		return hdr, nil

	case EOL, EOF:
		hdr.End = ww.currentPos()
		return hdr, nil

	default:
		return nil, unexpectedToken(ww.popToken(), LBRACE, EOL, DESCRIPTION, IDENT)
	}
}

func (ww *Walker) endStatement() (*Comment, *unexpectedTokenError) {
	tok := ww.popToken()

	var returnComment *Comment

	if tok.Type == COMMENT {
		returnComment = &Comment{
			Value: tok.Lit,
			SourceNode: SourceNode{
				Start: tok.Start,
				End:   tok.End,
			},
		}
		tok = ww.popToken()
	}

	if tok.Type == EOL || tok.Type == EOF {
		return returnComment, nil
	}

	return returnComment, unexpectedToken(tok, COMMENT, EOL)
}

func (ww *Walker) walkValueAssign(ref Reference) (Assignment, *unexpectedTokenError) {

	assign := Assignment{
		Key: ref,
		SourceNode: SourceNode{
			Start: ref.SourceNode.Start,
		},
	}

	_, err := ww.popType(ASSIGN)
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
