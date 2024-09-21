package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5/lib/j5schema"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/j5build/internal/structure"
	"github.com/pentops/log.go/log"
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
			return fmt.Errorf("bundle %s: %w", bundle.Name(), err)
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
		if err := runGeneratePlugin(ctx, bb, src, generator, outRoot); err != nil {
			return err

		}
	}
	return nil
}

func runGeneratePlugin(ctx context.Context, bb *builder.Builder, src *source.Source, generator *config_j5pb.GenerateConfig, out Dest) error {

	img, err := src.CombinedSourceImage(ctx, generator.Inputs)
	if err != nil {
		return err
	}

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

	if err := builder.MutateImageWithMods(img, generator.Mods); err != nil {
		return fmt.Errorf("MutateImageWithMods: %w", err)
	}

	err = bb.RunGenerateBuild(ctx, pc, img, generator)
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

type Callback struct {
	ObjectProperty func([]string, *schema_j5pb.ObjectProperty)
	Object         func([]string, *schema_j5pb.Object)
	Enum           func([]string, *schema_j5pb.Enum)
	Oneof          func([]string, *schema_j5pb.Oneof)
}

/*
type vetWalker struct {
	callback Callback
}

func walkAPI(api *source_j5pb.API, callback *Callback) {
	path := []string{}
	ww := vetWalker{
		callback: *callback,
	}
	for _, pkg := range api.Packages {
		ww.vetPackage(pkg, append(path, pkg.Name))
	}
}

func (ww *vetWalker) vetPackage(pkg *source_j5pb.Package, path []string) {
	for _, sch := range pkg.Schemas {
		switch st := sch.Type.(type) {
		case *schema_j5pb.RootSchema_Object:
			ww.vetObject(st.Object, append(path, st.Object.Name))
		case *schema_j5pb.RootSchema_Enum:
			ww.vetEnum(st.Enum, append(path, st.Enum.Name))
		case *schema_j5pb.RootSchema_Oneof:
			ww.vetOneof(st.Oneof, append(path, st.Oneof.Name))
		}
	}
}

func (ww *vetWalker) vetObject(obj *schema_j5pb.Object, path []string) {
	if ww.callback.Object != nil {
		ww.callback.Object(path, obj)
	}
	for _, prop := range obj.Properties {
		ww.vetObjectProperty(prop, append(path, prop.Name))
	}
}

func (ww *vetWalker) vetEnum(en *schema_j5pb.Enum, path []string) {
	if ww.callback.Enum != nil {
		ww.callback.Enum(path, en)
	}
}

func (ww *vetWalker) vetOneof(of *schema_j5pb.Oneof, path []string) {
	if ww.callback.Oneof != nil {
		ww.callback.Oneof(path, of)
	}
	for _, prop := range of.Properties {
		ww.vetObjectProperty(prop, append(path, prop.Name))
	}
}

func (ww *vetWalker) vetObjectProperty(prop *schema_j5pb.ObjectProperty, path []string) {
	if ww.callback.ObjectProperty != nil {
		ww.callback.ObjectProperty(path, prop)
	}
	switch st := prop.Schema.Type.(type) {
	case *schema_j5pb.Field_Object:
		if obj := st.Object.GetObject(); obj != nil {
			ww.vetObject(obj, append(path, "schema", "object", "object"))
		}
	case *schema_j5pb.Field_Enum:
		if en := st.Enum.GetEnum(); en != nil {
			ww.vetEnum(en, append(path, "schema", "enum", "enum"))
		}
	case *schema_j5pb.Field_Oneof:
		if of := st.Oneof.GetOneof(); of != nil {
			ww.vetOneof(of, append(path, "schema", "oneof", "oneof"))
		}
	}
}*/
