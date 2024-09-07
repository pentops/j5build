package cli

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pentops/j5build/internal/conversions/protobuild"
	"github.com/pentops/j5build/lib/j5source"
	"github.com/pentops/prototools/protoprint"
)

func runGenProto(ctx context.Context, cfg struct {
	Dir           string `flag:"dir" default:"." desc:"Root schema directory"`
	Bundle        string `flag:"bundle" default:"" desc:"Bundle file"`
	Verbose       bool   `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
	DebugProtoAST bool   `flag:"debug-proto-ast" default:"false" desc:"Print proto AST to output dir"`
}) error {

	outWriter := &fileWriter{dir: cfg.Dir}

	source, err := j5source.NewFSSource(ctx, os.DirFS(cfg.Dir))
	if err != nil {
		return err
	}

	bundleConfig, err := source.BundleConfig(cfg.Bundle)
	if err != nil {
		return err
	}

	bundleFS, err := source.BundleFS(cfg.Bundle)
	if err != nil {
		return err
	}

	packages := []string{}
	for _, pkg := range bundleConfig.Packages {
		packages = append(packages, pkg.Name)
	}

	localFiles := &fileReader{
		fs:       bundleFS,
		packages: packages,
	}

	deps, err := source.BundleDependencies(ctx, cfg.Bundle)
	if err != nil {
		return err
	}

	resolver, err := protobuild.NewResolver(deps, localFiles)
	if err != nil {
		return err
	}

	compiler := protobuild.NewCompiler(resolver)

	for _, pkg := range packages {
		out, err := compiler.CompilePackage(ctx, pkg)
		if err != nil {
			return err
		}

		for _, file := range out {
			filename := file.Path()
			if !strings.HasSuffix(filename, ".j5s.proto") {
				continue
			}

			out, err := protoprint.PrintFile(ctx, file)
			if err != nil {
				log.Printf("Error printing %s: %v", filename, err)
				return err
			}

			err = outWriter.PutFile(ctx, filename, []byte(out))
			if err != nil {
				return err
			}

		}

	}

	return nil

}

type fileWriter struct {
	dir string
}

func (f *fileWriter) PutFile(ctx context.Context, filename string, data []byte) error {
	dir := path.Join(f.dir, path.Dir(filename))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path.Join(f.dir, filename), data, 0644)
}

type fileReader struct {
	fs       fs.FS
	packages []string
}

func (rr *fileReader) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	return fs.ReadFile(rr.fs, filename)
}

func (rr *fileReader) ListPackages() []string {
	return rr.packages
}

func (rr *fileReader) ListSourceFiles(ctx context.Context, pkgName string) ([]string, error) {
	pkgRoot := strings.ReplaceAll(pkgName, ".", "/")

	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, pkgRoot, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5gen.proto") {
			return nil
		}
		if strings.HasSuffix(path, ".proto") {
			files = append(files, path)
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (rr *fileReader) ListJ5Files(ctx context.Context) ([]string, error) {
	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil

}
