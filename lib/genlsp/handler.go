package genlsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pentops/log.go/log"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type Formatter interface {
	Format(context.Context, *protocol.TextDocumentItem) ([]protocol.TextEdit, error)
}

type ChangeHandler interface {
	FileChanged(context.Context, *protocol.TextDocumentItem) ([]protocol.Diagnostic, error)
}

type LSPConfig struct {
	ProjectRoot string

	Formatter Formatter
	OnChange  ChangeHandler
}

type ServerStream struct {
	files      *fileSet
	dispatcher replyServer

	Formatter     Formatter
	ChangeHandler ChangeHandler
}

type replyServer interface {
	Notify(context.Context, string, interface{}) error
}

func NewServerStream(cfg LSPConfig) (*ServerStream, error) {
	files, err := newFileSet(cfg.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create file set: %w", err)
	}
	ss := &ServerStream{
		files:         files,
		Formatter:     cfg.Formatter,
		ChangeHandler: cfg.OnChange,
	}

	dbchange := newDebounce(500, ss.fileDidChange)
	files.onChange = dbchange.request

	return ss, nil
}

func (ss *ServerStream) fileDidChange(ctx context.Context, doc *protocol.TextDocumentItem) {
	err := ss.fileDidChangeErr(ctx, doc)
	if err != nil {
		log.WithError(ctx, err).Error("failed to handle file change")
	}
}

func (ss *ServerStream) fileDidChangeErr(ctx context.Context, doc *protocol.TextDocumentItem) error {
	if ss.ChangeHandler == nil {
		return nil
	}
	diagnostics, err := ss.ChangeHandler.FileChanged(ctx, doc)
	if err != nil {
		return err
	}
	if diagnostics == nil {
		// clear the diagnostics
		diagnostics = []protocol.Diagnostic{}
	}
	return ss.dispatcher.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, &protocol.PublishDiagnosticsParams{
		URI:         doc.URI,
		Diagnostics: diagnostics,
	})
}

func (ss *ServerStream) Run(ctx context.Context, rwc io.ReadWriteCloser) error {
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(rwc))
	ss.dispatcher = conn
	conn.Go(ctx, ss.handle)
	<-conn.Done()
	return nil
}

func doReqRes[REQ, RES any](ctx context.Context, replier jsonrpc2.Replier, jRequest jsonrpc2.Request, cb func(context.Context, *REQ) (RES, error)) error {
	params := new(REQ)
	if err := json.Unmarshal(jRequest.Params(), &params); err != nil {
		return replyParseError(ctx, replier, err)
	}
	res, err := cb(ctx, params)
	return replier(ctx, res, err)
}

func doReq[REQ any](ctx context.Context, replier jsonrpc2.Replier, jRequest jsonrpc2.Request, cb func(context.Context, *REQ) error) error {
	params := new(REQ)
	if err := json.Unmarshal(jRequest.Params(), &params); err != nil {
		return replyParseError(ctx, replier, err)
	}
	err := cb(ctx, params)
	return replier(ctx, nil, err)
}

func replyParseError(ctx context.Context, reply jsonrpc2.Replier, err error) error {
	return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
}

func (h *ServerStream) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	method := req.Method()
	ctx = log.WithField(ctx, "method", method)
	log.Debug(ctx, "handling request")

	switch method {
	case protocol.MethodInitialize:
		return doReqRes(ctx, reply, req, h.Initialize)
	case protocol.MethodInitialized:
		return doReq(ctx, reply, req, h.Initialized)
	case protocol.MethodTextDocumentDidOpen:
		return doReq(ctx, reply, req, h.files.DidOpen)
	case protocol.MethodTextDocumentDidClose:
		return doReq(ctx, reply, req, h.files.DidClose)
	case protocol.MethodTextDocumentDidChange:
		return doReq(ctx, reply, req, h.files.DidChange)
	case protocol.MethodTextDocumentDidSave:
		return doReq(ctx, reply, req, h.files.DidSave)
	case protocol.MethodTextDocumentFormatting:
		return doReqRes(ctx, reply, req, h.Formatting)
	default:
		return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
	}

}
func (h *ServerStream) Initialize(_ context.Context, req *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			DocumentFormattingProvider: true,
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
		},
	}, nil
}

func (h *ServerStream) Initialized(_ context.Context, _ *protocol.InitializedParams) error {
	return nil
}

func (h *ServerStream) Shutdown(_ context.Context) error {
	return nil
}

func (h *ServerStream) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	if h.Formatter == nil {
		return nil, fmt.Errorf("formatter not available")
	}

	doc, err := h.files.getDocument(ctx, params.TextDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return h.Formatter.Format(ctx, doc)
}
