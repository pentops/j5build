package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/lib/j5codec"
	"github.com/pentops/j5build/internal/export"
	"github.com/pentops/j5build/internal/j5client"
	"github.com/pentops/j5build/internal/structure"
	"github.com/pentops/runner/commander"
	"google.golang.org/protobuf/encoding/protojson"
)

func schemaSet() *commander.CommandSet {
	genGroup := commander.NewCommandSet()
	genGroup.Add("image", commander.NewCommand(RunImage))
	genGroup.Add("source", commander.NewCommand(RunSource))
	genGroup.Add("client", commander.NewCommand(RunClient))
	genGroup.Add("swagger", commander.NewCommand(RunSwagger))
	return genGroup
}

type BuildConfig struct {
	SourceConfig
	Output  string   `flag:"output" default:"-" description:"Destination to push image to. - for stdout, otherwise a file"`
	Package []string `flag:"package" default:"" description:"Filter output to listed packages"`
}

func (cfg BuildConfig) descriptorAPI(ctx context.Context) (*client_j5pb.API, error) {
	image, _, err := cfg.GetBundleImage(ctx)
	if err != nil {
		return nil, err
	}

	reflectionAPI, err := structure.APIFromImage(image)
	if err != nil {
		return nil, fmt.Errorf("ReflectFromSource: %w", err)
	}

	descriptorAPI, err := j5client.APIFromSource(reflectionAPI)
	if err != nil {
		return nil, fmt.Errorf("DescriptorFromReflection: %w", err)
	}

	if err := structure.ResolveProse(image, descriptorAPI); err != nil {
		return nil, fmt.Errorf("ResolveProse: %w", err)
	}

	return descriptorAPI, nil
}

func RunImage(ctx context.Context, cfg BuildConfig) error {
	image, _, err := cfg.GetBundleImage(ctx)
	if err != nil {
		return err
	}

	bb, err := protojson.Marshal(image)
	if err != nil {
		return err
	}
	return writeBytes(cfg.Output, bb)
}

func RunSource(ctx context.Context, cfg struct {
	BuildConfig
	Package []string `flag:"package" default:"" description:"Package to show"`
	Schema  string   `flag:"schema" default:"" description:"Schema to show"`
}) error {
	image, _, err := cfg.GetBundleImage(ctx)
	if err != nil {
		return err
	}

	sourceAPI, err := structure.APIFromImage(image)
	if err != nil {
		return err
	}
	out := sourceAPI.ProtoReflect()

	if len(cfg.Package) > 0 {
		sourceAPI.Packages, err = filterPackages(sourceAPI.Packages, cfg.Package)
		if err != nil {
			return err
		}
	}

	bb, err := j5codec.NewCodec().ProtoToJSON(out)
	if err != nil {
		return err
	}

	return writeBytes(cfg.Output, bb)
}

func RunClient(ctx context.Context, cfg BuildConfig) error {

	descriptorAPI, err := cfg.descriptorAPI(ctx)
	if err != nil {
		return err
	}

	if len(cfg.Package) > 0 {
		descriptorAPI.Packages, err = filterPackages(descriptorAPI.Packages, cfg.Package)
		if err != nil {
			return err
		}
	}

	bb, err := j5codec.NewCodec().ProtoToJSON(descriptorAPI.ProtoReflect())
	if err != nil {
		return err
	}

	return writeBytes(cfg.Output, bb)
}

type packageLike interface {
	GetName() string
}

func filterPackages[T packageLike](packages []T, filter []string) ([]T, error) {
	if len(filter) == 0 {
		return packages, nil
	}

	byName := map[string]T{}
	matching := []T{}

	for _, search := range packages {
		byName[search.GetName()] = search
	}

	for _, filter := range filter {
		search, ok := byName[filter]
		if !ok {
			for key := range byName {
				fmt.Printf("Have package %s\n", key)
			}
			return nil, fmt.Errorf("package %q not found in (%d) total", filter, len(byName))
		}

		matching = append(matching, search)
	}

	return matching, nil
}

func RunSwagger(ctx context.Context, cfg BuildConfig) error {
	descriptorAPI, err := cfg.descriptorAPI(ctx)
	if err != nil {
		return err
	}

	swaggerDoc, err := export.BuildSwagger(descriptorAPI)
	if err != nil {
		return err
	}

	asJson, err := json.Marshal(swaggerDoc)
	if err != nil {
		return err
	}

	return writeBytes(cfg.Output, asJson)

}

func writeBytes(to string, data []byte) error {
	if to == "-" {
		os.Stdout.Write(data)
		return nil
	}

	return os.WriteFile(to, data, 0644)
}
