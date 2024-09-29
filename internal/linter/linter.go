package linter

import (
	"context"
	"fmt"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type FileFactory func(filename string) protoreflect.Message

type Linter struct {
	parser      *bcl.Parser
	fileFactory FileFactory
}

func New(parser *bcl.Parser, fileFactory FileFactory) *Linter {
	return &Linter{
		parser:      parser,
		fileFactory: fileFactory,
	}
}

func NewGeneric() *Linter {
	return &Linter{}
}

func (l *Linter) LintFile(ctx context.Context, req *lsp.FileRequest) ([]lsp.Diagnostic, error) {

	var mainError error

	tree, err := ast.ParseFile(req.Content, false)
	if err != nil {
		if err == ast.HadErrors {
			mainError = errpos.AddSourceFile(tree.Errors, req.Filename, req.Content)
		} else if ews, ok := errpos.AsErrorsWithSource(err); ok {
			mainError = ews
		} else {
			return nil, fmt.Errorf("parse file not HadErrors - : %w", err)
		}
	}

	if mainError == nil && l.fileFactory != nil && l.parser != nil {
		msg := l.fileFactory(req.Filename)
		_, err = l.parser.ParseAST(tree, msg)
		if err == nil {
			return nil, nil
		}
		mainError = err
	}

	locErr, ok := errpos.AsErrorsWithSource(mainError)
	if !ok {
		return nil, mainError
	}

	for _, err := range locErr.Errors {
		log.WithFields(ctx, map[string]interface{}{
			"pos":   err.Pos.String(),
			"error": err.Err.Error(),
		}).Debug("Lint Diagnostic")
	}

	diagnostics := make([]lsp.Diagnostic, 0, len(locErr.Errors))

	for _, err := range locErr.Errors {
		diagnostics = append(diagnostics, lsp.Diagnostic{
			Range: lsp.Range{
				Start: lsp.Position{
					Line:      err.Pos.Start.Line,
					Character: err.Pos.Start.Column,
				},
				End: lsp.Position{
					Line:      err.Pos.End.Line,
					Character: err.Pos.End.Column,
				},
			},
			Code:     ptr("LINT"),
			Message:  err.Err.Error(),
			Severity: lsp.SeverityError,
			Source:   ptr("bcl"),
		})
	}

	return diagnostics, nil

}

func ptr[T any](v T) *T {
	return &v
}
