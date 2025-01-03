package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
)

// A repo has:
// - Config, with:
//   - Generate
//   - Plugins
// - Bundles, with:
//   - Dependencies
//   - Publish
// - Lock File

// Registry codebase uses builder.RunPublishBuild
// Builder's two build methods take:
//   - SourceImage.
//   - GenerateConfig or PublishConfig
//     Both of which take a list of plugins.
//     Publish also contains output format / destination.

// Build:
// - Fetch Repo
// - Parse Config
// - Identify one or more Bundles as input
// - Get a SourceImage for each bundle
//   - Resolve Dependencies
//   - Proto Parse -> Source Image
// - Merge SourceImages
// - Run Plugins
// - (Format Output)
// - Write / Upload Output

// We need:
// A set of bundles, accessing the code and config
// A dependency resolver to download the SourceImage for dependencies
// Config manager for the plugins

// RemoteResolver fetches, locks and caches dependencies from buf and j5
type RemoteResolver interface {
	GetRemoteDependency(ctx context.Context, input *config_j5pb.Input, locks *config_j5pb.LockFile) (*source_j5pb.SourceImage, error)
	LatestLocks(ctx context.Context, deps []*config_j5pb.Input) (*config_j5pb.LockFile, error)
}

type InputSource interface {
	GetSourceImage(ctx context.Context, input *config_j5pb.Input) (*source_j5pb.SourceImage, error)
}

type RepoRoot struct {
	thisRepo *repo
	resolver RemoteResolver
}

func NewFSRepoRoot(ctx context.Context, root fs.FS, resolver RemoteResolver) (*RepoRoot, error) {
	src := &RepoRoot{
		resolver: resolver,
	}

	thisRepo, err := src.newRepo(".", root)
	if err != nil {
		return nil, err
	}
	src.thisRepo = thisRepo

	return src, nil
}

func (src *RepoRoot) ListAllDependencies() ([]*config_j5pb.Input, error) {
	allDeps := []*config_j5pb.Input{}
	for _, bundle := range src.thisRepo.bundles {
		cfg, err := bundle.J5Config()
		if err != nil {
			return nil, fmt.Errorf("bundle %q: %w", bundle.DebugName(), err)
		}
		allDeps = append(allDeps, cfg.Dependencies...)
	}

	for _, generated := range src.thisRepo.config.Generate {
		allDeps = append(allDeps, generated.Inputs...)
	}
	return allDeps, nil
}

func (src *RepoRoot) GetSourceImage(ctx context.Context, input *config_j5pb.Input) (*source_j5pb.SourceImage, error) {
	if local, ok := input.Type.(*config_j5pb.Input_Local); ok {
		bundle := src.thisRepo.bundleByName(local.Local)
		if bundle == nil {
			return nil, fmt.Errorf("bundle %q not found", local.Local)
		}
		return bundle.SourceImage(ctx, src)
	}

	return src.resolver.GetRemoteDependency(ctx, input, src.thisRepo.lockFile)
}

type repo struct {
	repoRoot fs.FS
	bundles  []*bundleSource
	config   *config_j5pb.RepoConfigFile
	lockFile *config_j5pb.LockFile
}

func (rr *repo) bundleByName(name string) *bundleSource {
	for _, bundle := range rr.bundles {
		if bundle.refConfig.Name == name {
			return bundle
		}
	}
	return nil
}

