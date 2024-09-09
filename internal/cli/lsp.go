package cli

import (
	"context"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pentops/bcl.go/bcl/bclsp"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/conversions/j5parse"
	"google.golang.org/protobuf/reflect/protoreflect"
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

	fileStub := func(sourceFilename string) protoreflect.Message {
		dirName, _ := path.Split(sourceFilename)
		dirName = strings.TrimSuffix(dirName, "/")

		pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
		file := &sourcedef_j5pb.SourceFile{
			Path: sourceFilename,
			Package: &sourcedef_j5pb.Package{
				Name: pathPackage,
			},
			SourceLocations: &bcl_j5pb.SourceLocation{},
		}
		refl := file.ProtoReflect()

		return refl
	}

	return bclsp.RunLSP(ctx, bclsp.Config{
		ProjectRoot: cfg.Dir,
		Schema:      j5parse.J5SchemaSpec,
		FileFactory: fileStub,
	})
}
