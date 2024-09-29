package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

func tParseFile(t testing.TB, input string) *File {
	t.Helper()
	file, err := ParseFile(input, false)
	if err != nil {
		printErr(t, err)
		t.Fatal("FATAL: unexpected error")
	}
	return file
}

func printErr(t testing.TB, err error) {
	t.Helper()
	posErrs, ok := errpos.AsErrorsWithSource(err)
	if !ok {
		t.Fatalf("FATAL: expected position error, got %T %s", err, err.Error())
	}
	t.Log(posErrs.HumanString(2))
}

func TestErrors(t *testing.T) {
	t.Run("unexpected character", func(t *testing.T) {
		assertErr(t, `!`, errSet(errPos(1, 1)))
		assertErr(t, "package foo\n!", errSet(errPos(2, 1)))

	})

	t.Run("context", func(t *testing.T) {
		assertErr(t, `package pentops. `,
			errSet(
				errContains("IDENT"),
				errContains("EOF"),
				errPos(1, 16),
			),
		)
	})

	t.Run("unexpected close", func(t *testing.T) {
		assertErr(t, `block Foo }`, errSet(errPos(1, 11)))
	})
	t.Run("unexpected standalone close", func(t *testing.T) {
		assertErr(t, `}`, errSet(errPos(1, 1)))
	})

	t.Run("multiple errors", func(t *testing.T) {
		assertErr(t, strings.Join([]string{
			"block }",
			"good Foo",
			"bad   }",
			"good",
			"",
		}, "\n"), errSet(
			errPos(1, 7),
		), errSet(
			errPos(3, 7),
		))
	})
}

func errSet(assertions ...errorAssertion) []errorAssertion {
	return assertions
}

type errorAssertion func(*testing.T, *errpos.Err)

func assertErr(t *testing.T, input string, assertions ...[]errorAssertion) {
	t.Helper()

	_, err := ParseFile(input, false)
	if err == nil {
		t.Fatalf("FATAL: expected errors, got none")
	}

	errors, ok := errpos.AsErrorsWithSource(err)
	if !ok {
		t.Fatalf("FATAL: expected error to have source, got %T", err)
	}

	printErr(t, errors)

	for idx, assertionSet := range assertions {
		if idx >= len(errors.Errors) {
			t.Errorf("ERROR: Missing error %d", idx)
			continue
		}
		got := errors.Errors[idx]
		for _, assertion := range assertionSet {
			assertion(t, got)
		}
	}

}

func errContains(strs ...string) errorAssertion {
	return func(t *testing.T, err *errpos.Err) {
		for _, str := range strs {
			if !strings.Contains(err.Error(), str) {
				t.Errorf("ERROR: error did not contain %q: %q", str, err.Error())
			}
		}
	}
}

func errPos(line, col int) errorAssertion {
	line--
	col--
	return func(t *testing.T, err *errpos.Err) {

		if err.Pos == nil {
			t.Fatalf("ERROR: expected position %d:%d, got none: %#v", line, col, err)
		}
		position := *err.Pos

		if position.Start.Line != line {
			t.Errorf("ERROR: expected line %d, got %d", line, position.Start.Line)
		}

		if col > -1 {
			if position.Start.Column != col {
				t.Errorf("ERROR: expected column %d, got %d", col, position.Start.Column)
			}
		}
	}
}

func TestBasicAssign(t *testing.T) {
	input := `
package pentops.j5lang.example
version = "v1"
number = 123
bool = true
float = 1.23
`

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tBlock(tBlockType("package"), tBlockTags("pentops.j5lang.example")),
		tAssign("version", tString("v1")),
		tAssign("number", tDecimal("123")),
		tAssign("bool", tTrue),
		tAssign("float", tDecimal("1.23")),
	)
}

func TestArrayAssign(t *testing.T) {
	input := `
v1 = [1, 2, 3]
v2 = ["a", "b", "c"]
v3 = [true, false]
v4 = [1, true, "a"]
v5 = [1, [2, 3], [4, 5]]
v6 = []
`

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tAssign("v1", tArray(tDecimal("1"), tDecimal("2"), tDecimal("3"))),
		tAssign("v2", tArray(tString("a"), tString("b"), tString("c"))),
		tAssign("v3", tArray(tTrue, tFalse)),
		tAssign("v4", tArray(tDecimal("1"), tTrue, tString("a"))),
		tAssign("v5", tArray(
			tDecimal("1"),
			tArray(tDecimal("2"), tDecimal("3")),
			tArray(tDecimal("4"), tDecimal("5")),
		)),
		tAssign("v6", tArray()),
	)
}

