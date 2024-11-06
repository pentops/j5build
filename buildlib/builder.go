package buildlib

import (
	"context"
	"io"
	"io/fs"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/source"
)

type RegistryClient interface {
	GetImage(ctx context.Context, owner, repoName, version string) (*source_j5pb.SourceImage, error)
	LatestImage(ctx context.Context, owner, repoName string, reference *string) (*source_j5pb.SourceImage, error)
}

func NewBuilder(regClient RegistryClient) (*Builder, error) {

	resolver, err := source.NewResolver(regClient)
	if err != nil {
		return nil, err
	}

	dockerWrapper, err := builder.NewRunner(builder.DefaultRegistryAuths)
	if err != nil {
		return nil, err
	}

	impl := builder.NewBuilder(dockerWrapper)

	return &Builder{
		impl:     impl,
		resolver: resolver,
	}, nil
}

type Builder struct {
	impl     *builder.Builder
	resolver source.RemoteResolver
}

func (b *Builder) RunPublishBuild(ctx context.Context, pc PluginContext, input *source_j5pb.SourceImage, build *config_j5pb.PublishConfig) error {
	return b.impl.RunPublishBuild(ctx, pc.toBuilder(), input, build)
}

func (b *Builder) RunGenerateBuild(ctx context.Context, pc PluginContext, input *source_j5pb.SourceImage, build *config_j5pb.GenerateConfig) error {
	return b.impl.RunGenerateBuild(ctx, pc.toBuilder(), input, build)
}

func (b *Builder) MutateImageWithMods(img *source_j5pb.SourceImage, mods []*config_j5pb.ImageMod) error {
	return builder.MutateImageWithMods(img, mods)
}

func (b *Builder) SourceImage(ctx context.Context, fs fs.FS, bundleName string) (*source_j5pb.SourceImage, *config_j5pb.BundleConfigFile, error) {
	src, err := source.NewFSRepoRoot(ctx, fs, b.resolver)
	if err != nil {
		return nil, nil, err
	}
	return src.BundleImageSource(ctx, bundleName)
}

type PluginContext struct {
	Variables map[string]string
	ErrOut    io.Writer
	Dest      Dest
}

func (pc PluginContext) toBuilder() builder.PluginContext {
	return builder.PluginContext{
		Variables: pc.Variables,
		ErrOut:    pc.ErrOut,
		Dest:      pc.Dest,
	}
}

type Dest interface {
	PutFile(ctx context.Context, path string, body io.Reader) error
}
