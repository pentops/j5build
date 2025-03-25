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
type OnChange func(filename string, msg protoreflect.Message) error

type Linter struct {
	parser      *bcl.Parser
	fileFactory FileFactory
	onChange    OnChange
}

func New(parser *bcl.Parser, fileFactory FileFactory, validate OnChange) *Linter {
	return &Linter{
		parser:      parser,
		fileFactory: fileFactory,
		onChange:    validate,
	}
}

func NewGeneric() *Linter {
	return &Linter{}
}

func errorToDiagnostics(ctx context.Context, mainError error) ([]protocol.Diagnostic, error) {
	locErr, ok := errpos.AsErrorsWithSource(mainError)
	if !ok {
		log.WithError(ctx, mainError).Error("Error not ErrorsWithSource")
		return nil, mainError
	}

	diagnostics := make([]protocol.Diagnostic, 0, len(locErr.Errors))

	for _, err := range locErr.Errors {
		log.WithFields(ctx, map[string]interface{}{
			"pos":   err.Pos.String(),
			"error": err.Err.Error(),
		}).Debug("Lint Diagnostic")
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
func (l *Linter) FileChanged(ctx context.Context, req *protocol.TextDocumentItem) ([]protocol.Diagnostic, error) {

	// Step 1: Parse BCL
	tree, err := parser.ParseFile(req.Text, false)
	if err != nil {
		if err == parser.HadErrors {
			err = errpos.AddSourceFile(tree.Errors, req.URI.Filename(), req.Text)
			return errorToDiagnostics(ctx, err)
		} else if ews, ok := errpos.AsErrorsWithSource(err); ok {
			return errorToDiagnostics(ctx, ews)
		} else {
			return nil, fmt.Errorf("parse file not HadErrors - : %w", err)
		}
	}

	if l.fileFactory == nil || l.parser == nil {
		return nil, nil
	}

	// Step 2: Parse AST
	msg := l.fileFactory(req.URI.Filename())
	_, err = l.parser.ParseAST(tree, msg)
	if err != nil {
		err = errpos.AddSourceFile(err, req.URI.Filename(), req.Text)
		return errorToDiagnostics(ctx, err)
	}

	// Step 3: Validate
	if l.onChange == nil {
		return nil, nil
	}

	err = l.onChange(req.URI.Filename(), msg)
	if err != nil {
		err = errpos.AddSourceFile(err, req.URI.Filename(), req.Text)
		return errorToDiagnostics(ctx, err)
	}

	log.Debug(ctx, "No errors")
	return []protocol.Diagnostic{}, nil

}

func ptr[T any](v T) *T {
	return &v
}
