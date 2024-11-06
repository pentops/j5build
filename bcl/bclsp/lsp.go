package bclsp

import (
	"context"
	"os"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/bcl.go/internal/linter"
	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/pentops/bcl.go/lib/genlsp"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Config struct {
	ProjectRoot string
	Schema      *bcl_j5pb.Schema
	FileFactory func(filename string) protoreflect.Message
}

func RunLSP(ctx context.Context, config Config) error {

	lspc := genlsp.LSPConfig{
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

	if config.Schema != nil && config.FileFactory != nil {
		parser, err := bcl.NewParser(config.Schema)
		if err != nil {
			return err
		}
		lspc.Linter = linter.New(parser, config.FileFactory)
	} else {
		lspc.Linter = linter.NewGeneric()
	}

	lspc.Formatter = lsp.ASTFormatter{}

	log.Info(ctx, "Starting LSP server")

	ss, err := genlsp.NewServerStream(lspc)
	if err != nil {
		return err
	}

	return ss.Run(ctx, genlsp.StdIO())
}
