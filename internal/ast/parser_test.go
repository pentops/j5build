package ast

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
		tBlock(tBlockName("package", "pentops.j5lang.example")),
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

func TestBlockQualifier(t *testing.T) {
	input := `block Foo:type`

	file := tParseFile(t, input)

	assertStatements(t, file.Body.Statements,
		tBlock(
			tBlockName("block", "Foo"),
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

	assertStatements(t, file.Statements(),
		tBlock(tBlockName("import", "base.baz"), tBlockQualifier("baz")),
		tBlock(tBlockName("import", "base.bar")),
		tBlock(
			tBlockName("block", "Foo"),
			tBlockBody(
				tBlock(tBlockName("export")),
				tBlock(tBlockName("include", "bar.a")),
				tBlock(tBlockName("include", "baz.b")),
				tAssign("k", tString("v")),
			),
		),
	)
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

	assertStatements(t, file.Body.Statements,
		tBlock(
			tBlockName("enum", "Foo"),
			tBlockDescription("This is a description of Foo"),
			tBlockBody(
				tBlock(
					tBlockName("option", "GOOD"),
				),
				tBlock(
					tBlockName("BAD"),
					tBlockDescription("Really Really Bad"),
				),
				tBlock(
					tBlockName("UGLY"),
					tBlockDescription("This is a description of UGLY"),
				),
			),
		),
	)

	if len(file.Body.Statements) != 1 {
		t.Fatalf("expected 1 decl in file, got %d", len(file.Body.Statements))
	}

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

func tAssign(key string, value ASTValue) tAssertion {
	return func(t *testing.T, s Statement) {
		assign, ok := s.(Assignment)
		if !ok {
			t.Fatalf("expected Assignment, got %T", s)
		}

		if assign.Key.String() != key {
			t.Fatalf("expected key %q, got %#v", key, assign.Key)
		}

		if !valuesEqual(assign.Value, value) {
			t.Fatalf("expected val %#v, got %#v", value, assign.Value)
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

/*
	func tImport(path, alias string) tAssertion {
		return func(t *testing.T, s Statement) {
			imp, ok := s.(ImportStatement)
			if !ok {
				t.Fatalf("expected ImportStatement, got %T", s)
			}

			if imp.Path != path {
				t.Fatalf("expected path %q, got %q", path, imp.Path)
			}

			if imp.Alias != alias {
				t.Fatalf("expected alias %q, got %q", alias, imp.Alias)
			}
		}
	}

	func tDirective(key string, value string) tAssertion {
		return func(t *testing.T, s Statement) {
			directive, ok := s.(Directive)
			if !ok {
				t.Fatalf("expected Directive, got %#v", s)
			}

			if directive.Key.String() != key {
				t.Fatalf("expected key %q, got %#v", key, directive.Key)
			}

			if directive.Value.token.Lit != value {
				t.Fatalf("expected val %s, got %#v", value, directive.Value)
			}
		}
	}
*/
func tBlockName(parts ...string) blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		if len(block.Name) != len(parts) {
			t.Fatalf("expected %d parts in name, got %d", len(parts), len(block.Name))
		}

		for idx, part := range parts {
			if block.Name[idx].String() != part {
				t.Fatalf("expected part %q, got %q", part, block.Name[idx])
			}
		}
	}
}

func tBlockQualifier(qual ...string) blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		if len(block.Qualifiers) != len(qual) {
			t.Fatalf("expected %d qualifiers, got %d", len(qual), len(block.Qualifiers))
		}
		for idx, q := range qual {
			if block.Qualifiers[idx].String() != q {
				t.Fatalf("expected qualifier %q, got %q", q, block.Qualifiers[idx])
			}
		}

	}
}

func tBlockDescription(desc string) blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		bs, _ := block.Description.AsString()
		if bs != desc {
			t.Fatalf("expected description %q, got %#v", desc, block.Description)
		}
	}
}

func tBlockBody(assertions ...tAssertion) blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		assertStatements(t, block.Body.Statements, assertions...)
	}
}

func tExport() blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		if !block.Export {
			t.Fatalf("expected export, got none")
		}
	}
}

/*
func tIncludes(includes ...string) blockAssertion {
	return func(t *testing.T, block BlockStatement) {
		if len(block.Body.Includes) != len(includes) {
			t.Fatalf("expected %d includes, got %d", len(includes), len(block.Body.Includes))
		}

		for idx, inc := range includes {
			if block.Body.Includes[idx].String() != inc {
				t.Fatalf("expected include %q, got %q", inc, block.Body.Includes[idx])
			}
		}
	}
}*/

type blockAssertion func(*testing.T, BlockStatement)

func tBlock(assertions ...blockAssertion) tAssertion {
	return func(t *testing.T, s Statement) {
		block, ok := s.(BlockStatement)
		if !ok {
			t.Fatalf("expected BlockStatement, got %T", s)
		}

		for _, assertion := range assertions {
			assertion(t, block)
		}
	}
}

type tAssertion func(t *testing.T, s Statement)

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
