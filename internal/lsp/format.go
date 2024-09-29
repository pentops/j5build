package lsp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pentops/bcl.go/internal/ast"
	"github.com/sourcegraph/jsonrpc2"
)

func (h *langHandler) handleTextDocumentFormatting(ctx context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DocumentFormattingParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	doc, err := h.buildRequest(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %v", err)
	}

	diffs, err := h.Handlers.Fmter.FormatFile(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to format: %v", err)
	}
	return diffs, nil

}

type ASTFormatter struct{}

func (f ASTFormatter) FormatFile(ctx context.Context, doc *FileRequest) ([]TextEdit, error) {
	diffs, err := ast.FmtDiffs(doc.Content)
	if err != nil {
		return nil, err
	}

	edits := make([]TextEdit, 0, len(diffs))
	for _, diff := range diffs {
		edits = append(edits, TextEdit{
			Range: Range{
				Start: Position{Line: diff.FromLine, Character: 0},
				End:   Position{Line: diff.ToLine, Character: 0},
			},
			NewText: diff.NewText,
		})
	}
	return edits, nil
}