func (src *RepoRoot) newRepo(debugName string, repoRoot fs.FS) (*repo, error) {

	config, err := readDirConfigs(repoRoot)
	if err != nil {
		return nil, err
	}

	pluginBase, err := repoPluginBase(config)
	if err != nil {
		return nil, err
	}

	if err := resolveRepoPluginReferences(pluginBase, config); err != nil {
		return nil, fmt.Errorf("resolving config references: %w", err)
	}

	lockFile, err := readLockFile(repoRoot, "j5-lock.yaml")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading lock file: %w", err)
		}
		lockFile = &config_j5pb.LockFile{}
	}

	thisRepo := &repo{
		config:   config,
		repoRoot: repoRoot,
		lockFile: lockFile,
	}

	for _, refConfig := range config.Bundles {
		bundleRoot := repoRoot
		debugName := fmt.Sprintf("%s/%s", debugName, refConfig.Dir)
		if refConfig.Dir != "" {
			bundleRoot, err = fs.Sub(bundleRoot, refConfig.Dir)
			if err != nil {
				return nil, fmt.Errorf("bundle %q: %w", debugName, err)
			}
		}

		bundleConfig, err := readBundleConfigFile(bundleRoot)
		if err != nil {
			return nil, fmt.Errorf("bundle %q: %w", debugName, err)
		}

		err = resolveBundlePluginReferences(pluginBase, bundleConfig)
		if err != nil {
			return nil, fmt.Errorf("bundle %q Plugin References: %w", debugName, err)
		}

		thisRepo.bundles = append(thisRepo.bundles, &bundleSource{
			debugName: debugName,
			fs:        bundleRoot,
			dirInRepo: refConfig.Dir,
			refConfig: refConfig,
			config:    bundleConfig,
		})
	}

	if len(config.Packages) > 0 || len(config.Publish) > 0 || config.Registry != nil {
		// Inline Bundle
		thisRepo.bundles = append(thisRepo.bundles, &bundleSource{
			debugName: debugName,
			fs:        repoRoot,
			refConfig: &config_j5pb.BundleReference{
				Name: "",
				Dir:  "",
			},
			config: &config_j5pb.BundleConfigFile{
				Registry:     config.Registry,
				Publish:      config.Publish,
				Packages:     config.Packages,
				Options:      config.Options,
				Dependencies: config.Dependencies,
			},
		})
	}

	if err := thisRepo.validateBundles(); err != nil {
		return nil, err
	}

	return thisRepo, nil
}

func (src *repo) validateBundles() error {
	seenLocal := map[string]struct{}{}
	for _, bundle := range src.bundles {
		for _, dep := range bundle.localDependencies() {
			if _, ok := seenLocal[dep]; !ok {
				return fmt.Errorf("bundle %q depends on local %q, which is not defined (bundles are loaded in order)", bundle.debugName, dep)
			}
		}
		seenLocal[bundle.refConfig.Name] = struct{}{}
	}
	return nil
}

func (src RepoRoot) RepoConfig() *config_j5pb.RepoConfigFile {
	return src.thisRepo.config
}

func (src RepoRoot) AllBundles() []*bundleSource {
	return src.thisRepo.bundles
}

func (src *RepoRoot) CombinedSourceImage(ctx context.Context, inputs []*config_j5pb.Input) (*source_j5pb.SourceImage, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no inputs")
	}
	if len(inputs) == 1 {
		return src.GetSourceImage(ctx, inputs[0])
	}

	fullImage := &source_j5pb.SourceImage{}

	images := make([]*source_j5pb.SourceImage, 0, len(inputs))
	for _, input := range inputs {
		img, err := src.GetSourceImage(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("input %v: %w", input, err)
		}
		images = append(images, img)

		fullImage.Packages = append(fullImage.Packages, img.Packages...)
	}

	combined, err := combineSourceImages(images)
	if err != nil {
		return nil, err
	}

	files, sourceFilenames := combined.AllDependencyFiles()
	fullImage.File = files
	fullImage.SourceFilenames = sourceFilenames

	return fullImage, nil
}

func (src *RepoRoot) BundleDependencies(ctx context.Context, name string) (DependencySet, error) {
	bs, err := src.BundleSource(name)
	if err != nil {
		return nil, err
	}
	return bs.GetDependencies(ctx, src)
}

func (src *RepoRoot) BundleImageSource(ctx context.Context, name string) (*source_j5pb.SourceImage, *config_j5pb.BundleConfigFile, error) {
	bundleSource, err := src.BundleSource(name)
	if err != nil {
		return nil, nil, err
	}

	img, err := bundleSource.SourceImage(ctx, src)
	if err != nil {
		return nil, nil, err
	}

	cfg, err := bundleSource.J5Config()
	if err != nil {
		return nil, nil, err
	}

	return img, cfg, nil
}

func (src *RepoRoot) BundleSource(name string) (*bundleSource, error) {
	if name != "" {
		if bundle := src.thisRepo.bundleByName(name); bundle != nil {
			return bundle, nil
		}
		return nil, fmt.Errorf("bundle %q not found", name)
	}
	if len(src.thisRepo.bundles) == 0 {
		return nil, fmt.Errorf("no bundles found")
	}
	if len(src.thisRepo.bundles) > 1 {
		return nil, fmt.Errorf("multiple bundles found, must specify a name")
	}

	for _, bundle := range src.thisRepo.bundles {
		return bundle, nil
	}

	return nil, fmt.Errorf("no bundles found")

}

func (src *RepoRoot) SourceFile(ctx context.Context, filename string) ([]byte, error) {
	return fs.ReadFile(src.thisRepo.repoRoot, filename)
}
