package lsp

import (
	"context"
	"fmt"
	"log"
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
func (h *langHandler) lint(ctx context.Context, uri DocumentURI) (map[DocumentURI][]Diagnostic, error) {
	req, err := h.buildRequest(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %v", err)
	}

	results, err := h.Handlers.Linter.LintFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("lint error: %v", err)
	}

	uriToDiagnostics := make(map[DocumentURI][]Diagnostic)
	uriToDiagnostics[uri] = results

	return uriToDiagnostics, nil
}