func TestArrayAppend(t *testing.T) {
	input := `
v1 += 1
v1 += 2
`

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tAssignAppend("v1", tDecimal("1")),
		tAssignAppend("v1", tDecimal("2")),
	)
}

func TestBlockQualifier(t *testing.T) {
	input := `block Foo:type`

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tBlock(
			tBlockType("block"), tBlockTags("Foo"),
			tBlockQualifier("type"),
		),
	)
}

func TestDirectives(t *testing.T) {
	input := strings.Join([]string{
		`import base.baz : baz`,
		`import base.bar`,
		`block Foo {`,
		`  export`,
		`  include bar.a`,
		`  include baz.b`,
		`  k = v`,
		`}`,
	}, "\n")

	file := tParseFile(t, input)

	logBody(t, "file", "", file.Body)

	assertStatements(t, file.Body.Statements,
		tBlock(tBlockType("import"), tBlockTags("base.baz"), tBlockQualifier("baz")),
		tBlock(tBlockType("import"), tBlockTags("base.bar")),
		tBlock(
			tBlockType("block"), tBlockTags("Foo"),
			tBlockBody(
				tBlock(tBlockType("export")),
				tBlock(tBlockType("include"), tBlockTags("bar.a")),
				tBlock(tBlockType("include"), tBlockTags("baz.b")),
				tAssign("k", tString("v")),
			),
		),
	)
}

func TestMultilineDescription(t *testing.T) {
	input := strings.Join([]string{
		`block Foo {`,
		`  | This is a multiline`,
		`  |description`,
		`  |   `,
		`  | With an empty line`,
		``,
		`  | Second Description`,
		`}`,
	}, "\n")

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tBlock(
			tBlockType("block"),
			tBlockTags("Foo"),
			tBlockBody(
				tDescription("This is a multiline\ndescription\n\nWith an empty line"),
				tDescription("Second Description"),
			),
		),
	)
}

func logBody(t *testing.T, name string, indent string, body Body) {
	t.Helper()
	t.Logf(indent+"<open> %s\n", name)
	for idx, stmt := range body.Statements {
		t.Logf("  %d: %#v\n", idx, stmt)
		if block, ok := stmt.(*Block); ok {
			logBody(t, "block", indent+"  ", block.Body)
		}
	}
	t.Logf(indent+"<end> %s\n", name)
}

func TestBlockDescriptions(t *testing.T) {
	input := strings.Join([]string{
		/*  1 */ `enum Foo {`,
		/*  2 */ `  | This is a description of Foo`,
		/*  3 */ ``,
		/*  4 */ `  option GOOD`,
		/*  5 */ `  BAD | Really Really Bad`,
		/*  6 */ `  UGLY {`,
		/*  7 */ `    | This is a description of UGLY`,
		/*  8 */ `  }`,
		/*  9 */ `}`,
	}, "\n")

	file := tParseFile(t, input)

	logBody(t, "file", "", file.Body)

	assertStatements(t, file.Body.Statements,
		tBlock(
			tBlockType("enum"),
			tBlockTags("Foo"),
			tBlockBody(
				tDescription("This is a description of Foo"),
				tBlock(
					tBlockType("option"),
					tBlockTags("GOOD"),
				),
				tBlock(
					tBlockType("BAD"),
					tBlockDescription("Really Really Bad"),
				),
				tBlock(
					tBlockType("UGLY"),
					tBlockBody(
						tDescription("This is a description of UGLY"),
					),
				),
			),
		),
	)

}

func tString(s string) Value {
	return Value{token: lexer.Token{Type: lexer.STRING, Lit: s}}
}

func tDecimal(s string) Value {
	return Value{token: lexer.Token{Type: lexer.DECIMAL, Lit: s}}
}

var tTrue = Value{token: lexer.Token{Type: lexer.BOOL, Lit: "true"}}
var tFalse = Value{token: lexer.Token{Type: lexer.BOOL, Lit: "false"}}

