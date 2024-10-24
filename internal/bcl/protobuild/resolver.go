package protobuild

import (
	"context"
	"fmt"
	"strings"

	"github.com/pentops/j5build/internal/protosrc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type DependencySet interface {
	ListDependencyFiles(root string) []string
	GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error)
}

type Resolver struct {
	ExternalDeps DependencySet
}

func newResolver(externalDeps DependencySet) (*Resolver, error) {
	rr := &Resolver{
		ExternalDeps: externalDeps,
	}
	return rr, nil
}

type DependencyFile struct {
	Desc *descriptorpb.FileDescriptorProto
	Refl *protoreflect.FileDescriptor
}

func (rr *Resolver) readFile(ctx context.Context, filename string) (*SearchResult, error) {
	if protosrc.IsBuiltInProto(filename) {
		refl, err := protoregistry.GlobalFiles.FindFileByPath(filename)
		if err != nil {
			return nil, fmt.Errorf("global file: %w", err)
		}
		summary, err := buildSummaryFromReflect(refl)
		if err != nil {
			return nil, err
		}
		return &SearchResult{
			Filename: filename,
			Summary:  summary,
			Refl:     refl,
		}, nil
	}

	file, err := rr.ExternalDeps.GetDependencyFile(filename)
	if err != nil {
		return nil, fmt.Errorf("dependency file: %w", err)
	}

	summary, err := buildSummaryFromDescriptor(file)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Filename: filename,
		Summary:  summary,
		Desc:     file,
	}, nil
}

func (rr *Resolver) PackageFiles(ctx context.Context, pkgName string) ([]*SearchResult, error) {
	filenames, err := rr.listPackageFiles(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	files := make([]*SearchResult, 0)
	for _, filename := range filenames {
		file, err := rr.readFile(ctx, filename)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (rr *Resolver) listPackageFiles(ctx context.Context, pkgName string) ([]string, error) {
	root := strings.ReplaceAll(pkgName, ".", "/")
	if protosrc.IsBuiltInProto(root + "/") {
		return []string{}, nil
	}

	files := rr.ExternalDeps.ListDependencyFiles(root)
	if len(files) == 0 {
		return nil, fmt.Errorf("no files for package at %s", root)
	}
	return files, nil
}
