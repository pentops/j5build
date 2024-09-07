package lsp

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"time"
)

func (h *langHandler) lintRequest(uri DocumentURI, eventType eventType) {
	log.Printf("lintRequest: %v", uri)
	if h.lintTimer != nil {
		h.lintTimer.Reset(h.lintDebounce)
		log.Printf("lintRequest: reset timer")
		return
	}
	h.lintTimer = time.AfterFunc(h.lintDebounce, func() {
		h.lintTimer = nil
		h.request <- lintRequest{URI: uri, EventType: eventType}
	})
}

type eventType int

const (
	eventTypeChange eventType = iota
	eventTypeSave
	eventTypeOpen
)

type lintRequest struct {
	URI       DocumentURI
	EventType eventType
}

func (h *langHandler) linter() {
	running := make(map[DocumentURI]context.CancelFunc)

	for {
		lintReq, ok := <-h.request
		if !ok {
			break
		}

		cancel, ok := running[lintReq.URI]
		if ok {
			cancel()
		}

		ctx, cancel := context.WithCancel(context.Background())
		running[lintReq.URI] = cancel

		go func() {
			uriToDiagnostics, err := h.lint(ctx, lintReq.URI)
			if err != nil {
				log.Printf("lint error: %v", err)
				return
			}

			for diagURI, diagnostics := range uriToDiagnostics {
				if diagURI == "file:" {
					diagURI = lintReq.URI
				}
				version := 0
				if _, ok := h.files[lintReq.URI]; ok {
					version = h.files[lintReq.URI].Version
				}
				if diagnostics == nil {
					diagnostics = []Diagnostic{}
				}
				err = h.conn.Notify(
					ctx,
					"textDocument/publishDiagnostics",
					&PublishDiagnosticsParams{
						URI:         diagURI,
						Diagnostics: diagnostics,
						Version:     version,
					})
				if err != nil {
					log.Printf("error publishing diagnostics: %v", err)
				}
			}
		}()
	}
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

func (h *langHandler) lint(ctx context.Context, uri DocumentURI) (map[DocumentURI][]Diagnostic, error) {

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

	req := LintFileRequest{
		Filename: relFile,
		Content:  file.Text,
	}

	results, err := h.Handlers.Linter.LintFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("lint error: %v", err)
	}

	uriToDiagnostics := make(map[DocumentURI][]Diagnostic)
	uriToDiagnostics[uri] = results

	return uriToDiagnostics, nil
}