func tArray(values ...Value) Value {
	return Value{array: values}
}
func tAssignAppend(key string, value ASTValue) tAssertion {
	return func(t *testing.T, s Statement) {
		assign, ok := s.(*Assignment)
		if !ok {
			t.Fatalf("expected Assignment, got %T", s)
		}

		if assign.Key.String() != key {
			t.Fatalf("expected key %q, got %#v", key, assign.Key)
		}

		if !valuesEqual(assign.Value, value) {
			t.Fatalf("expected val %#v, got %#v", value, assign.Value)
		}
		if !assign.Append {
			t.Fatalf("expected append, got false")
		}
	}
}
func tAssign(key string, value ASTValue) tAssertion {
	return func(t *testing.T, s Statement) {
		assign, ok := s.(*Assignment)
		if !ok {
			t.Fatalf("expected Assignment, got %T", s)
		}

		if assign.Key.String() != key {
			t.Fatalf("expected key %q, got %#v", key, assign.Key)
		}

		if !valuesEqual(assign.Value, value) {
			t.Fatalf("expected val %#v, got %#v", value, assign.Value)
		}
		if assign.Append {
			t.Fatalf("expected no append")
		}
	}
}

func valuesEqual(a, b ASTValue) bool {
	aa, aIs := a.AsArray()
	bb, bIs := b.AsArray()
	if aIs || bIs {
		if !aIs || !bIs {
			return false
		}

		if len(aa) != len(bb) {
			return false
		}
		for idx, val := range aa {
			if !valuesEqual(val, bb[idx]) {
				return false
			}
		}
		return true
	}

	av, _ := a.AsString()
	bv, _ := b.AsString()
	return av == bv
}

func tBlockType(part string) blockAssertion {
	return func(t *testing.T, block *Block) {
		if block.Type.String() != part {
			t.Fatalf("expected type %q, got %q", part, block.Type)
		}
	}
}

func tBlockTags(parts ...string) blockAssertion {
	return func(t *testing.T, block *Block) {
		if len(block.Tags) != len(parts) {
			t.Fatalf("expected %d parts in name, got %d", len(parts), len(block.Tags))
		}

		for idx, part := range parts {
			tag := block.Tags[idx]
			tagStr, err := tag.AsString()
			if err != nil {
				t.Fatalf("expected tag %#v to be a string, %s", tag, err)
			}
			if tagStr != part {
				t.Fatalf("expected part %q, got %q", part, tagStr)
			}
		}
	}
}

func tBlockQualifier(qual ...string) blockAssertion {
	return func(t *testing.T, block *Block) {
		if len(block.Qualifiers) != len(qual) {
			t.Fatalf("expected %d qualifiers, got %d", len(qual), len(block.Qualifiers))
		}
		for idx, q := range qual {
			val, err := block.Qualifiers[idx].AsString()
			if err != nil {
				t.Fatalf("expected qualifier to be a string, got %T %s", block.Qualifiers[idx], err)
			}
			if val != q {
				t.Fatalf("expected qualifier %q, got %q", q, val)
			}
		}

	}
}

func tBlockDescription(desc string) blockAssertion {
	return func(t *testing.T, block *Block) {
		if block.Description == nil {
			t.Fatalf("expected description %q, got none", desc)
		}
		str := block.Description.Value
		if str != desc {
			t.Fatalf("expected description %q, got %q", desc, str)
		}

	}
}

type tAssertion func(t *testing.T, s Statement)

func tBlockBody(assertions ...tAssertion) blockAssertion {
	return func(t *testing.T, block *Block) {
		t.Helper()
		t.Logf("block body: %#v", block.Body)
		assertStatements(t, block.Body.Statements, assertions...)
	}
}

type blockAssertion func(*testing.T, *Block)

func tBlock(assertions ...blockAssertion) tAssertion {
	return func(t *testing.T, s Statement) {
		t.Helper()
		block, ok := s.(*Block)
		if !ok {
			t.Fatalf("expected BlockStatement, got %T", s)
		}

		for _, assertion := range assertions {
			assertion(t, block)
		}
	}
}

func tDescription(desc string) tAssertion {
	return func(t *testing.T, s Statement) {
		t.Helper()
		block, ok := s.(*Description)
		if !ok {
			t.Fatalf("expected Description, got %T", s)
		}

		str := block.Value
		if str != desc {
			t.Fatalf("expected description %q, got %q", desc, str)
		}
	}
}

func assertStatements(t *testing.T, statements []Statement, expected ...tAssertion) {
	t.Helper()
	for idx, opt := range statements {
		if idx >= len(expected) {
			t.Errorf("unexpected %#v", opt)
			continue
		}
		runner := expected[idx]
		t.Run(fmt.Sprintf("S%d", idx), func(t *testing.T) {
			t.Helper()
			t.Logf("statement %d: %#v", idx, opt)
			runner(t, opt)
		})
	}

	if len(statements) < len(expected) {
		t.Fatalf("expected %d statements, got %d", len(expected), len(statements))
	}
}
