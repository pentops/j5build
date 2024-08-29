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
	"google.golang.org/protobuf/encoding/prototext"

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
	Dir           string   `flag:"dir" default:"." desc:"Root schema directory"`
	Bundle        string   `flag:"bundle" default:"" desc:"Bundle file"`
	Verbose       bool     `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
	Files         []string `flag:"files" required:"false" desc:"Files to generate, otherwise all J5S"`
	DebugProtoAST bool     `flag:"debug-proto-ast" default:"false" desc:"Print proto AST to output dir"`
}) error {

	outWriter := &fileWriter{dir: cfg.Dir}
	j5Parser, err := j5parse.NewParser()
	if err != nil {
		return err
	}

	protoParser := protobuild.NewProtoParser()

	source, err := j5source.NewFSSource(ctx, os.DirFS(cfg.Dir))
	if err != nil {
		return err
	}

	deps, err := source.BundleDependencies(ctx, cfg.Bundle)
	if err != nil {
		return err
	}

	link := protobuild.New(deps)

	j5Parser.Verbose = cfg.Verbose
	j5Parser.FailFast = !cfg.Verbose

	root := os.DirFS(cfg.Dir)
	err = fs.WalkDir(root, ".", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		switch path.Ext(pathname) {
		case ".proto":
			if strings.HasSuffix(pathname, ".gen.proto") {
				return nil
			}
			data, err := fs.ReadFile(root, pathname)
			if err != nil {
				return err
			}
			file, err := protoParser.ParseFile(pathname, data)
			if err != nil {
				return err
			}

			return link.AddProtoFile(file)

		case ".j5s":
			data, err := fs.ReadFile(root, pathname)
			if err != nil {
				return err
			}

			file, err := j5Parser.ParseFile(pathname, string(data))
			if err != nil {
				return err
			}

			if cfg.DebugProtoAST {
				if err := outWriter.PutFile(ctx, pathname+".ast", []byte(prototext.Format(file))); err != nil {
					return err
				}
			}

			return link.AddJ5File(file)
		}

		return nil
	})

	if err != nil {
		debug, ok := errpos.AsErrorsWithSource(err)
		if !ok {
			return fmt.Errorf("unlinked error: %w", err)
		}

		str := debug.HumanString(3)
		fmt.Println(str)
		os.Exit(1)
	}

	if err := link.ConvertJ5(); err != nil {
		return err
	}

	descriptors, err := link.BuildDescriptors(ctx, cfg.Files)
	if err != nil {
		return err
	}

	if err := protoprint.PrintReflect(ctx, outWriter, descriptors, protoprint.Options{}); err != nil {
		return err
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
