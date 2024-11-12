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

// dependencyResolver provides non-local proto files, either Global or from a
// DependencySet.
type dependencyResolver struct {
	deps        DependencySet
	resultCache map[string]*SearchResult
}

func newDependencyResolver(externalDeps DependencySet) (*dependencyResolver, error) {
	rr := &dependencyResolver{
		deps:        externalDeps,
		resultCache: make(map[string]*SearchResult),
	}
	return rr, nil
}

type DependencyFile struct {
	Desc *descriptorpb.FileDescriptorProto
	Refl *protoreflect.FileDescriptor
}

func (rr *dependencyResolver) findFileByPath(ctx context.Context, filename string) (*SearchResult, error) {
	if res, ok := rr.resultCache[filename]; ok {
		return res, nil
	}
	file, err := rr.loadFile(ctx, filename)
	if err != nil {
		return nil, err
	}
	rr.resultCache[filename] = file
	return file, nil
}

func (rr *dependencyResolver) loadFile(_ context.Context, filename string) (*SearchResult, error) {

	ec := &ErrCollector{}
	if protosrc.IsBuiltInProto(filename) {
		refl, err := protoregistry.GlobalFiles.FindFileByPath(filename)
		if err != nil {
			return nil, fmt.Errorf("find builtin file %s: %w", filename, err)
		}
		summary, err := buildSummaryFromReflect(refl, ec)
		if err != nil {
			return nil, fmt.Errorf("summary for builtin %s: %w", filename, err)
		}
		return &SearchResult{
			Summary: summary,
			Refl:    refl,
		}, nil
	}

	file, err := rr.deps.GetDependencyFile(filename)
	if err != nil {
		return nil, fmt.Errorf("dependency file: %w", err)
	}

	summary, err := buildSummaryFromDescriptor(file, ec)
	if err != nil {
		return nil, fmt.Errorf("summary for dependency %s: %w", file, err)
	}
	return &SearchResult{
		Summary: summary,
		Desc:    file,
	}, nil
}

func (rr *dependencyResolver) PackageFiles(ctx context.Context, pkgName string) ([]*SearchResult, error) {
	filenames, err := rr.listPackageFiles(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	files := make([]*SearchResult, 0)
	for _, filename := range filenames {
		file, err := rr.findFileByPath(ctx, filename)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (rr *dependencyResolver) listPackageFiles(_ context.Context, pkgName string) ([]string, error) {
	root := strings.ReplaceAll(pkgName, ".", "/")
	if protosrc.IsBuiltInProto(root + "/") {
		return []string{}, nil
	}

	files := rr.deps.ListDependencyFiles(root)
	if len(files) == 0 {
		return nil, fmt.Errorf("no files for package at %s", root)
	}
	return files, nil
}
