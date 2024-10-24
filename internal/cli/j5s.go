package cli

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/j5parse"
	"github.com/pentops/j5build/internal/bcl/protobuild"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/prototools/protoprint"
	"github.com/pentops/runner/commander"
)

func j5sSet() *commander.CommandSet {
	genGroup := commander.NewCommandSet()
	genGroup.Add("fmt", commander.NewCommand(runJ5sFmt))
	genGroup.Add("lint", commander.NewCommand(runJ5sLint))
	genGroup.Add("genproto", commander.NewCommand(runJ5sGenProto))
	return genGroup
}

func runJ5sLint(ctx context.Context, cfg struct {
	Dir  string `flag:"dir" required:"false" description:"Source / working directory containing j5.yaml and buf.lock.yaml"`
	File string `flag:"file" required:"false" description:"Single file to format"`
}) error {

	parser, err := bcl.NewParser(j5parse.J5SchemaSpec)
	if err != nil {
		return err
	}

	allOK := true
	doFile := func(ctx context.Context, pathname string, data []byte) error {
		_, mainError := parser.ParseFile(pathname, string(data), j5parse.FileStub(pathname))
		if mainError == nil {
			return nil
		}

		locErr, ok := errpos.AsErrorsWithSource(mainError)
		if !ok {
			return mainError
		}

		log.Println(locErr.HumanString(2))

		allOK = false
		return nil
	}

	if cfg.File != "" {
		if cfg.Dir != "" {
			return fmt.Errorf("Cannot specify both dir and file")
		}
		data, err := os.ReadFile(cfg.File)
		if err != nil {
			return err
		}
		err = doFile(ctx, cfg.File, data)
		if err != nil {
			return err
		}
		return nil
	}

	err = runForJ5Files(ctx, os.DirFS(cfg.Dir), doFile)
	if err != nil {
		return err
	}
	if allOK {
		return nil
	}

	return fmt.Errorf("Linting failed")
}

func runJ5sFmt(ctx context.Context, cfg struct {
	Dir   string `flag:"dir" required:"false" description:"Source / working directory containing j5.yaml and buf.lock.yaml"`
	File  string `flag:"file" required:"false" description:"Single file to format"`
	Write bool   `flag:"write" default:"false" desc:"Write fixes to files"`
}) error {

	var outWriter *fileWriter

	doFile := func(ctx context.Context, pathname string, data []byte) error {
		fixed, err := bcl.Fmt(string(data))
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", pathname)
			fmt.Println(fixed)
			return nil
		} else {
			return outWriter.PutFile(ctx, pathname, []byte(fixed))
		}
	}

	if cfg.File != "" {
		if cfg.Dir != "" {
			return fmt.Errorf("Cannot specify both dir and file")
		}
		dir, pathname := path.Split(cfg.File)
		outWriter = &fileWriter{dir: dir}

		data, err := os.ReadFile(cfg.File)
		if err != nil {
			return err
		}
		err = doFile(ctx, pathname, data)
		if err != nil {
			return err
		}
		return nil
	}

	return runForJ5Files(ctx, os.DirFS(cfg.Dir), doFile)
}

func runForJ5Files(ctx context.Context, root fs.FS, doFile func(ctx context.Context, pathname string, data []byte) error) error {
	err := fs.WalkDir(root, ".", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if path.Ext(pathname) != ".j5s" {
			return nil
		}

		data, err := fs.ReadFile(root, pathname)
		if err != nil {
			return err
		}

		return doFile(ctx, pathname, data)
	})
	if err != nil {
		return err
	}
	return nil
}

func runJ5sGenProto(ctx context.Context, cfg struct {
	SourceConfig
	Verbose bool `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
}) error {
	src, err := cfg.GetSource(ctx)
	if err != nil {
		return err
	}

	err = cfg.EachBundle(ctx, func(bundle source.Bundle) error {

		bundleDir := bundle.DirInRepo()

		bundleConfig, err := bundle.J5Config()
		if err != nil {
			return err
		}

		bundleFS := bundle.FS()

		packages := []string{}
		for _, pkg := range bundleConfig.Packages {
			packages = append(packages, pkg.Name)
		}

		localFiles := &fileReader{
			fs:       bundleFS,
			fsName:   bundleDir,
			packages: packages,
		}

		deps, err := bundle.GetDependencies(ctx, src)
		if err != nil {
			return err
		}

		resolver, err := protobuild.NewResolver(deps, localFiles)
		if err != nil {
			return err
		}

		outWriter, err := cfg.FileWriterAt(ctx, bundle.DirInRepo())
		if err != nil {
			return err
		}

		compiler := protobuild.NewCompiler(resolver)

		for _, pkg := range packages {
			out, err := compiler.CompilePackage(ctx, pkg)
			if err != nil {
				return fmt.Errorf("compile package: %w", err)
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
	})

	if err == nil {
		return nil
	}

	e, ok := errpos.AsErrorsWithSource(err)
	if !ok {
		return err
	}
	fmt.Fprintln(os.Stderr, e.HumanString(3))

	return err
}

type fileReader struct {
	fs       fs.FS
	fsName   string
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
		if strings.HasSuffix(path, ".j5s.proto") {
			return nil
		}
		if strings.HasSuffix(path, ".proto") || strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", rr.fsName, err)
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
