package protobuild

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/pentops/j5build/internal/bcl/errpos"
	"github.com/pentops/j5build/internal/j5s/j5convert"
	"github.com/pentops/j5build/internal/j5s/j5parse"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/log.go/log"
)

type LocalFileSource interface {
	GetLocalFile(context.Context, string) ([]byte, error)
	ListPackages() []string
	ListSourceFiles(ctx context.Context, pkgName string) ([]string, error)
}

func NewBundleResolver(ctsx context.Context, bundle source.Bundle) (LocalFileSource, error) {

	bundleDir := bundle.DirInRepo()

	bundleConfig, err := bundle.J5Config()
	if err != nil {
		return nil, err
	}

	bundleFS := bundle.FS()

	packages := []string{}
	for _, pkg := range bundleConfig.Packages {
		packages = append(packages, pkg.Name)
	}

	localFiles := &fileReader{
		fs:       bundleFS,
		fsName:   bundleDir,
		packages: packages,
	}

	return localFiles, nil
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

type sourceResolver struct {
	bundleFiles       LocalFileSource
	j5Parser          *j5parse.Parser
	localPrefixes     []string
	localPackageNames map[string]struct{}
}

func newSourceResolver(localFiles LocalFileSource) (*sourceResolver, error) {
	packages := localFiles.ListPackages()

	localPackageNames := map[string]struct{}{}
	localPrefixes := make([]string, len(packages))
	for i, p := range packages {
		s := strings.ReplaceAll(p, ".", "/")
		localPrefixes[i] = s + "/"
		localPackageNames[p] = struct{}{}
	}

	j5Parser, err := j5parse.NewParser()
	if err != nil {
		return nil, err
	}

	sr := &sourceResolver{
		j5Parser: j5Parser,

		bundleFiles:       localFiles,
		localPackageNames: localPackageNames,
		localPrefixes:     localPrefixes,
	}

	return sr, nil
}

func (sr *sourceResolver) ListPackages() []string {
	return sr.bundleFiles.ListPackages()
}

func (sr *sourceResolver) packageForFile(filename string) (string, bool, error) {
	if !hasAPrefix(filename, sr.localPrefixes) {
		// not a local file, not in scope.
		return "", false, nil
	}

	pkg, _, err := j5convert.SplitPackageFromFilename(filename)
	if err != nil {
		return "", false, err
	}
	return pkg, true, nil
}

func (sr *sourceResolver) isLocalPackage(pkgName string) bool {
	_, ok := sr.localPackageNames[pkgName]
	return ok
}

func (sr *sourceResolver) listPackageFiles(ctx context.Context, pkgName string) ([]string, error) {
	root := strings.ReplaceAll(pkgName, ".", "/")

	files, err := sr.bundleFiles.ListSourceFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0)
	for _, f := range files {
		if strings.HasSuffix(f, ".j5s.proto") {
			continue
		}
		dir := path.Dir(f)
		if dir != root {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered, nil
}

func (sr *sourceResolver) getFileContent(ctx context.Context, sourceFilename string) ([]byte, error) {
	return sr.bundleFiles.GetLocalFile(ctx, sourceFilename)
}

func (sr *sourceResolver) getFile(ctx context.Context, sourceFilename string, ec *ErrCollector) (*SourceFile, error) {
	log.WithField(ctx, "sourceFilename", sourceFilename).Debug("read local source file")

	data, err := sr.bundleFiles.GetLocalFile(ctx, sourceFilename)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(sourceFilename, ".j5s") {
		return sr.parseJ5s(ctx, sourceFilename, data, ec)
	}

	if strings.HasSuffix(sourceFilename, ".proto") {
		return sr.parseProto(ctx, sourceFilename, data, ec)
	}

	return nil, fmt.Errorf("unsupported file type: %s", sourceFilename)
}

func (sr *sourceResolver) parseJ5s(_ context.Context, sourceFilename string, data []byte, ec *ErrCollector) (*SourceFile, error) {

	sourceFile, err := sr.j5Parser.ParseFile(sourceFilename, string(data))
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(data))
	}

	summary, err := j5convert.SourceSummary(sourceFile, ec)
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(data))
	}

	return &SourceFile{
		Filename:  sourceFilename,
		RawSource: data,
		J5Source:  sourceFile,
		Summary:   summary,
	}, nil
}

func (sr *sourceResolver) parseProto(ctx context.Context, sourceFilename string, data []byte, ec *ErrCollector) (*SourceFile, error) {

	result, summary, err := protoToDescriptor(ctx, sourceFilename, data, ec)
	if err != nil {
		return nil, err
	}

	return &SourceFile{
		Filename:  sourceFilename,
		Summary:   summary,
		RawSource: data,

		Result: &SearchResult{
			Summary:     summary,
			ParseResult: &result,
		},
	}, nil
}
