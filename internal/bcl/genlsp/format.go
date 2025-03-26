package genlsp

import (
	"context"

	"github.com/pentops/j5build/internal/bcl/internal/parser"
	"go.lsp.dev/protocol"
)

type astFormatter struct{}

func (f astFormatter) Format(ctx context.Context, doc *protocol.TextDocumentItem) ([]protocol.TextEdit, error) {
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
