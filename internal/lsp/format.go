package lsp

import (
	"context"

	"github.com/pentops/bcl.go/internal/parser"
	"go.lsp.dev/protocol"
)

type ASTFormatter struct{}

func (f ASTFormatter) Format(ctx context.Context, doc *protocol.TextDocumentItem) ([]protocol.TextEdit, error) {
	diffs, err := parser.FmtDiffs(doc.Text)
	if err != nil {
		return nil, err
	}

	edits := make([]protocol.TextEdit, 0, len(diffs))
	for _, diff := range diffs {
		edits = append(edits, protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(diff.FromLine), Character: 0},
				End:   protocol.Position{Line: uint32(diff.ToLine), Character: 0},
			},
			NewText: diff.NewText,
		})
	}
	return edits, nil
}
