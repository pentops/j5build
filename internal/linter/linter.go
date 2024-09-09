package linter

import (
	"context"
	"log"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lsp"
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

func (l *Linter) LintFile(ctx context.Context, req lsp.LintFileRequest) ([]lsp.Diagnostic, error) {

	msg := l.fileFactory(req.Filename)
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
