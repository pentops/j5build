package lexer

import (
	"fmt"
	"unicode"

	"github.com/pentops/bcl.go/bcl/errpos"
)

type Lexer struct {
	pos    errpos.Position
	ch     rune
	offset int
	data   []rune
	isEOL  bool
}

func NewLexer(data string) *Lexer {
	return &Lexer{
		pos:  errpos.Position{Line: 1, Column: 0},
		data: []rune(data),
	}
}

const eof = -1

func (l *Lexer) next() {

	if l.isEOL {
		l.pos.Line++
		// column begins at 0, incremented to 1 at the end of this function
		l.pos.Column = 1
		l.isEOL = false
	} else {
		l.pos.Column++
	}

	if l.offset >= len(l.data) {
		l.ch = eof
		return
	}
	r := rune(l.data[l.offset])
	l.offset++

	if r == '\n' {
		// the EOL position is the end of this line, the next character will
		// reset to n+1, 0
		l.isEOL = true
	}

	l.ch = r
}

func (l *Lexer) peek() rune {
	if l.offset >= len(l.data) {
		return eof
	}
	return rune(l.data[l.offset])
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.peek()) {
		l.next()
	}
}

func (l *Lexer) peekPastWhitespace() rune {
	for n := l.offset; n < len(l.data); n++ {
		r := rune(l.data[n])
		if !unicode.IsSpace(r) {
			return r
		}
	}
	return eof
}

func (l *Lexer) tokenOf(ty TokenType) Token {
	return Token{
		Type:  ty,
		Start: l.pos,
		End:   l.pos,
	}
}

func (l *Lexer) AllTokens() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.NextToken()
		if err != nil {
			return nil, fmt.Errorf("scanning file: %w", err)
		}
		if tok.Type == EOF {
			return tokens, nil
		}
		tokens = append(tokens, tok)
	}
}

func (l *Lexer) errf(format string, args ...interface{}) error {
	current := l.pos
	return &errpos.Err{
		Pos: &current,
		Err: fmt.Errorf(format, args...),
	}
}
func (l *Lexer) unexpectedEOF() error {
	return l.errf("unexpected EOF")
}

// NextToken scans the input for the next token. It returns the position of the token,
// the token's type, and the literal value.
func (l *Lexer) NextToken() (Token, error) {
	// keep looping until we return a token
	var err error
	for {
		l.next()
		if l.ch == eof {
			return l.tokenOf(EOF), nil
		}

		if op, ok := operators[l.ch]; ok {
			return l.tokenOf(op), nil
		}

		switch l.ch {
		case '/':
			opener := l.peek()
			commentStart := l.pos
			var lit string
			switch opener {
			case '/':
				lit = l.lexLineComment()
				return Token{
					Type:  COMMENT,
					Start: commentStart,
					End:   l.pos,
					Lit:   lit,
				}, nil

			case '*':
				lit, err = l.lexBlockComment()
				if err != nil {
					return Token{}, err
				}
				return Token{
					Type:  COMMENT,
					Start: commentStart,
					End:   l.pos,
					Lit:   lit,
				}, nil
			default:
				lit, err := l.lexRegex()
				if err != nil {
					return Token{}, err
				}
				return Token{
					Type:  REGEX,
					Start: l.pos,
					End:   l.pos,
					Lit:   lit,
				}, nil

			}
		case '"':
			startPos := l.pos
			lit, err := l.lexString()
			if err != nil {
				return Token{}, err
			}
			return Token{
				Type:  STRING,
				Start: startPos,
				End:   l.pos,
				Lit:   lit,
			}, nil

		case '|':
			startPos := l.pos
			lit := l.lexDescription()
			return Token{
				Type:  DESCRIPTION,
				Start: startPos,
				End:   l.pos,
				Lit:   lit,
			}, nil

		case '\n':
			return l.tokenOf(EOL), nil

		default:
			if unicode.IsSpace(l.ch) {
				continue
			} else if unicode.IsDigit(l.ch) {
				return l.lexNumber()
			} else if unicode.IsLetter(l.ch) {
				startPos := l.pos
				lit := l.lexIdent()
				if keyword, ok := asKeyword(lit); ok {
					return Token{
						Type:  keyword,
						Start: startPos,
						End:   l.pos,
					}, nil
				}
				if lit == "true" || lit == "false" {
					return Token{
						Type:  BOOL,
						Start: startPos,
						End:   l.pos,
						Lit:   lit,
					}, nil
				}

				return Token{
					Type:  IDENT,
					Start: startPos,
					End:   l.pos,
					Lit:   lit,
				}, nil
			} else {
				return Token{}, l.errf("unexpected character: %c", l.ch)
			}
		}
	}
}

