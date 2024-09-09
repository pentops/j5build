package source

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/proto"
)

type j5Cache struct {
	dir string
}

func newJ5Cache() (*j5Cache, error) {
	cacheDir := os.Getenv("J5_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.Getenv("HOME"), ".cache", "j5")
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	return &j5Cache{
		dir: cacheDir,
	}, nil
}

func (c *j5Cache) tryGet(ctx context.Context, name, version string) (*source_j5pb.SourceImage, bool) {
	data, err := os.ReadFile(filepath.Join(c.dir, name, version, "src.img"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		log.WithError(ctx, err).Error("failed to read cache file")
		return nil, false
	}

	img := &source_j5pb.SourceImage{}
	if err := proto.Unmarshal(data, img); err != nil {
		log.WithError(ctx, err).Error("failed to unmarshal cached image")
		return nil, false
	}

	return img, true
}

func (c *j5Cache) put(ctx context.Context, name, version string, img *source_j5pb.SourceImage) error {
	log.WithFields(ctx, map[string]interface{}{
		"bundle":  name,
		"version": version,
	}).Debug("caching source image")
	data, err := proto.Marshal(img)
	if err != nil {
		return err
	}
	dir := filepath.Join(c.dir, name, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, "src.img"), data, 0644); err != nil {
		return err
	}
	return nil
}
