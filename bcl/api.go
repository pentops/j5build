package bcl

import (
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/lexer"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

func ParseIntoSchema(input string, obj j5reflect.Object, sourceLoc *sourcedef_j5pb.SourceLocation, spec *schema.ConversionSpec) error {
	tree, err := ParseFile(input)
	if err != nil {
		return err
	}

	err = ConvertTreeToSource(tree, obj, sourceLoc, spec)
	if err != nil {
		return errpos.AddSource(err, input)
	}

	return nil
}

func ParseFile(input string) (*ast.File, error) {
	l := lexer.NewLexer(input)

	tokens, err := l.AllTokens()
	if err != nil {
		return nil, errpos.AddSource(err, input)
	}

	tree, err := ast.Walk(tokens)
	if err != nil {
		return nil, errpos.AddSource(err, input)
	}

	return tree, nil
}
