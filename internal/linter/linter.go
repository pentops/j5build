package linter

import (
	"context"
	"log"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/bcl.go/internal/lsp"
)

type Linter struct {
	parser *j5parse.Parser
}

func New(parser *j5parse.Parser) *Linter {
	return &Linter{
		parser: parser,
	}
}

func (l *Linter) LintFile(ctx context.Context, req lsp.LintFileRequest) ([]lsp.Diagnostic, error) {

	log.Printf("CONTENT\n%s\n", req.Content)

	_, mainError := l.parser.ParseFile(req.Filename, req.Content)
	if mainError == nil {
		return nil, nil
	}

	locErr, ok := errpos.AsErrorsWithSource(mainError)
	if !ok {
		return nil, mainError
	}

	log.Printf("OUTPUT\n")

	log.Println(locErr.HumanString(2))

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
