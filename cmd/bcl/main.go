package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/bcl.go/internal/linter"
	"github.com/pentops/bcl.go/internal/protobuild"
	"github.com/pentops/prototools/protoprint"
	"github.com/pentops/runner/commander"

	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/pentops/j5/lib/j5source"
	"github.com/sourcegraph/jsonrpc2"
)

var Version = "dev"

func main() {
	cmdGroup := commander.NewCommandSet()
	cmdGroup.Add("j5gen", commander.NewCommand(runJ5Gen))
	cmdGroup.Add("lint", commander.NewCommand(runLint))
	cmdGroup.Add("fmt", commander.NewCommand(runFmt))
	cmdGroup.Add("lsp", commander.NewCommand(runLSP))
	cmdGroup.RunMain("bcl", Version)
}

func runLSP(ctx context.Context, cfg struct {
	ProjectRoot string `flag:"project-root" default:"" desc:"Project root directory"`
}) error {

	config := lsp.LSPConfig{
		ProjectRoot: cfg.ProjectRoot,
	}

	if config.ProjectRoot == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		config.ProjectRoot = pwd
	}
	log.Printf("ProjectRoot: %v", config.ProjectRoot)

	logFile := "$HOME/.bcl/log.txt"
	logFile = os.ExpandEnv(logFile)
	if err := os.MkdirAll(path.Dir(logFile), 0755); err != nil {
		return err
	}
	stream, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	parser, err := j5parse.NewParser()
	if err != nil {
		return err
	}
	bclLinter := linter.New(parser)

	handlers := lsp.LSPHandlers{
		Linter: bclLinter,
	}

	defer stream.Close()
	log.Default().SetOutput(stream)
	log.Printf("BEGIN")
	conn := jsonrpc2.NewConn(
		ctx,
		jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
		lsp.NewHandler(config, handlers),
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

func runLint(ctx context.Context, cfg struct {
	ProjectRoot string `flag:"project-root" default:"" desc:"Project root directory"`
}) error {
	parser, err := j5parse.NewParser()
	if err != nil {
		return err
	}
	bclLinter := linter.New(parser)
	_ = bclLinter

	if cfg.ProjectRoot == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cfg.ProjectRoot = pwd
	}

	log.Printf("ProjectRoot: %v", cfg.ProjectRoot)

	os.Exit(100)
	return nil
}

func runFmt(ctx context.Context, cfg struct {
	Dir   string `flag:"dir" default:"." desc:"Root schema directory"`
	Write bool   `flag:"write" default:"false" desc:"Write fixes to files"`
}) error {

	doFile := func(pathname string, data []byte) (string, error) {
		tree, err := ast.ParseFile(string(data), true)
		if err != nil {
			if err == ast.HadErrors {
				return "", errpos.AddSource(tree.Errors, string(data))
			}
			return "", fmt.Errorf("parse file not HadErrors: %w", err)
		}

		fixed := ast.Print(tree)
		return fixed, nil
	}

	stat, err := os.Lstat(cfg.Dir)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		data, err := os.ReadFile(cfg.Dir)
		if err != nil {
			return err
		}
		out, err := doFile(cfg.Dir, data)
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", cfg.Dir)
			fmt.Println(out)
		} else {
			return os.WriteFile(cfg.Dir, []byte(out), 0644)
		}
		return nil
	}

	outWriter := &fileWriter{dir: cfg.Dir}
	root := os.DirFS(cfg.Dir)
	err = fs.WalkDir(root, ".", func(pathname string, d fs.DirEntry, err error) error {
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

		out, err := doFile(pathname, data)
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", pathname)
			fmt.Println(out)
			return nil
		} else {
			return outWriter.PutFile(ctx, pathname+".fied", []byte(out))
		}
	})
	if err != nil {
		return err
	}
	return nil
}

func runJ5Gen(ctx context.Context, cfg struct {
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

	j5Files, err := localFiles.ListJ5Files(ctx)
	if err != nil {
		return err
	}

	for _, filename := range j5Files {
		log.Printf("pre-compile %s\n", filename)
		_, err := resolver.ParseToDescriptor(ctx, filename)
		if err != nil {
			debug, ok := errpos.AsErrorsWithSource(err)
			if !ok {
				return fmt.Errorf("unlinked error: %w", err)
			}

			str := debug.HumanString(3)
			fmt.Println(str)
			os.Exit(1)
		}
	}

	log.Printf("Pre-Compiled %d files", len(j5Files))

	for _, pkg := range packages {
		pkgSrc, err := localFiles.ListSourceFiles(ctx, pkg)
		if err != nil {
			return fmt.Errorf("listing package %s: %w", pkg, err)
		}

		for _, filename := range pkgSrc {
			if !strings.HasSuffix(filename, ".j5s") {
				continue
			}

			log.Printf("Compiling %s", filename)

			filename := strings.TrimSuffix(filename, ".j5s") + ".j5gen.proto"

			built, err := resolver.Compile(ctx, filename)
			if err != nil {
				log.Printf("Error compiling %s: %v", filename, err)
				return err
			}
			fileOut := built[0]

			out, err := protoprint.PrintFile(ctx, fileOut)
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
