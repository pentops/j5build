package builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/plugin/v1/plugin_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/j5build/internal/structure"
	"github.com/pentops/log.go/log"
	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func NewBuilder(runner PipeRunner) *Builder {
	return &Builder{
		runner: runner,
	}
}

type PluginContext struct {
	Variables map[string]string
	ErrOut    io.Writer
	Dest      Dest
}

type PipeRunner interface {
	Run(ctx context.Context, rc RunContext) error
}
type Builder struct {
	runner PipeRunner
}

type Dest interface {
	PutFile(ctx context.Context, path string, body io.Reader) error
}

func (b *Builder) RunGenerateBuild(ctx context.Context, pc PluginContext, input *source_j5pb.SourceImage, build *config_j5pb.GenerateConfig) error {
	return b.runPlugins(ctx, pc, input, build.Plugins)
}

func (b *Builder) RunPublishBuild(ctx context.Context, pc PluginContext, input *source_j5pb.SourceImage, build *config_j5pb.PublishConfig) error {
	err := b.runPlugins(ctx, pc, input, build.Plugins)
	if err != nil {
		return err
	}

	if build.OutputFormat != nil {
		switch pkg := build.OutputFormat.Type.(type) {
		case *config_j5pb.OutputType_GoProxy_:

			gomodFile, err := buildGomodFile(pkg.GoProxy)
			if err != nil {
				return err
			}

			err = pc.Dest.PutFile(ctx, "go.mod", bytes.NewReader(gomodFile))
			if err != nil {
				return err
			}
			return nil

		}
		// Fallthrough default, is OK to not specify
	}
	return nil
}

func buildGomodFile(pkg *config_j5pb.OutputType_GoProxy) ([]byte, error) {
	mm := modfile.File{}
	if err := mm.AddModuleStmt(pkg.Path); err != nil {
		return nil, err
	}

	if pkg.GoVersion == "" {
		pkg.GoVersion = "1.22.3"
	}

	if err := mm.AddGoStmt(pkg.GoVersion); err != nil {
		return nil, err
	}

	for _, dep := range pkg.Deps {
		if err := mm.AddRequire(dep.Path, dep.Version); err != nil {
			return nil, err
		}
	}

	return mm.Format()
}

func (b *Builder) runPlugins(ctx context.Context, pc PluginContext, input *source_j5pb.SourceImage, plugins []*config_j5pb.BuildPlugin) error {

	if len(plugins) == 0 {
		return fmt.Errorf("no plugins")
	}

	pluginType := plugins[0].Type

	switch pluginType {
	case config_j5pb.Plugin_PLUGIN_PROTO:
		protoBuildRequest, err := source.CodeGeneratorRequestFromImage(input)
		if err != nil {
			return fmt.Errorf("CodeGeneratorRequestFromImage: %w", err)
		}

		errGroup, ctx := errgroup.WithContext(ctx)
		for _, plugin := range plugins {
			plugin := plugin
			errGroup.Go(func() error {
				ctx = log.WithField(ctx, "plugin", plugin.Name)
				log.Info(ctx, "Running Plugin")
				if plugin.Type != config_j5pb.Plugin_PLUGIN_PROTO {
					return fmt.Errorf("plugin type mismatch: %s", plugin.Type)
				}
				if err := b.runProtocPlugin(ctx, pc, plugin, protoBuildRequest); err != nil {
					return fmt.Errorf("plugin %s: %w", plugin.Name, err)
				}
				return nil
			})
		}
		return errGroup.Wait()

	case config_j5pb.Plugin_J5_CLIENT:

		sourceAPI, err := structure.APIFromImage(input)
		if err != nil {
			return err
		}

		clientAPI, err := j5client.APIFromSource(sourceAPI)
		if err != nil {
			return err
		}

		if len(clientAPI.Packages) == 0 {
			return fmt.Errorf("no packages found")
		}

		errGroup, ctx := errgroup.WithContext(ctx)
		for _, plugin := range plugins {
			plugin := plugin
			errGroup.Go(func() error {
				ctx = log.WithField(ctx, "plugin", plugin.Name)
				log.Info(ctx, "Running Plugin")
				if plugin.Type != config_j5pb.Plugin_PLUGIN_J5_CLIENT {
					return fmt.Errorf("plugin type mismatch: %s", plugin.Type)
				}
				if err := b.runJ5ClientPlugin(ctx, pc, plugin, clientAPI); err != nil {
					return fmt.Errorf("plugin %s: %w", plugin.Name, err)
				}
				return nil
			})
		}
		return errGroup.Wait()

	default:
		return fmt.Errorf("unsupported plugin type: %s", pluginType)
	}

}

