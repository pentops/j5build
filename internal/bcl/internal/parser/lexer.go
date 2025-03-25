package parser

import (
	"fmt"
	"unicode"

	"github.com/pentops/bcl.go/bcl/errpos"
)

type Lexer struct {
	line   int // 0 based
	column int // 0 based

	ch     rune
	offset int
	data   []rune
	isEOL  bool

	Errors errpos.Errors
}

func NewLexer(data string) *Lexer {
	return &Lexer{
		data:   []rune(data),
		line:   0,
		column: -1,
	}
}

const lexerEofChr = -1

func (l *Lexer) next() {

	if l.isEOL {
		l.line++
		l.column = 0
		l.isEOL = false
	} else {
		l.column++
	}

	if l.offset >= len(l.data) {
		l.ch = lexerEofChr
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

func (l *Lexer) getPosition() Position {
	return Position{
		Line:   l.line,
		Column: l.column,
	}
}

func (l *Lexer) peek() rune {
	if l.offset >= len(l.data) {
		return lexerEofChr
	}
	return rune(l.data[l.offset])
}

func (l *Lexer) skipWhitespace() {
	for {
		v := l.peek()
		if !unicode.IsSpace(v) {
			break
		}
		if v == '\n' {
			break
		}
		l.next()
	}
}

func (l *Lexer) tokenOf(ty TokenType) Token {
	lit := string(l.ch)
	if l.ch == lexerEofChr {
		lit = ""
	}
	return Token{
		Lit:   lit,
		Type:  ty,
		Start: l.getPosition(),
		End:   l.getPosition(),
	}
}

func (l *Lexer) AllTokens(failFast bool) ([]Token, bool, error) {
	var tokens []Token
	for {
		tok, err := l.NextToken()
		if err != nil {
			posErr, ok := errpos.AsError(err)
			if !ok {
				return nil, false, err
			}

			l.Errors = append(l.Errors, posErr)
			if failFast {
				return nil, false, nil
			}

		}
		if tok.Type == EOF {
			break
		}
		tokens = append(tokens, tok)
	}
	if len(l.Errors) > 0 {
		return nil, false, nil
	}
	return tokens, true, nil
}

func (l *Lexer) errf(format string, args ...interface{}) error {
	current := l.getPosition()
	return &errpos.Err{
		Pos: &errpos.Position{
			Start: current,
			End:   current,
		},
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
	for {
		l.next()
		if l.ch == lexerEofChr {
			return l.tokenOf(EOF), nil
		}

		if op, ok := operators[l.ch]; ok {
			return l.tokenOf(op), nil
		}

		startPos := l.getPosition()
		switch l.ch {
		case '/':
			opener := l.peek()
			var lit string
			switch opener {
			case '/':
				lit = l.lexLineComment()
				return Token{
					Type:  COMMENT,
					Start: startPos,
					End:   l.getPosition(),
					Lit:   lit,
				}, nil

			case '*':
				lit = l.lexBlockComment()
				return Token{
					Type:  BLOCK_COMMENT,
					Start: startPos,
					End:   l.getPosition(),
					Lit:   lit,
				}, nil
			default:
				lit, err := l.lexRegex()
				if err != nil {
					return Token{}, err
				}
				return Token{
					Type:  REGEX,
					Start: startPos,
					End:   l.getPosition(),
					Lit:   lit,
				}, nil

			}
		case '"':
			lit, err := l.lexString()
			if err != nil {
				return Token{}, err
			}
			return Token{
				Type:  STRING,
				Start: startPos,
				End:   l.getPosition(),
				Lit:   lit,
			}, nil

		case '|':
			lit := l.lexDescriptionLine()
			return Token{
				Type:  DESCRIPTION,
				Start: startPos,
				End:   l.getPosition(),
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
				lit := l.lexIdent()
				if keyword, ok := asKeyword(lit); ok {
					return Token{
						Type:  keyword,
						Start: startPos,
						End:   l.getPosition(),
					}, nil
				}
				if lit == "true" || lit == "false" {
					return Token{
						Type:  BOOL,
						Start: startPos,
						End:   l.getPosition(),
						Lit:   lit,
					}, nil
				}

				return Token{
					Type:  IDENT,
					Start: startPos,
					End:   l.getPosition(),
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
		Start: l.getPosition(),
		End:   l.getPosition(),
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
			tt.End = l.getPosition()
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

		if l.ch == lexerEofChr {
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

		if l.ch == lexerEofChr {
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

func (l *Lexer) lexDescriptionLine() string {
	var lit string
	l.skipWhitespace()
	for {
		next := l.peek()
		if next == lexerEofChr {
			return lit
		}
		if next == '\n' {
			return lit
		}
		l.next()
		lit = lit + string(l.ch)
	}
}

func (l *Lexer) lexBlockComment() string {
	l.next() // consume the first *
	commentText := ""
	for {
		l.next()
		if l.ch == '*' && l.peek() == '/' {
			l.next()
			return commentText
		}
		if l.ch == lexerEofChr {
			return commentText
		}
		commentText = commentText + string(l.ch)
	}
}

func (l *Lexer) lexLineComment() string {
	l.next() // consume the second /
	var lit string
	for {
		next := l.peek()
		if next == lexerEofChr || next == '\n' {
			return lit
		}
		l.next()
		lit = lit + string(l.ch)
	}
}
