package source

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
)

type Bundle interface {
	DebugName() string
	J5Config() (*config_j5pb.BundleConfigFile, error)
	SourceImage(ctx context.Context, resolver InputSource) (*source_j5pb.SourceImage, error)
	DirInRepo() string
	FS() fs.FS
	GetDependencies(ctx context.Context, resolver InputSource) (DependencySet, error)
}

type bundleSource struct {
	debugName string
	fs        fs.FS
	refConfig *config_j5pb.BundleReference
	config    *config_j5pb.BundleConfigFile
	dirInRepo string
}

func (bs bundleSource) DirInRepo() string {
	return bs.dirInRepo
}

func (b bundleSource) FS() fs.FS {
	return b.fs
}

func (b bundleSource) DebugName() string {
	return b.debugName
}

func (b *bundleSource) J5Config() (*config_j5pb.BundleConfigFile, error) {
	return b.config, nil
}

func (b *bundleSource) SourceImage(ctx context.Context, resolver InputSource) (*source_j5pb.SourceImage, error) {
	img, err := b.readImageFromDir(ctx, resolver)
	if err != nil {
		return nil, fmt.Errorf("reading source image for %s: %w", b.debugName, err)
	}

	if img.SourceName == "" {
		img.SourceName = b.debugName
	}
	return img, nil
}

func (bundle *bundleSource) GetDependencies(ctx context.Context, resolver InputSource) (DependencySet, error) {
	j5Config, err := bundle.J5Config()
	if err != nil {
		return nil, err
	}
	dependencies := make([]*source_j5pb.SourceImage, 0, len(j5Config.Dependencies))
	for _, dep := range j5Config.Dependencies {
		img, err := resolver.GetSourceImage(ctx, dep)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, img)
	}
	for _, dep := range j5Config.Includes {
		img, err := resolver.GetSourceImage(ctx, dep.Input)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, img)
	}
	return combineSourceImages(dependencies)

}
func (bundle *bundleSource) getDependencies(ctx context.Context, resolver InputSource) ([]*source_j5pb.SourceImage, error) {
	j5Config, err := bundle.J5Config()
	if err != nil {
		return nil, err
	}
	dependencies := make([]*source_j5pb.SourceImage, len(j5Config.Dependencies))
	for idx, dep := range j5Config.Dependencies {
		img, err := resolver.GetSourceImage(ctx, dep)
		if err != nil {
			return nil, err
		}
		dependencies[idx] = img
	}
	return dependencies, nil
}

// getIncludes returns the images corresponding to the inputs. The returned
// slice will have the same indexes as the input.
func (bundle *bundleSource) getIncludes(ctx context.Context, resolver InputSource) ([]*source_j5pb.SourceImage, error) {
	j5Config, err := bundle.J5Config()
	if err != nil {
		return nil, err
	}
	dependencies := make([]*source_j5pb.SourceImage, len(j5Config.Includes))
	for idx, spec := range j5Config.Includes {
		img, err := resolver.GetSourceImage(ctx, spec.Input)
		if err != nil {
			return nil, err
		}

		dependencies[idx] = img
	}

	return dependencies, nil
}

func (bundle *bundleSource) readImageFromDir(ctx context.Context, resolver InputSource) (*source_j5pb.SourceImage, error) {

	dependencyImages, err := bundle.getDependencies(ctx, resolver)
	if err != nil {
		return nil, err
	}

	includeImages, err := bundle.getIncludes(ctx, resolver)
	if err != nil {
		return nil, err
	}

	combinedDeps, err := combineSourceImages(append(dependencyImages, includeImages...))
	if err != nil {
		return nil, err
	}

	includedFilenames := make([]string, 0)
	for _, included := range includeImages {
		includedFilenames = append(includedFilenames, included.SourceFilenames...)
	}

	img, err := readImageFromDir(ctx, bundle.fs, includedFilenames, combinedDeps)
	if err != nil {
		return nil, err
	}

	img.Packages = make([]*source_j5pb.PackageInfo, len(bundle.config.Packages))
	for idx, pkg := range bundle.config.Packages {
		img.Packages[idx] = &source_j5pb.PackageInfo{
			Name:  pkg.Name,
			Prose: pkg.Prose,
			Label: pkg.Label,
		}
	}

	for _, included := range includeImages {
		img.Prose = append(img.Prose, included.Prose...)
		img.Packages = append(img.Packages, included.Packages...)
	}
	return img, nil
}
