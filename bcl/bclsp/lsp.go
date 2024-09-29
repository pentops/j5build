package bclsp

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/bcl.go/internal/linter"
	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/pentops/log.go/log"
	"github.com/sourcegraph/jsonrpc2"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Config struct {
	ProjectRoot string
	Schema      *bcl_j5pb.Schema
	FileFactory func(filename string) protoreflect.Message
}

type logWrapper struct {
	log.Logger
	ctx context.Context
}

func (l logWrapper) Printf(format string, v ...interface{}) {
	logStr := fmt.Sprintf(format, v...)
	l.Logger.Debug(l.ctx, logStr)
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
	ctx = log.WithField(ctx, "ProjectRoot", config.ProjectRoot)

	handlers := lsp.LSPHandlers{}

	if config.Schema != nil && config.FileFactory != nil {
		parser, err := bcl.NewParser(config.Schema)
		if err != nil {
			return err
		}
		handlers.Linter = linter.New(parser, config.FileFactory)
	} else {
		handlers.Linter = linter.NewGeneric()
	}

	handlers.Fmter = lsp.ASTFormatter{}

	log.Info(ctx, "Starting LSP server")

	conn := jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(lspc, handlers),
		jsonrpc2.LogMessages(logWrapper{Logger: log.DefaultLogger, ctx: ctx}),
	)

	select {
	case <-ctx.Done():
		log.Info(ctx, "Context Done")
		conn.Close()

		return ctx.Err()
	case <-conn.DisconnectNotify():
		log.Info(ctx, "Disconnect Notify")
	}

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