// lexInt scans the input until the end of an integer and then returns the
// literal.
func (l *Lexer) lexNumber() (Token, error) {
	tt := Token{
		Type:  INT,
		Start: l.pos,
		End:   l.pos,
		Lit:   string(l.ch),
	}
	var seenDot bool
	for {
		next := l.peek()
		if unicode.IsDigit(next) {
			l.next()
			tt.Lit = tt.Lit + string(l.ch)
		} else if next == '.' {
			if seenDot {
				return tt, l.errf("unexpected second dot in number literal")
			}
			l.next()
			seenDot = true
			tt.Type = DECIMAL
			tt.Lit = tt.Lit + string('.')

		} else {
			// scanned something not in the integer
			tt.End = l.pos
			return tt, nil
		}
	}
}

// lexIdent scans the input until the end of an identifier and then returns the
// literal.
func (l *Lexer) lexIdent() string {
	var lit = string(l.ch)
	for {
		next := l.peek()
		if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
			l.next()
			lit = lit + string(l.ch)
		} else {
			return lit
		}
	}
}

// lexString scans the input until the end of a string and then returns the
// literal.
func (l *Lexer) lexString() (string, error) {
	var lit string
	quote := l.ch
	for {
		l.next()

		if l.ch == eof {
			return "", l.unexpectedEOF()
		}
		if l.ch == quote {
			// at the end of the string
			return lit, nil
		}
		if l.ch == '\n' {
			return "", l.errf("unexpected EOL in string, did you mean to escape it? ('\\n')")
		}

		if l.ch == '\\' {
			if err := l.lexEscape(quote); err != nil {
				return "", err
			}
			// continue, having consumed the escape sequence, the next character
			// is just 'normal'
		}
		lit = lit + string(l.ch)
	}
}

// lexRegex scans the input until the end of a /regex/ which ignores bad
// escapes.
// The only special escape, because there has to be one, is that a // is a /.
// Actual newline characters are invalid, use the \n notation. because it's a
// regex.
func (l *Lexer) lexRegex() (string, error) {
	var lit string
	for {
		l.next()

		if l.ch == eof {
			return "", l.unexpectedEOF()
		}
		if l.ch == '\n' {
			return "", l.errf("unexpected EOL in regex, did you mean to escape it? ('\\n')")
		}

		// a // becomes /
		if l.ch == '/' {
			if l.peek() == '/' {
				l.next()
				lit = lit + "/"
				continue
			}
			return lit, nil
		}
		lit = lit + string(l.ch)
	}
}

// lexEscape scans the input for an escape sequence and returns an error if the
// escape sequence is invalid.
func (l *Lexer) lexEscape(quote rune) error {
	switch l.peek() {
	case '\\', '\n', quote:
		l.next()
		return nil
	}
	err := l.errf("invalid escape, did you mean '\\\\'?")
	return err
}

// lexDescription scans the input lines the next line is not a description
func (l *Lexer) lexDescription() string {
	var lit string
	for {
		line := l.lexDescriptionLine()
		if l.peekPastWhitespace() != '|' {
			lit = lit + line
			return lit
		}
		l.skipWhitespace() // leading whitespace on newline
		l.next()           // consume the |
		lit = lit + line + "\n"
	}
}

func (l *Lexer) lexDescriptionLine() string {
	var lit string
	l.next()
	l.skipWhitespace()
	for {
		next := l.peek()
		if next == eof {
			return lit
		}
		if next == '\n' {
			return lit
		}
		l.next()
		lit = lit + string(l.ch)
	}
}

func (l *Lexer) lexBlockComment() (string, error) {
	l.next() // consume the first *
	commentText := ""
	for {
		l.next()
		if l.ch == '*' && l.peek() == '/' {
			l.next()
			return commentText, nil
		}
		if l.ch == eof {
			return commentText, nil
		}
		commentText = commentText + string(l.ch)
	}
}

func (l *Lexer) lexLineComment() string {
	l.next() // consume the second /
	var lit string
	for {
		next := l.peek()
		if next == eof || next == '\n' {
			return lit
		}
		l.next()
		lit = lit + string(l.ch)
	}
}
