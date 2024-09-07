package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/structure"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/log.go/log"
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
		for _, pkg := range sourceAPI.Packages {
			fmt.Printf("Package %s OK\n", pkg.Name)
		}

		for _, publish := range bundleConfig.Publish {
			if err := bb.RunPublishBuild(ctx, builder.PluginContext{
				Variables: map[string]string{},
				ErrOut:    os.Stderr,
				Dest:      NewDiscardFS(),
			}, img, publish); err != nil {
				return err
			}
		}
	}

	outRoot := NewDiscardFS()

	j5Config := src.RepoConfig()
	for _, generator := range j5Config.Generate {
		img, err := src.CombinedSourceImage(ctx, generator.Inputs)
		if err != nil {
			return err
		}
		if err := runGeneratePlugin(ctx, bb, img, generator, outRoot); err != nil {
			return err

		}
	}
	return nil
}

func runGenerate(ctx context.Context, cfg struct {
	SourceConfig
	Clean bool `flag:"clean" description:"Remove the directories in config as 'managedPaths' before generating"`
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

	outRoot, err := NewLocalFS(cfg.Source)
	if err != nil {
		return err
	}

	if cfg.Clean {
		repoConfig := src.RepoConfig()
		if err := outRoot.Clean(repoConfig.ManagedPaths); err != nil {
			return err
		}
	}

	j5Config := src.RepoConfig()
	for _, generator := range j5Config.Generate {
		img, err := src.CombinedSourceImage(ctx, generator.Inputs)
		if err != nil {
			return err
		}
		if err := runGeneratePlugin(ctx, bb, img, generator, outRoot); err != nil {
			return err

		}
	}
	return nil
}

func runGeneratePlugin(ctx context.Context, bb *builder.Builder, img *source_j5pb.SourceImage, generator *config_j5pb.GenerateConfig, out Dest) error {

	errOut := &lineWriter{
		writeLine: func(line string) {
			log.WithField(ctx, "generator", generator.Name).Info(line)
		},
	}

	dest := out.Sub(generator.Output)

	pc := builder.PluginContext{
		Variables: map[string]string{},
		Dest:      dest,
		ErrOut:    errOut,
	}

	err := bb.RunGenerateBuild(ctx, pc, img, generator)
	errOut.flush()
	if err != nil {
		return err
	}

	return nil
}

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
