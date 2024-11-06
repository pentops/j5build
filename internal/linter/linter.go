package linter

import (
	"context"
	"fmt"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/parser"
	"github.com/pentops/log.go/log"
	"go.lsp.dev/protocol"
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

func (l *Linter) Lint(ctx context.Context, req *protocol.TextDocumentItem) ([]protocol.Diagnostic, error) {

	var mainError error

	tree, err := parser.ParseFile(req.Text, false)
	if err != nil {
		if err == parser.HadErrors {
			mainError = errpos.AddSourceFile(tree.Errors, req.URI.Filename(), req.Text)
		} else if ews, ok := errpos.AsErrorsWithSource(err); ok {
			mainError = ews
		} else {
			return nil, fmt.Errorf("parse file not HadErrors - : %w", err)
		}
	}

	if mainError == nil && l.fileFactory != nil && l.parser != nil {
		msg := l.fileFactory(req.URI.Filename())
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

	diagnostics := make([]protocol.Diagnostic, 0, len(locErr.Errors))

	for _, err := range locErr.Errors {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(err.Pos.Start.Line),
					Character: uint32(err.Pos.Start.Column),
				},
				End: protocol.Position{
					Line:      uint32(err.Pos.End.Line),
					Character: uint32(err.Pos.End.Column),
				},
			},
			Code:     ptr("LINT"),
			Message:  err.Err.Error(),
			Severity: protocol.DiagnosticSeverityError,
			Source:   "bcl",
		})
	}

	return diagnostics, nil

}

func ptr[T any](v T) *T {
	return &v
}
