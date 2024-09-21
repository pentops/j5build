package cli

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/pentops/j5build/internal/conversions/protobuild"
	"github.com/pentops/prototools/protoprint"
	"golang.org/x/exp/maps"
)

func runJ5sInfo(ctx context.Context, cfg struct {
	SourceConfig
	Filename string `flag:"filename" desc:"Filename to print information about"`
}) error {

	bs, err := newBundleSet(ctx, cfg.SourceConfig)
	if err != nil {
		return err
	}

	info, err := bs.resolver.FileInfo(ctx, cfg.Filename)
	if err != nil {
		return err
	}

	fmt.Printf("File: %s\n", cfg.Filename)
	fmt.Printf("Package: %s\n", info.Package)
	exports := maps.Keys(info.Exports)
	sort.Strings(exports)
	for _, key := range exports {
		export := info.Exports[key]
		fmt.Printf("Export: %s\n", export.Name)
	}

	return nil
}

type bundleSet struct {
	resolver *protobuild.Resolver
	packages []string
}

func newBundleSet(ctx context.Context, cfg SourceConfig) (*bundleSet, error) {

	src, err := cfg.GetSource(ctx)
	if err != nil {
		return nil, err
	}

	bundleDir, err := src.BundleDir(cfg.Bundle)
	if err != nil {
		return nil, err
	}

	bundleConfig, err := src.BundleConfig(cfg.Bundle)
	if err != nil {
		return nil, err
	}

	bundleFS, err := src.BundleFS(cfg.Bundle)
	if err != nil {
		return nil, err
	}

	packages := []string{}
	for _, pkg := range bundleConfig.Packages {
		packages = append(packages, pkg.Name)
	}

	localFiles := &fileReader{
		fs:       bundleFS,
		fsName:   bundleDir,
		packages: packages,
	}

	deps, err := src.BundleDependencies(ctx, cfg.Bundle)
	if err != nil {
		return nil, err
	}

	resolver, err := protobuild.NewResolver(deps, localFiles)
	if err != nil {
		return nil, err
	}

	return &bundleSet{
		resolver: resolver,
		packages: packages,
	}, nil
}

func runGenProto(ctx context.Context, cfg struct {
	SourceConfig
	Verbose bool `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
}) error {

	bs, err := newBundleSet(ctx, cfg.SourceConfig)
	if err != nil {
		return err
	}

	outWriter, err := cfg.BundleWriter(ctx)
	if err != nil {
		return err
	}

	compiler := protobuild.NewCompiler(bs.resolver)

	for _, pkg := range bs.packages {
		out, err := compiler.CompilePackage(ctx, pkg)
		if err != nil {
			return err
		}

		for _, file := range out {
			filename := file.Path()
			if !strings.HasSuffix(filename, ".j5s.proto") {
				continue
			}

			out, err := protoprint.PrintFile(ctx, file)
			if err != nil {
				log.Printf("Error printing %s: %v", filename, err)
				return err
			}

			err = outWriter.PutFile(ctx, filename, []byte(out))
			if err != nil {
				return err
			}

		}

	}

	return nil

}

type fileReader struct {
	fs       fs.FS
	fsName   string
	packages []string
}

func (rr *fileReader) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	return fs.ReadFile(rr.fs, filename)
}

func (rr *fileReader) ListPackages() []string {
	return rr.packages
}

func (rr *fileReader) ListSourceFiles(ctx context.Context, pkgName string) ([]string, error) {
	pkgRoot := strings.ReplaceAll(pkgName, ".", "/")

	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, pkgRoot, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5s.proto") {
			return nil
		}
		if strings.HasSuffix(path, ".proto") || strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", rr.fsName, err)
	}
	return files, nil
}

func (rr *fileReader) ListJ5Files(ctx context.Context) ([]string, error) {
	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, ".", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil

}
