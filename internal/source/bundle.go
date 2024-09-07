package source

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
)

type bundleSource struct {
	debugName string
	fs        fs.FS
	refConfig *config_j5pb.BundleReference
	config    *config_j5pb.BundleConfigFile
	dirInRepo string
}

func (b bundleSource) Name() string {
	return b.debugName
}

func (b *bundleSource) J5Config() (*config_j5pb.BundleConfigFile, error) {
	if b.config != nil {
		return b.config, nil
	}

	config, err := readBundleConfigFile(b.fs, "j5.yaml")
	if err != nil {
		return nil, fmt.Errorf("config for bundle %s: %w", b.debugName, err)
	}

	b.config = config
	return config, nil
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
