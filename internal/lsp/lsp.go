package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/sourcegraph/jsonrpc2"
)

type FileRequest struct {
	Filename string
	Content  string
}

type Linter interface {
	LintFile(context.Context, *FileRequest) ([]Diagnostic, error)
}

type Fmter interface {
	FormatFile(context.Context, *FileRequest) ([]TextEdit, error)
}

type LSPHandlers struct {
	Linter Linter
	Fmter  Fmter
}

type LSPConfig struct {
	ProjectRoot string
}

func NewHandler(config LSPConfig, handlers LSPHandlers) jsonrpc2.Handler {
	handler := &langHandler{
		lintTimer:    nil,
		lintDebounce: 500 * time.Millisecond,
		files:        make(map[DocumentURI]*File),
		request:      make(chan lintRequest),
		Handlers:     handlers,
		Config:       config,
	}

	go handler.linter()
	return jsonrpc2.HandlerWithError(handler.handle)
}

// File is
type File struct {
	LanguageID string
	Text       string
	Version    int
}

type langHandler struct {
	Handlers LSPHandlers
	Config   LSPConfig

	lintTimer    *time.Timer
	lintDebounce time.Duration
	request      chan lintRequest
	conn         *jsonrpc2.Conn
	files        map[DocumentURI]*File
}

func (h *langHandler) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {

	switch req.Method {
	case "initialize":
		return h.handleInitialize(ctx, conn, req)
	case "initialized":
		return
	case "shutdown":
		return h.handleShutdown(ctx, conn, req)
	case "textDocument/didOpen":
		return h.handleTextDocumentDidOpen(ctx, conn, req)
	case "textDocument/didChange":
		return h.handleTextDocumentDidChange(ctx, conn, req)
	case "textDocument/didSave":
		return h.handleTextDocumentDidSave(ctx, conn, req)
	case "textDocument/didClose":
		return h.handleTextDocumentDidClose(ctx, conn, req)
	case "textDocument/formatting":
		return h.handleTextDocumentFormatting(ctx, conn, req)
	}

	return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: fmt.Sprintf("method not supported: %s", req.Method)}
}

func (h *langHandler) handleInitialize(_ context.Context, conn *jsonrpc2.Conn, _ *jsonrpc2.Request) (result any, err error) {
	h.conn = conn
	return &InitializeResult{
		Capabilities: ServerCapabilities{
			DocumentFormattingProvider: true,
			TextDocumentSync: TextDocumentSyncOptions{
				OpenClose: true,
				Change:    TDSKFull,
				Save: SaveOptions{
					IncludeText: true,
				},
			},
		},
	}, nil
}

func (h *langHandler) handleShutdown(_ context.Context, conn *jsonrpc2.Conn, _ *jsonrpc2.Request) (result any, err error) {
	return nil, conn.Close()
}

func fromURI(uri DocumentURI) (string, error) {
	u, err := url.ParseRequestURI(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("only file URIs are supported, got %v", u.Scheme)
	}
	return u.Path, nil
}

func (h *langHandler) buildRequest(uri DocumentURI) (*FileRequest, error) {
	file, ok := h.files[uri]
	if !ok {
		return nil, fmt.Errorf("document not found: %v", uri)
	}

	fname, err := fromURI(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid uri: %v: %v", err, uri)
	}
	fname = filepath.ToSlash(fname)

	relFile, err := filepath.Rel(h.Config.ProjectRoot, fname)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %v", err)
	}

	req := &FileRequest{
		Filename: relFile,
		Content:  file.Text,
	}
	return req, nil
}

func (h *langHandler) handleTextDocumentDidOpen(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	if err := h.openFile(params.TextDocument.URI, params.TextDocument.LanguageID, params.TextDocument.Version); err != nil {
		return nil, err
	}
	if err := h.updateFile(params.TextDocument.URI, params.TextDocument.Text, &params.TextDocument.Version, eventTypeOpen); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *langHandler) handleTextDocumentDidSave(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	if params.Text != nil {
		err = h.updateFile(params.TextDocument.URI, *params.Text, nil, eventTypeSave)
	} else {
		err = h.saveFile(params.TextDocument.URI)
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *langHandler) handleTextDocumentDidClose(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	if err := h.closeFile(params.TextDocument.URI); err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *langHandler) handleTextDocumentDidChange(_ context.Context, _ *jsonrpc2.Conn, req *jsonrpc2.Request) (result any, err error) {
	if req.Params == nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}

	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(*req.Params, &params); err != nil {
		return nil, err
	}

	if len(params.ContentChanges) == 1 {
		if err := h.updateFile(params.TextDocument.URI, params.ContentChanges[0].Text, &params.TextDocument.Version, eventTypeChange); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (h *langHandler) updateFile(uri DocumentURI, text string, version *int, eventType eventType) error {
	f, ok := h.files[uri]
	if !ok {
		return fmt.Errorf("document not found: %v", uri)
	}
	f.Text = text
	if version != nil {
		f.Version = *version
	}

	h.lintRequest(uri, eventType)
	return nil
}

func (h *langHandler) closeFile(uri DocumentURI) error {
	delete(h.files, uri)
	return nil
}

func (h *langHandler) saveFile(uri DocumentURI) error {

	h.lintRequest(uri, eventTypeSave)
	return nil
}

func (h *langHandler) openFile(uri DocumentURI, languageID string, version int) error {
	f := &File{
		Text:       "",
		LanguageID: languageID,
		Version:    version,
	}
	h.files[uri] = f
	return nil
}
