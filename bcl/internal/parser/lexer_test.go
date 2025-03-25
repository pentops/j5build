package parser

import (
	"strings"
	"testing"

	"github.com/pentops/bcl.go/bcl/errpos"
)

func tTokIdent(lit string) Token {
	return Token{
		Type: IDENT,
		Lit:  lit,
	}
}

func tTokInt(lit string) Token {
	return Token{
		Type: INT,
		Lit:  lit,
	}
}

func tTokString(lit string) Token {
	return Token{
		Type: STRING,
		Lit:  lit,
	}
}

func tTokRegex(lit string) Token {
	return Token{
		Type: REGEX,
		Lit:  lit,
	}
}

func tTokComment(lit string) Token {
	return Token{
		Type: COMMENT,
		Lit:  lit,
	}
}
func tTokBlockComment(lit string) Token {
	return Token{
		Type: BLOCK_COMMENT,
		Lit:  lit,
	}
}

func tTokDescription(lit string) Token {
	return Token{
		Type: DESCRIPTION,
		Lit:  lit,
	}
}

func tTokDecimal(lit string) Token {
	return Token{
		Type: DECIMAL,
		Lit:  lit,
	}
}

func tTokBool(lit string) Token {
	return Token{
		Type: BOOL,
		Lit:  lit,
	}
}

var (
	tTokAssign = Token{
		Type: ASSIGN,
		Lit:  "=",
	}
	tTokEOF = Token{
		Type: EOF,
	}

	tTokLBrace = Token{
		Type: LBRACE,
		Lit:  "{",
	}

	tTokRBrace = Token{
		Type: RBRACE,
		Lit:  "}",
	}
	tTokDot = Token{
		Type: DOT,
		Lit:  ".",
	}
	tTokEOL = Token{
		Type: EOL,
		Lit:  "\n",
	}
	tTokLBracket = Token{
		Type: LBRACK,
		Lit:  "[",
	}
	tTokRBracket = Token{
		Type: RBRACK,
		Lit:  "]",
	}
	tTokComma = Token{
		Type: COMMA,
		Lit:  ",",
	}
)

type positionAsserter struct {
	lex *Lexer
	t   *testing.T
}

func newPosAsserter(t *testing.T, val string) *positionAsserter {
	return &positionAsserter{
		lex: NewLexer(val),
		t:   t,
	}
}

func (a *positionAsserter) pos(l, c int) {
	a.t.Helper()
	current := a.lex.getPosition()
	if current.Line != l || current.Column != c {
		a.t.Fatalf("expected %d:%d, got %d:%d", l, c, current.Line, current.Column)
	}
}
func (a *positionAsserter) peek(r rune) {
	a.t.Helper()
	got := a.lex.peek()
	if got != r {
		a.t.Fatalf("Peek Val: expected %q, got %q", r, got)
	}
}
func (a *positionAsserter) next(r rune) {
	a.t.Helper()
	a.lex.next()
	if a.lex.ch != r {
		a.t.Fatalf("Next Val: expected %q, got %q", r, a.lex.ch)
	}
}

func TestPositionWalk(t *testing.T) {

	a := newPosAsserter(t, "01\n01")

	// first positions are invalid, as nothing has been read.
	a.pos(0, -1)
	a.peek('0')
	a.pos(0, -1)

	// Begin at L0,C0
	a.next('0')
	a.pos(0, 0)
	a.next('1')
	a.pos(0, 1)
	a.next('\n')
	a.pos(0, 2)
	a.next('0')
	a.pos(1, 0)
	a.next('1')
	a.pos(1, 1)

}

