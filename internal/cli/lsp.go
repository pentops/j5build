package cli

import (
	"context"
	"log"
	"os"

	"github.com/pentops/bcl.go/bcl/bclsp"
	"github.com/pentops/j5build/internal/conversions/j5parse"
)

func runLSP(ctx context.Context, cfg struct {
	Dir string `flag:"project-root" default:"" desc:"Root schema directory"`
}) error {

	if cfg.Dir == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg.Dir = pwd
	}

	log.Printf("ARGS: %+v", os.Args)

	return bclsp.RunLSP(ctx, bclsp.Config{
		ProjectRoot: cfg.Dir,
		Schema:      j5parse.J5SchemaSpec,
		FileFactory: j5parse.FileStub,
	})
}
