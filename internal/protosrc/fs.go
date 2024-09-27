package protosrc

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
)

func NewFSResolver(fs fs.FS) protocompile.Resolver {
	return &protocompile.SourceResolver{
		Accessor: func(filename string) (io.ReadCloser, error) {
			return fs.Open(filename)
		},
	}
}

func ReadFSImage(ctx context.Context, bundleRoot fs.FS, includeFilenames []string, dependencies protocompile.Resolver) (*source_j5pb.SourceImage, error) {

	proseFiles := []*source_j5pb.ProseFile{}
	filenames := includeFilenames
	err := fs.WalkDir(bundleRoot, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))

		switch ext {
		case ".proto":
			filenames = append(filenames, path)
			return nil

		case ".md":
			data, err := fs.ReadFile(bundleRoot, path)
			if err != nil {
				return err
			}
			proseFiles = append(proseFiles, &source_j5pb.ProseFile{
				Path:    path,
				Content: data,
			})
			return nil

		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	resolver := protocompile.CompositeResolver{
		NewFSResolver(bundleRoot),
		dependencies,
	}
	compiler := NewCompiler(resolver)
	files, err := compiler.Compile(ctx, filenames)
	if err != nil {
		return nil, err
	}

	return &source_j5pb.SourceImage{
		File:            files,
		Prose:           proseFiles,
		SourceFilenames: filenames,
	}, nil
}
