package genlsp

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pentops/log.go/log"
	"go.lsp.dev/protocol"
)

// fileSet syncs a local file system with the client.
type fileSet struct {
	root   fs.FS
	files  map[string]*protocol.TextDocumentItem
	prefix string

	onChange func(context.Context, *protocol.TextDocumentItem)
}

//var _ fs.FS = &FileSet{}

func newFileSet(root string) (*fileSet, error) {
	// TODO: Support windows paths
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	rootFS := os.DirFS(root)
	return &fileSet{
		root:   rootFS,
		prefix: fmt.Sprintf("file://%s/", root),
		files:  make(map[string]*protocol.TextDocumentItem),
	}, nil
}

func (fs *fileSet) relativeURL(uri protocol.DocumentURI) (string, error) {
	if !strings.HasPrefix(string(uri), fs.prefix) {
		return "", fmt.Errorf("expected file URI, got %v", uri)
	}
	relPath := strings.TrimPrefix(string(uri), fs.prefix)
	return relPath, nil
}

func (fs *fileSet) getDocument(_ context.Context, docID protocol.TextDocumentIdentifier) (*protocol.TextDocumentItem, error) {
	uri := docID.URI
	local, err := fs.relativeURL(uri)
	if err != nil {
		return nil, err
	}
	doc, ok := fs.files[local]
	if !ok {
		return nil, fmt.Errorf("document not open: %v", uri)
	}
	return doc, nil
}

func (fs *fileSet) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	ctx = log.WithField(ctx, "fileURI", params.TextDocument.URI)
	local, err := fs.relativeURL(params.TextDocument.URI)
	if err != nil {
		return err
	}

	log.WithFields(ctx, map[string]interface{}{
		"version":  params.TextDocument.Version,
		"language": params.TextDocument.LanguageID,
		"textLen":  len(params.TextDocument.Text),
		"local":    local,
	}).Debug("DidOpen")
	fs.files[local] = &params.TextDocument
	return nil
}

func (fs *fileSet) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	ctx = log.WithField(ctx, "fileURI", params.TextDocument.URI)
	local, err := fs.relativeURL(params.TextDocument.URI)
	if err != nil {
		return err
	}
	log.WithFields(ctx, map[string]interface{}{
		"version": params.TextDocument.Version,
		"changes": len(params.ContentChanges),
		"local":   local,
	}).Debug("DidChange")
	if len(params.ContentChanges) != 1 {
		return fmt.Errorf("expected exactly one content change, got %v", len(params.ContentChanges))
	}

	file, ok := fs.files[local]
	if !ok {
		return fmt.Errorf("document not open: %v", params.TextDocument.URI)
	}
	file.Text = params.ContentChanges[0].Text
	file.Version = params.TextDocument.Version

	if fs.onChange != nil {
		fs.onChange(ctx, file)
	}

	return nil
}

func (fs *fileSet) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	ctx = log.WithField(ctx, "fileURI", params.TextDocument.URI)
	local, err := fs.relativeURL(params.TextDocument.URI)
	if err != nil {
		return err
	}
	log.WithFields(ctx, map[string]interface{}{
		"local": local,
	}).Debug("DidClose")
	delete(fs.files, local)
	return nil
}

func (fs *fileSet) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	ctx = log.WithField(ctx, "fileURI", params.TextDocument.URI)
	local, err := fs.relativeURL(params.TextDocument.URI)
	if err != nil {
		return err
	}
	log.WithFields(ctx, map[string]interface{}{
		"textLen": len(params.Text),
		"local":   local,
	}).Debug("DidSave")
	return nil
}
