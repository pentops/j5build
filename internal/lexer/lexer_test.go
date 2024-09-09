package lexer

import (
	"strings"
	"testing"

	"github.com/pentops/bcl.go/bcl/errpos"
)

func tIdent(lit string) Token {
	return Token{
		Type: IDENT,
		Lit:  lit,
	}
}

func tInt(lit string) Token {
	return Token{
		Type: INT,
		Lit:  lit,
	}
}

func tString(lit string) Token {
	return Token{
		Type: STRING,
		Lit:  lit,
	}
}

func tRegex(lit string) Token {
	return Token{
		Type: REGEX,
		Lit:  lit,
	}
}

func tComment(lit string) Token {
	return Token{
		Type: COMMENT,
		Lit:  lit,
	}
}

func tDescription(lit string) Token {
	return Token{
		Type: DESCRIPTION,
		Lit:  lit,
	}
}

func tDecimal(lit string) Token {
	return Token{
		Type: DECIMAL,
		Lit:  lit,
	}
}

func tBool(lit string) Token {
	return Token{
		Type: BOOL,
		Lit:  lit,
	}
}

var (
	tAssign = Token{
		Type: ASSIGN,
	}
	tEOF = Token{
		Type: EOF,
	}

	tLBrace = Token{
		Type: LBRACE,
	}

	tRBrace = Token{
		Type: RBRACE,
	}
	tDot = Token{
		Type: DOT,
	}
	tEOL = Token{
		Type: EOL,
	}
	tLBracket = Token{
		Type: LBRACK,
	}
	tRBracket = Token{
		Type: RBRACK,
	}
	tComma = Token{
		Type: COMMA,
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
			tIdent("ab").tStart(1, 1).tEnd(1, 2),
			tAssign.tStart(1, 3).tEnd(1, 3),
			tInt("123").tStart(1, 4).tEnd(1, 6),
			tEOF.tStart(1, 7).tEnd(1, 7),
		},
	}, {
		name: "assign with spaces",
		input: []string{
			`ab = 123`,
			`  cd = 456  `,
			`  `,
		},
		expected: []Token{
			tIdent("ab").tStart(1, 1).tEnd(1, 2),
			tAssign.tStart(1, 4).tEnd(1, 4),
			tInt("123").tStart(1, 6).tEnd(1, 8),
			tEOL.tStart(1, 9).tEnd(1, 9),

			tIdent("cd").tStart(2, 3).tEnd(2, 4),
			tAssign.tStart(2, 6).tEnd(2, 6),
			tInt("456").tStart(2, 8).tEnd(2, 10),
			tEOL.tStart(2, 13).tEnd(2, 13),

			tEOF.tStart(3, 3).tEnd(3, 3),
		},
	}, {
		name: "identifier with dots",
		input: []string{
			`vv.with.dots = 123`,
		},
		expected: []Token{
			tIdent("vv"),
			tDot,
			tIdent("with"),
			tDot,
			tIdent("dots"),
			tAssign,
			tInt("123"),
			tEOF,
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
			tIdent("vv"), tAssign, tInt("123"), tEOL,
			tIdent("vv"), tAssign, tString("value"), tEOL,
			tIdent("vv"), tAssign, tDecimal("123.456"), tEOL,
			tIdent("vv"), tAssign, tBool("true"), tEOL,
			tIdent("vv"), tAssign, tBool("false"), tEOF,
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
			tIdent("vv"), tAssign, tLBracket, tInt("1"), tComma, tInt("2"), tComma, tInt("3"), tRBracket, tEOL,
			tIdent("vv"), tAssign, tLBracket, tRBracket, tEOL,
			tIdent("vv"), tAssign, tLBracket, tInt("1"), tComma, tString("2"), tComma, tDecimal("3.4"), tComma, tBool("true"), tComma, tBool("false"), tRBracket, tEOL,
			tIdent("vv"), tAssign, tLBracket, tString("a"), tComma, tLBracket, tString("b"), tComma, tString("c"), tRBracket, tComma, tString("d"), tRBracket, tEOF,
		},
	}, {
		name: "type declaration",
		input: []string{
			`object Foo {}`,
		},
		expected: []Token{
			tIdent("object"),
			tIdent("Foo"),
			tLBrace,
			tRBrace,
			tEOF,
		},
	}, {
		name: "string quotes",
		input: []string{
			`vv = "value"`,
		},
		expected: []Token{
			tIdent("vv"),
			tAssign,
			tString("value"),
			tEOF,
		},
	}, {
		name: "string escaped quotes",
		input: []string{
			`vv = "value \"with\" quotes"`,
		},
		expected: []Token{
			tIdent("vv"),
			tAssign,
			tString("value \"with\" quotes"),
			tEOF,
		},
	}, {
		name: "string with useless escapes",
		input: []string{
			`vv = "value \\ with \\ useless \\ escapes"`,
		},
		expected: []Token{
			tIdent("vv"),
			tAssign,
			tString("value \\ with \\ useless \\ escapes"),
			tEOF,
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
			tIdent("vv"),
			tAssign,
			tString("value\nwith newline"),
			tEOF,
		},
	}, {
		name: "extend identifier",
		input: []string{
			`key123_ü = 123`,
		},
		expected: []Token{
			tIdent("key123_ü"),
			tAssign,
			tInt("123"),
			tEOF,
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
			tIdent("vv"), tAssign, tInt("123"),
			tComment(" c1"), tEOL,
			tIdent("vv"), tAssign, tInt("123"), tEOL,
			tComment(" c2"), tEOL,
			tComment("c3"), tEOF,
		},
	}, {
		name: "regex",
		input: []string{
			`vv = /regex/`,
		},
		expected: []Token{
			tIdent("vv"),
			tAssign,
			tRegex("regex"),
			tEOF,
		},
	}, {
		name: "block comment empty",
		input: []string{
			"/**/ vv",
		},
		expected: []Token{
			tComment(""),
			tIdent("vv"),
			tEOF,
		},
	}, {
		name: "block comment",
		input: []string{
			"/* line1",
			"line2 */",
			"vv",
		},
		expected: []Token{
			tComment(" line1\nline2 "),
			tEOL,
			tIdent("vv"),
			tEOF,
		},
	}, {
		name: "description",
		input: []string{
			`  | line1 of description`,
			`  | line2 of description`,
			"vv = 123",
		},
		expected: []Token{
			tDescription("line1 of description\nline2 of description"),
			tEOL,
			tIdent("vv"), tAssign, tInt("123"),
			tEOF,
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
			tDescription("line1\n\nline3\nline4\n\nline6"),
			tEOF,
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
			t.Logf("Full  Got: %q", tok.Lit)
			t.Logf("Full Want: %q", want.Lit)
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
		tEOL,
		tIdent("package"), tIdent("pentops"), tDot, tIdent("j5lang"), tDot, tIdent("example"), tEOL,
		tIdent("version"), tAssign, tString("v1"), tEOL,
		tEOL,
		tComment(" Comment Line"), tEOL,
		tIdent("object"), tIdent("Foo"), tLBrace, tEOL,
		tDescription("Foo is an example object\nfrom ... Python I guess?\nUnsure."), tEOL,
		tEOL,
		tIdent("field"), tIdent("id"), tIdent("uuid"), tLBrace, tRBrace, tEOL,
		tEOL,
		tIdent("field"), tIdent("name"), tIdent("string"), tLBrace, tEOL,
		tIdent("min_len"), tAssign, tInt("10"), tEOL,
		tRBrace, tEOL,
		tRBrace, tEOL,
		tEOL,
		tComment(" Comment Block\n\nWith Lines\n"),
		tEOF,
	})

}
