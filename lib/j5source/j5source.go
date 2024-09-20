package j5source

import (
	"context"
	"io/fs"

	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/source"
	"google.golang.org/protobuf/types/descriptorpb"
)

type DependencySet interface {
	GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error)
	AllDependencyFiles() ([]*descriptorpb.FileDescriptorProto, []string)
	ListDependencyFiles(prefix string) []string
}

type Source struct {
	src *source.Source
}

func (w *Source) BundleConfig(name string) (*config_j5pb.BundleConfigFile, error) {
	return w.src.BundleConfig(name)
}

func (w *Source) BundleFS(name string) (fs.FS, error) {
	return w.src.BundleFS(name)
}

func (w *Source) BundleDependencies(ctx context.Context, name string) (DependencySet, error) {
	return w.src.BundleDependencies(ctx, name)
}

func NewFSSource(ctx context.Context, root fs.FS) (*Source, error) {
	resolver, err := source.NewEnvResolver()
	if err != nil {
		return nil, err
	}
	src, err := source.NewFSSource(ctx, root, resolver)
	if err != nil {
		return nil, err
	}
	return &Source{src: src}, nil
}
