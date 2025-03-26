package genlsp

import (
	"context"
	"os"

	"github.com/pentops/j5build/internal/bcl"
	"github.com/pentops/j5build/internal/bcl/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5build/internal/bcl/internal/linter"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Config struct {
	ProjectRoot string
	Schema      *bcl_j5pb.Schema
	FileFactory func(filename string) protoreflect.Message
	OnChange    func(filename string, parsed protoreflect.Message) error
}

func BuildLSPHandler(config Config) (*lspConfig, error) {
	lspc := lspConfig{
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

	lspc.Formatter = astFormatter{}

	return &lspc, nil

}

func RunLSP(ctx context.Context, config Config) error {
	lspc, err := BuildLSPHandler(config)
	if err != nil {
		return err
	}

	ctx = log.WithField(ctx, "ProjectRoot", config.ProjectRoot)
	log.Info(ctx, "Starting LSP server")

	ss, err := newServerStream(*lspc)
	if err != nil {
		return err
	}

	return ss.Run(ctx, stdIO())
}
