package source

import (
	"fmt"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type DependencySet interface {
	GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error)
	ListDependencyFiles(prefix string) []string
	AllDependencyFiles() ([]*descriptorpb.FileDescriptorProto, []string)
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type imageFiles struct {
	primary      map[string]*descriptorpb.FileDescriptorProto
	dependencies map[string]*descriptorpb.FileDescriptorProto
}

func (ii *imageFiles) FindFileByPath(filename string) (protocompile.SearchResult, error) {
	depFile, err := ii.GetDependencyFile(filename)
	if err == nil {
		return protocompile.SearchResult{
			Proto: depFile,
		}, nil
	}
	return protocompile.SearchResult{}, fmt.Errorf("FindPackageByPath: file %s not found", filename)
}

func (ii *imageFiles) GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error) {
	if file, ok := ii.primary[filename]; ok {
		return file, nil
	}
	if file, ok := ii.dependencies[filename]; ok {
		return file, nil
	}
	return nil, fmt.Errorf("could not find file %q", filename)
}

func (ii *imageFiles) ListDependencyFiles(prefix string) []string {

	files := make([]string, 0, len(ii.primary))
	for _, file := range ii.primary {
		name := file.GetName()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		files = append(files, name)
	}
	return files
}

func (ii *imageFiles) AllDependencyFiles() ([]*descriptorpb.FileDescriptorProto, []string) {

	files := make([]*descriptorpb.FileDescriptorProto, 0, len(ii.primary)+len(ii.dependencies))
	filenames := make([]string, 0, len(ii.primary))

	for filename, file := range ii.dependencies {
		if _, ok := ii.primary[filename]; ok {
			continue
		}
		files = append(files, file)
	}
	for _, file := range ii.primary {
		files = append(files, file)
		filenames = append(filenames, file.GetName())
	}
	return files, filenames
}

func combineSourceImages(images []*source_j5pb.SourceImage) (*imageFiles, error) {

	fileMap := map[string]*descriptorpb.FileDescriptorProto{}
	depMap := map[string]*descriptorpb.FileDescriptorProto{}
	fileSourceMap := map[string]*source_j5pb.SourceImage{}
	for _, img := range images {
		isSource := map[string]bool{}
		for _, file := range img.SourceFilenames {
			isSource[file] = true
		}

		for _, file := range img.File {
			if !isSource[*file.Name] {
				depMap[*file.Name] = file
				continue
			}
			existing, ok := fileMap[*file.Name]
			if !ok {
				fileMap[*file.Name] = file
				fileSourceMap[*file.Name] = img
				continue
			}

			if proto.Equal(existing, file) {
				continue
			}

			a := fileSourceMap[*file.Name]
			aName := fmt.Sprintf("%s:%s", a.SourceName, strVal(a.Version))
			bName := fmt.Sprintf("%s:%s", img.SourceName, strVal(img.Version))

			return nil, fmt.Errorf("file %q has conflicting content in %s and %s", *file.Name, aName, bName)
		}
	}

	combined := &imageFiles{
		primary:      fileMap,
		dependencies: depMap,
	}

	return combined, nil
}
