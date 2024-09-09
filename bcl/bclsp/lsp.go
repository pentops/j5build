package bclsp

import (
	"context"
	"log"
	"os"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/internal/linter"
	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/sourcegraph/jsonrpc2"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Config struct {
	ProjectRoot string
	Schema      *bcl_j5pb.Schema
	FileFactory func(filename string) protoreflect.Message
}

func RunLSP(ctx context.Context, config Config) error {

	lspc := lsp.LSPConfig{
		ProjectRoot: config.ProjectRoot,
	}

	if config.ProjectRoot == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		config.ProjectRoot = pwd
	}
	log.Printf("ProjectRoot: %v", config.ProjectRoot)

	parser, err := bcl.NewParser(config.Schema)
	if err != nil {
		return err
	}
	bclLinter := linter.New(parser, config.FileFactory)

	handlers := lsp.LSPHandlers{
		Linter: bclLinter,
	}

	log.Printf("BEGIN")
	conn := jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(lspc, handlers),
		jsonrpc2.LogMessages(log.Default()),
	)

	select {
	case <-ctx.Done():
		log.Printf("DONE")

		conn.Close()

		return ctx.Err()
	case <-conn.DisconnectNotify():
		log.Printf("DISCONNECT")
	}

	log.Printf("END")
	return nil
}

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (c stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (c stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}
