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
	OnChange    func(filename string, parsed protoreflect.Message) error
}

func BuildLSPHandler(config Config) (*genlsp.LSPConfig, error) {
	lspc := genlsp.LSPConfig{
		ProjectRoot: config.ProjectRoot,
	}

	if config.ProjectRoot == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		config.ProjectRoot = pwd
	}

	if config.Schema != nil && config.FileFactory != nil {
		parser, err := bcl.NewParser(config.Schema)
		if err != nil {
			return nil, err
		}
		lspc.OnChange = linter.New(parser, config.FileFactory, config.OnChange)
	} else {
		lspc.OnChange = linter.NewGeneric()
	}

	lspc.Formatter = lsp.ASTFormatter{}

	return &lspc, nil

}

func RunLSP(ctx context.Context, config Config) error {
	lspc, err := BuildLSPHandler(config)
	if err != nil {
		return err
	}

	ctx = log.WithField(ctx, "ProjectRoot", config.ProjectRoot)
	log.Info(ctx, "Starting LSP server")

	ss, err := genlsp.NewServerStream(*lspc)
	if err != nil {
		return err
	}

	return ss.Run(ctx, genlsp.StdIO())
}
