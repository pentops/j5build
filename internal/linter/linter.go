package linter

import (
	"context"
	"log"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lsp"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Linter struct {
	parser *bcl.Parser
}

func New(parser *bcl.Parser) *Linter {
	return &Linter{
		parser: parser,
	}
}

func (l *Linter) LintFile(ctx context.Context, req lsp.LintFileRequest, msg protoreflect.Message) ([]lsp.Diagnostic, error) {

	_, mainError := l.parser.ParseFile(req.Filename, req.Content, msg)
	if mainError == nil {
		return nil, nil
	}

	locErr, ok := errpos.AsErrorsWithSource(mainError)
	if !ok {
		return nil, mainError
	}

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
