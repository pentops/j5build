package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/bcl.go/internal/linter"
	"github.com/pentops/bcl.go/internal/protobuild"
	"github.com/pentops/prototools/protoprint"
	"github.com/pentops/runner/commander"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/pentops/bcl.go/internal/lsp"
	"github.com/sourcegraph/jsonrpc2"
)

var Version = "dev"

func main() {
	cmdGroup := commander.NewCommandSet()
	cmdGroup.Add("j5gen", commander.NewCommand(runJ5Gen))
	cmdGroup.Add("lint", commander.NewCommand(runLint))
	cmdGroup.Add("fix", commander.NewCommand(runFix))
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

func runFix(ctx context.Context, cfg struct {
}) error {
	return fmt.Errorf("not implemented")
}

func runJ5Gen(ctx context.Context, cfg struct {
	Dir     string `flag:"dir" default:"." desc:"Root schema directory"`
	Verbose bool   `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
}) error {

	parser, err := j5parse.NewParser()
	if err != nil {
		return err
	}

	parser.Verbose = cfg.Verbose
	parser.FailFast = !cfg.Verbose

	desc := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{},
	}

	doFile := func(filename, data string) error {
		fmt.Printf("==========\n\nBEGIN %s\n", filename)

		file, err := parser.ParseFile(filename, data)
		if err != nil {
			return err
		}

		/*
			jsonData, err := codec.NewCodec().ProtoToJSON(file.ProtoReflect())
			if err != nil {
				return err
			}
			buf := &bytes.Buffer{}
			if err := json.Indent(buf, jsonData, "", "  "); err != nil {
				return err
			}
			fmt.Println(buf.String())

		*/
		protoDesc, err := protobuild.BuildFile(file)
		if err != nil {
			return err
		}

		//fmt.Println(protojson.Format(protoDesc))

		desc.File = append(desc.File, protoDesc)

		return nil
	}

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

		if err := doFile(pathname, string(data)); err != nil {
			return err
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
	if err := protoprint.PrintProtoFiles(ctx, &fileWriter{dir: cfg.Dir}, desc, protoprint.Options{}); err != nil {
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
