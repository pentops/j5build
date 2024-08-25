package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/bcl.go/internal/protobuild"
	"github.com/pentops/j5/codec"
	"github.com/pentops/prototools/protoprint"
	"github.com/pentops/runner/commander"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/descriptorpb"
)

var Version = "dev"

func main() {
	cmdGroup := commander.NewCommandSet()
	cmdGroup.Add("j5gen", commander.NewCommand(runJ5Gen))
	cmdGroup.RunMain("bcl", Version)
}

func runJ5Gen(ctx context.Context, cfg struct {
	Dir string `flag:"dir" default:"." desc:"Root schema directory"`
}) error {

	parser := j5parse.NewParser()

	desc := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{},
	}

	doFile := func(filename, data string) error {
		fmt.Printf("==========\n\nBEGIN %s\n", filename)

		file, err := parser.ParseFile(filename, data)
		if err != nil {
			return err
		}

		jsonData, err := codec.NewCodec().ProtoToJSON(file.ProtoReflect())
		if err != nil {
			return err
		}
		buf := &bytes.Buffer{}
		if err := json.Indent(buf, jsonData, "", "  "); err != nil {
			return err
		}
		fmt.Println(buf.String())

		protoDesc, err := protobuild.BuildFile(file)
		if err != nil {
			return err
		}

		fmt.Println(protojson.Format(protoDesc))

		desc.File = append(desc.File, protoDesc)

		return nil
	}

	root := os.DirFS(cfg.Dir)
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
