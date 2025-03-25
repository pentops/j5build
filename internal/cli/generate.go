package cli

import (
	"context"
	"fmt"

	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/builder"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/log.go/log"
)

func runGenerate(ctx context.Context, cfg struct {
	SourceConfig
	NoClean bool `flag:"no-clean" description:"Do not remove the directories in config as 'managedPaths' before generating"`
	NoJ5s   bool `flag:"no-j5s" description:"Do not convert J5s source files to proto"`
}) error {

	if !cfg.NoJ5s {
		if err := runJ5sGenProto(ctx, j5sGenProtoConfig{
			SourceConfig: cfg.SourceConfig,
		}); err != nil {
			return err
		}
	}

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

	if !cfg.NoClean {
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

func runGeneratePlugin(ctx context.Context, bb *builder.Builder, src *source.RepoRoot, generator *config_j5pb.GenerateConfig, out Dest) error {

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

/*
type Callback struct {
	ObjectProperty func([]string, *schema_j5pb.ObjectProperty)
	Object         func([]string, *schema_j5pb.Object)
	Enum           func([]string, *schema_j5pb.Enum)
	Oneof          func([]string, *schema_j5pb.Oneof)
}

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
