package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protoyaml-go"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pentops/j5/gen/j5/config/v1/config_j5pb"
	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	// Pre Loaded Protos
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	_ "github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	_ "github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	_ "github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	_ "github.com/pentops/j5/gen/j5/state/v1/psm_j5pb"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	_ "google.golang.org/genproto/googleapis/api/httpbody"
)

var configPaths = []string{
	"j5.yaml",
	"ext/j5/j5.yaml",
}

func readDirConfigs(root fs.FS) (*config_j5pb.RepoConfigFile, error) {
	var config *config_j5pb.RepoConfigFile
	var err error
	for _, filename := range configPaths {
		config, err = readConfigFile(root, filename)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("reading file %s: %w", filename, err)
		}
		break
	}

	if config == nil {
		return nil, fmt.Errorf("no J5 config found")
	}

	return config, nil
}

func readConfigFile(root fs.FS, filename string) (*config_j5pb.RepoConfigFile, error) {
	data, err := fs.ReadFile(root, filename)
	if err != nil {
		return nil, err
	}
	config := &config_j5pb.RepoConfigFile{}
	if err := protoyaml.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func readBundleConfigFile(root fs.FS, filename string) (*config_j5pb.BundleConfigFile, error) {
	data, err := fs.ReadFile(root, filename)
	if err != nil {
		return nil, err
	}
	config := &config_j5pb.BundleConfigFile{}
	if err := protoyaml.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func readLockFile(root fs.FS, filename string) (*config_j5pb.LockFile, error) {
	data, err := fs.ReadFile(root, filename)
	if err != nil {
		return nil, err
	}
	lockFile := &config_j5pb.LockFile{}
	if err := protoyaml.Unmarshal(data, lockFile); err != nil {
		return nil, err
	}
	return lockFile, nil
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

	dependencyImage, err := combineSourceImages(dependencies)
	if err != nil {
		return nil, err
	}
	return dependencyImage, nil
}

func (bundle *bundleSource) readImageFromDir(ctx context.Context, resolver InputSource) (*source_j5pb.SourceImage, error) {

	dependencies, err := bundle.GetDependencies(ctx, resolver)
	if err != nil {
		return nil, err
	}

	img, err := readImageFromDir(ctx, bundle.fs, dependencies)
	if err != nil {
		return nil, err
	}

	img.Packages = bundle.config.Packages
	img.Options = bundle.config.Options
	return img, nil
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

	for _, file := range ii.primary {
		files = append(files, file)
		filenames = append(filenames, file.GetName())
	}
	for filename, file := range ii.dependencies {
		if _, ok := ii.primary[filename]; ok {
			continue
		}
		files = append(files, file)
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

type DependencySet interface {
	GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error)
	ListDependencyFiles(prefix string) []string
	AllDependencyFiles() ([]*descriptorpb.FileDescriptorProto, []string)
}

func readImageFromDir(ctx context.Context, bundleRoot fs.FS, dependencies DependencySet) (*source_j5pb.SourceImage, error) {

	proseFiles := []*source_j5pb.ProseFile{}
	filenames := []string{}
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

	parser := protoparse.Parser{
		ImportPaths:                     []string{""},
		IncludeSourceCodeInfo:           true,
		InterpretOptionsInUnlinkedFiles: true,

		WarningReporter: func(err reporter.ErrorWithPos) {
			log.WithFields(ctx, map[string]interface{}{
				"error": err.Error(),
			}).Warn("protoparse warning")
		},
		LookupImport: func(filename string) (*desc.FileDescriptor, error) {
			for _, prefix := range []string{
				"google/protobuf/",
				"google/api/",
				"buf/validate/",
				"j5/ext/v1/",
				"j5/list/v1/",
				"j5/source/v1/",
				"j5/messaging/v1/",
				"j5/state/v1/",
				"j5/client/v1/",
			} {
				if strings.HasPrefix(filename, prefix) {
					ff, err := desc.LoadFileDescriptor(filename)
					if err != nil {
						fmt.Printf("%s ERROR in pre-loaded: %s\n", filename, err)
						return nil, fmt.Errorf("loading pre-loaded file %q: %w", filename, err)
					}
					fmt.Printf("%s IS in pre-loaded\n", filename)
					return ff, nil
				}
			}
			fmt.Printf("%s not in pre-loaded\n", filename)
			return nil, fmt.Errorf("could not find file %q", filename)

		},
		LookupImportProto: dependencies.GetDependencyFile,

		Accessor: func(filename string) (io.ReadCloser, error) {
			return bundleRoot.Open(filename)
		},
	}

	customDesc, err := parser.ParseFiles(filenames...)
	if err != nil {
		panicErr := protocompile.PanicError{}
		if errors.As(err, &panicErr) {
			fmt.Printf("PANIC: %s\n", panicErr.Stack)
		}

		return nil, err
	}

	realDesc := desc.ToFileDescriptorSet(customDesc...)

	return &source_j5pb.SourceImage{
		File:            realDesc.File,
		Prose:           proseFiles,
		SourceFilenames: filenames,
	}, nil
}
