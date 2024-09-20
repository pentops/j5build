package source

import (
	"context"
	"fmt"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/log.go/log"
)

type RegistryClient interface {
	GetImage(ctx context.Context, owner, repoName, version string) (*source_j5pb.SourceImage, error)
	LatestImage(ctx context.Context, owner, repoName string, reference *string) (*source_j5pb.SourceImage, error)
}

type Resolver struct {
	regClient RegistryClient
	j5Cache   *j5Cache
}

func NewResolver(regClient RegistryClient) (*Resolver, error) {
	return &Resolver{
		regClient: regClient,
	}, nil
}

func NewEnvResolver() (*Resolver, error) {
	regClient, err := envRegistryClient()
	if err != nil {
		return nil, err
	}

	cache, err := newJ5Cache()
	if err != nil {
		return nil, err
	}

	return &Resolver{
		regClient: regClient,
		j5Cache:   cache,
	}, nil
}

func (rr *Resolver) GetRemoteDependency(ctx context.Context, input *config_j5pb.Input, locks *config_j5pb.LockFile) (*source_j5pb.SourceImage, error) {
	switch st := input.Type.(type) {

	case *config_j5pb.Input_Registry_:
		return rr.cacheDance(ctx, cacheSpec{
			repoType:  "registry",
			owner:     st.Registry.Owner,
			repoName:  st.Registry.Name,
			version:   st.Registry.Version,
			reference: coalesce(st.Registry.Reference, ptr("main")),
		}, rr.regClient, locks)

	default:
		return nil, fmt.Errorf("unsupported source type %T", input.Type)
	}

}

type cacheSpec struct {
	repoType  string
	owner     string
	repoName  string
	version   *string
	reference *string
}

func (rr *Resolver) cacheDance(ctx context.Context, spec cacheSpec, source RegistryClient, locks *config_j5pb.LockFile) (*source_j5pb.SourceImage, error) {

	fullName := fmt.Sprintf("%s/%s/%s", spec.repoType, spec.owner, spec.repoName)
	ctx = log.WithField(ctx, "bundle", fullName)
	var version *string
	if spec.version != nil {
		version = ptr(*spec.version)
		ctx = log.WithField(ctx, "specVersion", *version)
	} else if lockVersion := getInputLockVersion(locks, fullName); lockVersion != nil {
		ctx = log.WithField(ctx, "lockVersion", *lockVersion)
		log.Debug(ctx, "using lock version")
		version = ptr(*lockVersion)
	}

	// only use cache if version is explicit, otherwise needs to pull latest
	if version != nil {
		if cached, ok := rr.getCachedInput(ctx, fullName, *version); ok {
			log.Debug(ctx, "using cached input")
			return cached, nil
		}
	}
	if version == nil {
		if spec.reference != nil {
			version = ptr(*spec.reference)
		} else {
			version = ptr("main")
		}
	}

	ctx = log.WithField(ctx, "depVersion", *version)
	log.Debug(ctx, "cache miss")

	img, err := source.GetImage(ctx, spec.owner, spec.repoName, *version)
	if err != nil {
		return nil, err
	}
	if img.SourceName == "" {
		img.SourceName = fullName
	}

	if rr.j5Cache != nil && img.Version != nil {
		if err := rr.j5Cache.put(ctx, fullName, *img.Version, img); err != nil {
			log.WithError(ctx, err).Error("failed to cache input")
		}
	}

	return img, nil
}

func (src *Resolver) LatestLocks(ctx context.Context, deps []*config_j5pb.Input) (*config_j5pb.LockFile, error) {

	lockFile := &config_j5pb.LockFile{}
	seen := map[string]struct{}{}
	for _, dep := range deps {
		var spec *cacheSpec
		var resolver RegistryClient
		switch st := dep.Type.(type) {
		case *config_j5pb.Input_Registry_:
			spec = &cacheSpec{
				repoType:  "registry",
				owner:     st.Registry.Owner,
				repoName:  st.Registry.Name,
				version:   st.Registry.Version,
				reference: coalesce(st.Registry.Reference, ptr("main")),
			}
			resolver = src.regClient

		default:
			continue
		}

		fullName := fmt.Sprintf("%s/%s/%s", spec.repoType, spec.owner, spec.repoName)
		if _, ok := seen[fullName]; ok {
			continue
		}
		seen[fullName] = struct{}{}

		img, err := resolver.LatestImage(ctx, spec.owner, spec.repoName, spec.reference)
		if err != nil {
			return nil, err
		}

		if img == nil || img.Version == nil || *img.Version == "" {
			return nil, fmt.Errorf("no version for %s", fullName)
		}

		if src.j5Cache != nil && img.Version != nil {
			if err := src.j5Cache.put(ctx, fullName, *img.Version, img); err != nil {
				log.WithError(ctx, err).Error("failed to cache input")
			}
		}

		ctx = log.WithFields(ctx, map[string]interface{}{
			"dep":         fullName,
			"lockVersion": *img.Version,
		})
		log.Info(ctx, "adding lock")

		lock := &config_j5pb.InputLock{
			Name:    fullName,
			Version: *img.Version,
		}

		lockFile.Inputs = append(lockFile.Inputs, lock)
	}
	return lockFile, nil

}

func getInputLockVersion(locks *config_j5pb.LockFile, name string) *string {
	if locks == nil {
		return nil
	}
	for _, dep := range locks.Inputs {
		if dep.Name == name {
			return ptr(dep.Version)
		}
	}
	return nil
}

func ptr[T any](v T) *T {
	return &v
}

func coalesce[T any](vals ...*T) *T {
	for _, val := range vals {
		if val != nil {
			return val
		}
	}
	return nil
}

func (src *Resolver) getCachedInput(ctx context.Context, name, version string) (*source_j5pb.SourceImage, bool) {
	if src.j5Cache == nil {
		return nil, false
	}
	image, ok := src.j5Cache.tryGet(ctx, name, version)
	if !ok {
		return nil, false
	}
	if image.SourceName == "" {
		image.SourceName = name
	}
	return image, true
}
