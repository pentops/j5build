package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/j5build/internal/structure"
	"google.golang.org/protobuf/proto"
)

func runVerify(ctx context.Context, cfg struct {
	SourceConfig
}) error {

	src, err := cfg.GetSource(ctx)
	if err != nil {
		return err
	}

	dockerWrapper, err := builder.NewRunner(builder.DefaultRegistryAuths)
	if err != nil {
		return err
	}
	bb := builder.NewBuilder(dockerWrapper)

	for _, bundle := range src.AllBundles() {

		err := func(bundle source.Bundle) error {

			img, err := bundle.SourceImage(ctx, src)
			if err != nil {
				return err
			}

			bundleConfig, err := bundle.J5Config()
			if err != nil {
				return err
			}

			sourceAPI, err := structure.APIFromImage(img)
			if err != nil {
				return fmt.Errorf("Source API From Image: %w", err)
			}

			clientAPI, err := j5client.APIFromSource(sourceAPI)
			if err != nil {
				return fmt.Errorf("Client API From Source: %w", err)
			}

			if err := structure.ResolveProse(img, clientAPI); err != nil {
				return fmt.Errorf("ResolveProse: %w", err)
			}

			_, err = j5schema.PackageSetFromSourceAPI(sourceAPI.Packages)
			if err != nil {
				return fmt.Errorf("building reflection from descriptor: %w", err)
			}

			for _, publish := range bundleConfig.Publish {
				img := img
				if len(bundleConfig.Publish) > 1 {
					img = proto.Clone(img).(*source_j5pb.SourceImage)
				}
				if err := builder.MutateImageWithMods(img, publish.Mods); err != nil {
					return fmt.Errorf("MutateImageWithMods: %w", err)
				}
				if err := bb.RunPublishBuild(ctx, builder.PluginContext{
					Variables: map[string]string{},
					ErrOut:    os.Stderr,
					Dest:      NewDiscardFS(),
				}, img, publish); err != nil {
					return err
				}
			}
			return nil
		}(bundle)

		if err != nil {
			return fmt.Errorf("bundle %s: %w", bundle.DebugName(), err)
		}

	}

	outRoot := NewDiscardFS()

	j5Config := src.RepoConfig()
	for _, generator := range j5Config.Generate {
		if err := runGeneratePlugin(ctx, bb, src, generator, outRoot); err != nil {
			return err

		}
	}
	return nil
}
