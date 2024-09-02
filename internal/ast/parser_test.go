package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/bcl/errpos"
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

	file, err := ParseFile(input, false)
	if err != HadErrors {
		t.Fatalf("Err is not HadErrors, was: %v", err)
	}

	if file.Errors == nil {
		t.Fatalf("FATAL: expected errors, got none")
	}

	errors, err := errpos.MustAddSource(errpos.Errors(file.Errors), input)
	if err != nil {
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

	if file.Package != "pentops.j5lang.example" {
		t.Errorf("expected package pentops.j5lang.example, got %s", file.Package)
	}
	assertStatements(t, file.Body.Statements,
		tAssign("version", "v1"),
		tAssign("number", "123"),
		tAssign("bool", "true"),
		tAssign("float", "1.23"),
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
		`import base.baz as baz`,
		`import base.bar`,
		`block Foo {`,
		`  export`,
		`  include bar.a`,
		`  include baz.b`,
		`  k = v`,
		`}`,
	}, "\n")

	file := tParseFile(t, input)

	assertImports(t, file.Imports(),
		ImportStatement{Path: "base.baz", Alias: "baz"},
		ImportStatement{Path: "base.bar", Alias: ""},
	)

	assertStatements(t, file.Statements(),
		tBlock(
			tBlockName("block", "Foo"),
			tExport(),
			tIncludes("bar.a", "baz.b"),
			tBlockBody(
				tAssign("k", "v"),
			),
		),
	)
}

func assertImports(t *testing.T, imports []*ImportStatement, expected ...ImportStatement) {
	t.Helper()
	if len(imports) != len(expected) {
		t.Fatalf("expected %d imports, got %d", len(expected), len(imports))
	}

	for idx, imp := range imports {
		if imp.Path != expected[idx].Path {
			t.Errorf("expected path %q, got %q", expected[idx].Path, imp.Path)
		}

		if imp.Alias != expected[idx].Alias {
			t.Errorf("expected alias %q, got %q", expected[idx].Alias, imp.Alias)
		}
	}
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

func tAssign(key string, value string) tAssertion {
	return func(t *testing.T, s Statement) {
		assign, ok := s.(Assignment)
		if !ok {
			t.Fatalf("expected Assignment, got %T", s)
		}

		if assign.Key.String() != key {
			t.Fatalf("expected key %q, got %#v", key, assign.Key)
		}

		if assign.Value.token.Lit != value {
			t.Fatalf("expected val %s, got %#v", value, assign.Value)
		}
	}
}

/*
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
}

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
	for idx, opt := range statements {
		if idx >= len(expected) {
			t.Errorf("unexpected %#v", opt)
			continue
		}
		runner := expected[idx]
		t.Run(fmt.Sprintf("S%d", idx), func(t *testing.T) {
			t.Logf("statement %d: %#v", idx, opt)
			runner(t, opt)
		})
	}

	if len(statements) < len(expected) {
		t.Fatalf("expected %d statements, got %d", len(expected), len(statements))
	}
}