func TestSimple(t *testing.T) {

	for _, tc := range []struct {
		name              string
		input             []string
		expected          []Token
		expectError       *Position
		expectedPositions []Position
	}{{
		name:        "error",
		input:       []string{"\"\n"},
		expectError: &Position{Line: 0, Column: 1},
	}, {
		name:  "assign",
		input: []string{`ab=123`},
		expected: []Token{
			tTokIdent("ab").tStart(1, 1).tEnd(1, 2),
			tTokAssign.tStart(1, 3).tEnd(1, 3),
			tTokInt("123").tStart(1, 4).tEnd(1, 6),
			tTokEOF.tStart(1, 7).tEnd(1, 7),
		},
	}, {
		name: "assign with spaces",
		input: []string{
			`ab = 123`,
			`  cd = 456  `,
			`  `,
		},
		expected: []Token{
			tTokIdent("ab").tStart(1, 1).tEnd(1, 2),
			tTokAssign.tStart(1, 4).tEnd(1, 4),
			tTokInt("123").tStart(1, 6).tEnd(1, 8),
			tTokEOL.tStart(1, 9).tEnd(1, 9),

			tTokIdent("cd").tStart(2, 3).tEnd(2, 4),
			tTokAssign.tStart(2, 6).tEnd(2, 6),
			tTokInt("456").tStart(2, 8).tEnd(2, 10),
			tTokEOL.tStart(2, 13).tEnd(2, 13),

			tTokEOF.tStart(3, 3).tEnd(3, 3),
		},
	}, {
		name: "identifier with dots",
		input: []string{
			`vv.with.dots = 123`,
		},
		expected: []Token{
			tTokIdent("vv"),
			tTokDot,
			tTokIdent("with"),
			tTokDot,
			tTokIdent("dots"),
			tTokAssign,
			tTokInt("123"),
			tTokEOF,
		},
	}, {
		name: "literal types",
		input: []string{
			`vv = 123`,
			`vv = "value"`,
			`vv = 123.456`,
			`vv = true`,
			`vv = false`,
		},
		expected: []Token{
			tTokIdent("vv"), tTokAssign, tTokInt("123"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokString("value"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokDecimal("123.456"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokBool("true"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokBool("false"), tTokEOF,
		},
	}, {
		name: "array",
		input: []string{
			`vv = [1, 2, 3]`,
			`vv = []`,
			`vv = [1, "2", 3.4, true, false]`,
			`vv = ["a", ["b", "c"], "d"]`,
		},
		expected: []Token{
			tTokIdent("vv"), tTokAssign, tTokLBracket, tTokInt("1"), tTokComma, tTokInt("2"), tTokComma, tTokInt("3"), tTokRBracket, tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokLBracket, tTokRBracket, tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokLBracket, tTokInt("1"), tTokComma, tTokString("2"), tTokComma, tTokDecimal("3.4"), tTokComma, tTokBool("true"), tTokComma, tTokBool("false"), tTokRBracket, tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokLBracket, tTokString("a"), tTokComma, tTokLBracket, tTokString("b"), tTokComma, tTokString("c"), tTokRBracket, tTokComma, tTokString("d"), tTokRBracket, tTokEOF,
		},
	}, {
		name: "type declaration",
		input: []string{
			`object Foo {}`,
		},
		expected: []Token{
			tTokIdent("object"),
			tTokIdent("Foo"),
			tTokLBrace,
			tTokRBrace,
			tTokEOF,
		},
	}, {
		name: "string quotes",
		input: []string{
			`vv = "value"`,
		},
		expected: []Token{
			tTokIdent("vv"),
			tTokAssign,
			tTokString("value"),
			tTokEOF,
		},
	}, {
		name: "string escaped quotes",
		input: []string{
			`vv = "value \"with\" quotes"`,
		},
		expected: []Token{
			tTokIdent("vv"),
			tTokAssign,
			tTokString("value \"with\" quotes"),
			tTokEOF,
		},
	}, {
		name: "string with useless escapes",
		input: []string{
			`vv = "value \\ with \\ useless \\ escapes"`,
		},
		expected: []Token{
			tTokIdent("vv"),
			tTokAssign,
			tTokString("value \\ with \\ useless \\ escapes"),
			tTokEOF,
		},
	}, {
		name: "string with invalid escape",
		input: []string{
			`vv = "value \ with invalid escape"`,
		},
		expectError: &Position{Line: 0, Column: 12},
	}, {
		name: "Newline in string is bad",
		input: []string{
			`vv = "value`,
			`with newline"`,
		},
		expectError: &Position{Line: 0, Column: 11},
	}, {
		name: "Escaped is fine",
		input: []string{
			`vv = "value\`,
			`with newline"`,
		},
		// note no EOL token, strings and comments and descriptions include the
		// newline
		expected: []Token{
			tTokIdent("vv"),
			tTokAssign,
			tTokString("value\nwith newline"),
			tTokEOF,
		},
	}, {
		name: "extend identifier",
		input: []string{
			`key123_ü = 123`,
		},
		expected: []Token{
			tTokIdent("key123_ü"),
			tTokAssign,
			tTokInt("123"),
			tTokEOF,
		},
	}, {
		name: "comment line",
		input: []string{
			"vv = 123 // c1",
			"vv = 123",
			"// c2",
			" //c3",
		},
		expected: []Token{
			tTokIdent("vv"), tTokAssign, tTokInt("123"),
			tTokComment(" c1"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokInt("123"), tTokEOL,
			tTokComment(" c2"), tTokEOL,
			tTokComment("c3"), tTokEOF,
		},
	}, {
		name: "regex",
		input: []string{
			`vv = /regex/`,
		},
		expected: []Token{
			tTokIdent("vv"),
			tTokAssign,
			tTokRegex("regex"),
			tTokEOF,
		},
	}, {
		name: "block comment empty",
		input: []string{
			"/**/ vv",
		},
		expected: []Token{
			tTokBlockComment(""),
			tTokIdent("vv"),
			tTokEOF,
		},
	}, {
		name: "block comment",
		input: []string{
			"/* line1",
			"line2 */",
			"vv",
		},
		expected: []Token{
			tTokBlockComment(" line1\nline2 "),
			tTokEOL,
			tTokIdent("vv"),
			tTokEOF,
		},
	}, {
		name: "description",
		input: []string{
			`  | line1 of description`,
			`  |line2 of description`,
			"vv = 123",
		},
		expected: []Token{
			tTokDescription("line1 of description"), tTokEOL,
			tTokDescription("line2 of description"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokInt("123"),
			tTokEOF,
		},
	}, {
		name: "longer description",
		input: []string{
			`  | line1`,
			`  |`,
			`  | line3`,
			`  | line4`,
			`  | `,
			`  | line6`,
		},
		expected: []Token{
			tTokDescription("line1"), tTokEOL,
			tTokDescription(""), tTokEOL,
			tTokDescription("line3"), tTokEOL,
			tTokDescription("line4"), tTokEOL,
			tTokDescription(""), tTokEOL,
			tTokDescription("line6"),
			tTokEOF,
		},
	}, {
		name: "multi description",
		input: []string{
			`  | description 1`,
			``,
			`  | description 2`,
			"vv = 123",
		},
		expected: []Token{
			tTokDescription("description 1"), tTokEOL,
			tTokEOL,
			tTokDescription("description 2"), tTokEOL,
			tTokIdent("vv"), tTokAssign, tTokInt("123"),
			tTokEOF,
		},
	}, {
		name: "unexpected character",
		input: []string{
			`&`,
		},
		expectError: tPos(1, 1),
	}, {
		name: "unexpected eof",
		input: []string{
			`vv = "`,
		},
		expectError: tPos(1, 7),
	}} {

		t.Run(tc.name, func(t *testing.T) {

			sourceFile := strings.Join(tc.input, "\n")
			tokens, err := scanAll(sourceFile)

			if tc.expectError != nil {
				if err == nil {
					t.Fatalf("expected error at %s but got none", tc.expectError)
				}
				posErrs, ok := errpos.AsErrorsWithSource(err)
				if !ok {
					t.Fatalf("expected position error, got %T", err)
				}
				if len(posErrs.Errors) != 1 {
					t.Fatalf("expected 1 error, got %d", len(posErrs.Errors))
				}
				t.Logf("STR %s\n", posErrs.HumanString(0))

				pos := posErrs.Errors[0].Pos
				if pos == nil {
					t.Fatalf("no error position")
				}
				if pos.Start.String() != tc.expectError.String() {
					t.Fatalf("expected error at %d,%d, got %d,%d (%s %s)", tc.expectError.Line, tc.expectError.Column, pos.Start.Line, pos.Start.Column, pos, tc.expectError)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertTokensEqual(t, tokens, tc.expected)

		})
	}
}

func tPos(line, col int) *Position {
	return &Position{
		Line:   line - 1,
		Column: col - 1,
	}
}

func (t Token) tStart(line, col int) Token {
	t.Start.Line = line - 1
	t.Start.Column = col - 1
	return t
}
func (t Token) tEnd(line, col int) Token {
	t.End.Line = line - 1
	t.End.Column = col - 1
	return t
}

func assertTokensEqual(t *testing.T, tokens, expected []Token) {

	for idx, tok := range tokens {
		if len(expected) <= idx {
			t.Errorf("BAD % 3d: %s (extra)", idx, tok)
			continue
		}
		want := expected[idx]
		if tok.Type != expected[idx].Type || tok.Lit != want.Lit {
			t.Errorf("BAD % 3d: %s want %s", idx, tok, want)
			continue
		}
		if want.Start.Line > 0 {
			if tok.Start.Line != want.Start.Line || tok.Start.Column != want.Start.Column {
				t.Errorf("BAD % 3d: %s start position %s, want %s", idx, tok, tok.Start, want.Start)
			}
		}
		if want.End.Line > 0 {
			if tok.End.Line != want.End.Line || tok.End.Column != want.End.Column {
				t.Errorf("BAD % 3d: %s end position %s, want %s", idx, tok, tok.End, want.End)
			}
		}

		t.Logf("OK  % 3d: %s at %s to %s", idx, tok, tok.Start, tok.End)
	}

	if len(expected) > len(tokens) {
		for _, tok := range expected[len(tokens):] {
			t.Errorf("missing %s", tok)
		}
	}
}

func scanAll(input string) ([]Token, error) {
	lexer := NewLexer(input)
	tokens := []Token{}
	for {
		tok, err := lexer.NextToken()
		if err != nil {
			return tokens, errpos.AddSource(err, input)
		}
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens, nil
}

func TestFullExample(t *testing.T) {
	input := `
package pentops.j5lang.example
version = "v1"

// Comment Line
object Foo {
	| Foo is an example object
	| from ... Python I guess?
	| Unsure.

	field id uuid {}

	field name string {
		min_len = 10
	}
}

/* Comment Block

With Lines
*/`

	tokens, err := scanAll(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertTokensEqual(t, tokens, []Token{
		tTokEOL,
		tTokIdent("package"), tTokIdent("pentops"), tTokDot, tTokIdent("j5lang"), tTokDot, tTokIdent("example"), tTokEOL,
		tTokIdent("version"), tTokAssign, tTokString("v1"), tTokEOL,
		tTokEOL,
		tTokComment(" Comment Line"), tTokEOL,
		tTokIdent("object"), tTokIdent("Foo"), tTokLBrace, tTokEOL,
		tTokDescription("Foo is an example object"), tTokEOL,
		tTokDescription("from ... Python I guess?"), tTokEOL,
		tTokDescription("Unsure."), tTokEOL,
		tTokEOL,
		tTokIdent("field"), tTokIdent("id"), tTokIdent("uuid"), tTokLBrace, tTokRBrace, tTokEOL,
		tTokEOL,
		tTokIdent("field"), tTokIdent("name"), tTokIdent("string"), tTokLBrace, tTokEOL,
		tTokIdent("min_len"), tTokAssign, tTokInt("10"), tTokEOL,
		tTokRBrace, tTokEOL,
		tTokRBrace, tTokEOL,
		tTokEOL,
		tTokBlockComment(" Comment Block\n\nWith Lines\n"),
		tTokEOF,
	})

}