func (b *Builder) runProtocPlugin(ctx context.Context, pc PluginContext, plugin *config_j5pb.BuildPlugin, sourceProto *pluginpb.CodeGeneratorRequest) error {

	start := time.Now()

	ctx = log.WithField(ctx, "builder", plugin.GetName())

	if plugin.Local != nil {
		ctx = log.WithField(ctx, "local-cmd", plugin.Local.Cmd)
	} else {
		ctx = log.WithField(ctx, "docker-runner", plugin.Docker.Image)
	}

	log.Debug(ctx, "Running Protoc Plugin")

	parameters := make([]string, 0, len(plugin.Opts))
	for k, v := range plugin.Opts {
		parameters = append(parameters, fmt.Sprintf("%s=%s", k, v))
	}
	parameter := strings.Join(parameters, ",")
	sourceProto.Parameter = &parameter

	reqBytes, err := proto.Marshal(sourceProto)
	if err != nil {
		return err
	}

	outBuffer := &bytes.Buffer{}
	inBuffer := bytes.NewReader(reqBytes)

	if err := b.runner.Run(ctx, RunContext{
		Vars:    pc.Variables,
		StdIn:   inBuffer,
		StdOut:  outBuffer,
		StdErr:  pc.ErrOut,
		Command: plugin,
	}); err != nil {
		return err
	}

	resp := pluginpb.CodeGeneratorResponse{}
	if err := proto.Unmarshal(outBuffer.Bytes(), &resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("plugin error: %s", *resp.Error)
	}

	for _, f := range resp.File {
		name := f.GetName()
		reader := bytes.NewReader([]byte(f.GetContent()))
		if err := pc.Dest.PutFile(ctx, name, reader); err != nil {
			return err
		}
	}

	log.WithFields(ctx, map[string]interface{}{
		"files":           len(resp.File),
		"durationSeconds": time.Since(start).Seconds(),
	}).Info("Protoc Plugin Complete")

	return nil
}

func (b *Builder) runJ5ClientPlugin(ctx context.Context, pc PluginContext, plugin *config_j5pb.BuildPlugin, descriptorAPI *client_j5pb.API) error {

	start := time.Now()

	buildRequest := &plugin_j5pb.CodeGenerationRequest{
		Packages: descriptorAPI.Packages,
		Options:  map[string]string{},
	}
	for key, opt := range plugin.Opts {
		buildRequest.Options[key] = opt
	}

	reqBytes, err := proto.Marshal(buildRequest)
	if err != nil {
		return err
	}

	outBuffer := &bytes.Buffer{}
	inBuffer := bytes.NewReader(reqBytes)

	if err := b.runner.Run(ctx, RunContext{
		Vars:    pc.Variables,
		StdIn:   inBuffer,
		StdOut:  outBuffer,
		StdErr:  pc.ErrOut,
		Command: plugin,
	}); err != nil {
		return err
	}

	resp := &plugin_j5pb.CodeGenerationResponse{}
	if err := proto.Unmarshal(outBuffer.Bytes(), resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("plugin error: %s", *resp.Error)
	}

	for _, f := range resp.Files {
		name := f.GetName()
		reader := bytes.NewReader([]byte(f.GetContent()))
		if err := pc.Dest.PutFile(ctx, name, reader); err != nil {
			return err
		}
	}

	log.WithFields(ctx, map[string]interface{}{
		"files":           len(resp.Files),
		"durationSeconds": time.Since(start).Seconds(),
	}).Info("Protoc Plugin Complete")

	return nil

}
