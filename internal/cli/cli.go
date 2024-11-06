package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"runtime/debug"

	"github.com/bufbuild/protoyaml-go"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/runner/commander"
)

var Commit = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "dev"
}()

func CommandSet() *commander.CommandSet {

	cmdGroup := commander.NewCommandSet()
	cmdGroup.Add("version", commander.NewCommand(runVersion))

	cmdGroup.Add("generate", commander.NewCommand(runGenerate))
	cmdGroup.Add("verify", commander.NewCommand(runVerify))
	cmdGroup.Add("publish", commander.NewCommand(runPublish))

	cmdGroup.Add("schema", schemaSet())
	cmdGroup.Add("protoc", protocSet())
	cmdGroup.Add("j5s", j5sSet())

	cmdGroup.Add("latest-deps", commander.NewCommand(runLatestDeps))

	cmdGroup.Add("lsp", commander.NewCommand(runLSP))

	return cmdGroup
}

func runVersion(ctx context.Context, cfg struct{}) error {
	fmt.Printf("jsonapi version %v\n", Commit)
	return nil
}

func runLatestDeps(ctx context.Context, cfg struct {
	SourceConfig
}) error {
	src, err := cfg.GetSource(ctx)
	if err != nil {
		return err
	}

	allDeps, err := src.ListAllDependencies()
	if err != nil {
		return err
	}

	resolver, err := source.NewEnvResolver()
	if err != nil {
		return err
	}

	newLockFile, err := resolver.LatestLocks(ctx, allDeps)
	if err != nil {
		return err
	}

	data, err := protoyaml.MarshalOptions{}.Marshal(newLockFile)
	if err != nil {
		return err
	}
	return cfg.WriteFile("j5-lock.yaml", data)
}

type SourceConfig struct {
	Source string `flag:"dir" default:"." description:"Source / working directory containing j5.yaml and buf.lock.yaml"`
	Bundle string `flag:"bundle" default:"" description:"When the bundle j5.yaml is in a subdirectory"`
}

func (cfg SourceConfig) WriteFile(filename string, data []byte) error {
	return os.WriteFile(filepath.Join(cfg.Source, filename), data, 0644)
}

func (cfg SourceConfig) GetSource(ctx context.Context) (*source.RepoRoot, error) {

	resolver, err := source.NewEnvResolver()
	if err != nil {
		return nil, err
	}

	fsRoot := os.DirFS(cfg.Source)
	return source.NewFSRepoRoot(ctx, fsRoot, resolver)
}

func (cfg SourceConfig) EachBundle(ctx context.Context, fn func(source.Bundle) error) error {
	src, err := cfg.GetSource(ctx)
	if err != nil {
		return err
	}

	if cfg.Bundle != "" {
		bundle, err := src.BundleSource(cfg.Bundle)
		if err != nil {
			return err
		}
		return fn(bundle)
	}

	for _, bundle := range src.AllBundles() {
		if err := fn(bundle); err != nil {
			return err
		}
	}
	return nil
}

func (cfg SourceConfig) FileWriterAt(ctx context.Context, prefix string) (*fileWriter, error) {
	return &fileWriter{
		dir: filepath.Join(cfg.Source, prefix),
	}, nil
}

func (cfg SourceConfig) GetBundleImage(ctx context.Context) (*source_j5pb.SourceImage, *config_j5pb.BundleConfigFile, error) {
	source, err := cfg.GetSource(ctx)
	if err != nil {
		return nil, nil, err
	}

	return source.BundleImageSource(ctx, cfg.Bundle)
}

type lineWriter struct {
	buf       []byte
	writeLine func(string)
}

func (w *lineWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if b == '\n' {
			w.writeLine(string(w.buf))
			w.buf = w.buf[:0]
		} else {
			w.buf = append(w.buf, b)
		}
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if len(w.buf) > 0 {
		w.writeLine(string(w.buf))
	}
	w.buf = []byte{}
}

type Dest interface {
	builder.Dest
	Sub(subPath string) Dest
}

type DiscardFS struct{}

func NewDiscardFS() *DiscardFS {
	return &DiscardFS{}
}

func (d *DiscardFS) Sub(subPath string) Dest {
	return d
}

func (d *DiscardFS) PutFile(ctx context.Context, subPath string, body io.Reader) error {
	return nil
}

type LocalFS struct {
	root string
}

func NewLocalFS(root string) (*LocalFS, error) {
	return &LocalFS{
		root: root,
	}, nil
}

func (local *LocalFS) Sub(subPath string) Dest {
	return &LocalFS{
		root: filepath.Join(local.root, subPath),
	}
}

func (local *LocalFS) Clean(paths []string) error {
	for _, path := range paths {
		err := os.RemoveAll(filepath.Join(local.root, path))
		if err != nil {
			return err
		}
	}
	return nil
}

func (local *LocalFS) PutFile(ctx context.Context, subPath string, body io.Reader) error {
	key := filepath.Join(local.root, subPath)
	err := os.MkdirAll(filepath.Dir(key), 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(key)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
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
