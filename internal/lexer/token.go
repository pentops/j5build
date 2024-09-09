// package token is adapted from the Go standard library's token package.
package lexer

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/pentops/bcl.go/bcl/errpos"
)

type TokenType int

const (
	INVALID TokenType = iota
	EOF
	EOL

	literal_beg
	IDENT
	STRING  // "abc"
	REGEX   // /abc/
	INT     // 123
	DECIMAL // 123.45
	BOOL    // true or false
	COMMENT
	DESCRIPTION
	literal_end

	operator_beg
	ASSIGN // =
	LBRACE // {
	RBRACE // }
	LBRACK // [
	RBRACK // ]
	DOT    // .
	COMMA  // ,
	COLON  // :
	BANG   // !
	operator_end

	keyword_beg
	//PACKAGE // package
	//IMPORT  // import
	//EXPORT  // export
	//INCLUDE // include
	keyword_end

	AnyLiteral
)

var tokens = [...]string{
	INVALID: "INVALID",
	EOF:     "EOF",
	EOL:     "EOL",

	// Literal
	literal_beg: "",
	IDENT:       "IDENT",
	STRING:      "STRING",
	REGEX:       "REGEX",
	INT:         "INT",
	DECIMAL:     "DECIMAL",
	BOOL:        "BOOL",
	COMMENT:     "COMMENT",
	DESCRIPTION: "DESCRIPTION",
	literal_end: "",

	// Operators
	operator_beg: "",
	ASSIGN:       "=",
	LBRACE:       "{",
	RBRACE:       "}",
	LBRACK:       "[",
	RBRACK:       "]",
	DOT:          ".",
	COMMA:        ",",
	COLON:        ":",
	BANG:         "!",
	operator_end: "",

	// Keywords
	keyword_beg: "",
	//PACKAGE:     "package",
	//IMPORT:      "import",
	//EXPORT:      "export",
	//INCLUDE:     "include",
	keyword_end: "",

	AnyLiteral: "<Literal>",
}

type Position = errpos.Point

type Token struct {
	Type       TokenType
	Lit        string
	Start, End Position
}

func (tok Token) String() string {
	if tok.Type.IsLiteral() {
		short := tok.Lit
		if len(short) > 8 {
			short = short[:5] + "..."
		}
		return fmt.Sprintf("%s(%s)", tok.Type.String(), short)
	}

	if tok.Type.IsKeyword() {
		return fmt.Sprintf("keyword(%s)", tok.Type.String())
	}

	if tok.Type.IsOperator() {
		return fmt.Sprintf("operator('%s')", tok.Type.String())
	}

	return tok.Type.String()
}

func (tok TokenType) String() string {
	s := ""
	if 0 <= tok && tok < TokenType(len(tokens)) {
		s = tokens[tok]

	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

var keywords map[string]TokenType

var operators map[rune]TokenType

func init() {
	keywords = make(map[string]TokenType, keyword_end-(keyword_beg+1))
	for i := keyword_beg + 1; i < keyword_end; i++ {
		keywords[tokens[i]] = i
	}

	operators = map[rune]TokenType{}
	for i := operator_beg + 1; i < operator_end; i++ {
		operators[rune(tokens[i][0])] = i
	}
}

// Lookup maps an identifier to its keyword token or [IDENT] (if not a keyword).
func asKeyword(ident string) (TokenType, bool) {
	if tok, is_keyword := keywords[ident]; is_keyword {
		return tok, true
	}
	return IDENT, false
}

// IsKeyword returns true for tokens corresponding to keywords;
// it returns false otherwise.
func (tok TokenType) IsKeyword() bool { return keyword_beg < tok && tok < keyword_end }

// IsLiteral returns true for tokens corresponding to identifiers, basic type literals;
// it returns false otherwise.
func (tok TokenType) IsLiteral() bool { return literal_beg < tok && tok < literal_end }

// IsOperator returns true for tokens corresponding to operators;
// it returns false otherwise.
func (tok TokenType) IsOperator() bool { return operator_beg < tok && tok < operator_end }

func (tok TokenType) IsTag() bool {
	return tok == IDENT || tok == STRING || tok == REGEX || tok == BANG
}

// IsKeyword reports whether name is a Go keyword, such as "func" or "return".
func IsKeyword(name string) bool {
	// TODO: opt: use a perfect hash function instead of a global map.
	_, ok := keywords[name]
	return ok
}

// IsIdentifier reports whether name is a Go identifier, that is, a non-empty
// string made up of letters, digits, and underscores, where the first character
// is not a digit. Keywords are not identifiers.
func IsIdentifier(name string) bool {
	if name == "" || IsKeyword(name) {
		return false
	}
	for i, c := range name {
		if !unicode.IsLetter(c) && c != '_' && (i == 0 || !unicode.IsDigit(c)) {
			return false
		}
	}
	return true
}
