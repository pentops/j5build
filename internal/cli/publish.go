package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/builder"
)

func runPublish(ctx context.Context, cfg struct {
	SourceConfig
	Dest    string `flag:"dest" description:"Destination directory for published files"`
	Publish string `flag:"publish" optional:"true" description:"Name of the 'publish' to run (required when more than one exists)"`
}) error {

	img, inputConfig, err := cfg.GetBundleImage(ctx)
	if err != nil {
		return err
	}

	var publish *config_j5pb.PublishConfig
	if cfg.Publish == "" {
		if len(inputConfig.Publish) != 1 {
			return fmt.Errorf("no publish specified and %d publishes found", len(inputConfig.Publish))
		}
		publish = inputConfig.Publish[0]
	} else {
		for _, p := range inputConfig.Publish {
			if p.Name == cfg.Publish {
				publish = p
				break
			}
		}
		if publish == nil {
			return fmt.Errorf("no publish found with name %q", cfg.Publish)
		}
	}

	if err := builder.MutateImageWithMods(img, publish.Mods); err != nil {
		return fmt.Errorf("MutateImageWithMods: %w", err)
	}

	dockerWrapper, err := builder.NewRunner(builder.DefaultRegistryAuths)
	if err != nil {
		return err
	}
	bb := builder.NewBuilder(dockerWrapper)

	outRoot, err := NewLocalFS(cfg.Dest)
	if err != nil {
		return err
	}

	pc := builder.PluginContext{
		Variables: map[string]string{},
		Dest:      outRoot,
		ErrOut:    os.Stderr,
	}

	return bb.RunPublishBuild(ctx, pc, img, publish)
}
