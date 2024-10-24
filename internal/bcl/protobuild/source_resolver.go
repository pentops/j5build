package protobuild

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"github.com/pentops/j5build/internal/bcl/j5parse"
	"golang.org/x/exp/maps"
)

type LocalFileSource interface {
	GetLocalFile(context.Context, string) ([]byte, error)
	ListPackages() []string
	ListSourceFiles(ctx context.Context, pkgName string) ([]string, error)
}

type sourceResolver struct {
	BundleFiles       LocalFileSource
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
	}

	j5Parser, err := j5parse.NewParser()
	if err != nil {
		return nil, err
	}

	sr := &sourceResolver{
		j5Parser: j5Parser,

		BundleFiles:       localFiles,
		localPackageNames: localPackageNames,
		localPrefixes:     localPrefixes,
	}

	return sr, nil
}

func (sr *sourceResolver) listPackages() []string {
	pkgs := maps.Keys(sr.localPackageNames)
	sort.Strings(pkgs)
	return pkgs
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

	files, err := sr.BundleFiles.ListSourceFiles(ctx, root)
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

func (sr *sourceResolver) getFileData(ctx context.Context, filename string) ([]byte, error) {
	return sr.BundleFiles.GetLocalFile(ctx, filename)
}

func (sr *sourceResolver) getFile(ctx context.Context, sourceFilename string) (*SourceFile, error) {
	data, err := sr.BundleFiles.GetLocalFile(ctx, sourceFilename)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(sourceFilename, ".j5s") {
		return sr.parseJ5s(ctx, sourceFilename, data)
	}

	if strings.HasSuffix(sourceFilename, ".proto") {
		return sr.parseProto(ctx, sourceFilename, data)
	}

	return nil, fmt.Errorf("unsupported file type: %s", sourceFilename)
}

func (sr *sourceResolver) parseJ5s(ctx context.Context, sourceFilename string, data []byte) (*SourceFile, error) {

	sourceFile, err := sr.j5Parser.ParseFile(sourceFilename, string(data))
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(data))
	}

	summary, err := j5convert.SourceSummary(sourceFile)
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

func (sr *sourceResolver) parseProto(ctx context.Context, sourceFilename string, data []byte) (*SourceFile, error) {
	data, err := sr.BundleFiles.GetLocalFile(ctx, sourceFilename)
	if err != nil {
		return nil, err
	}
	result, summary, err := protoToDescriptor(ctx, sourceFilename, data)
	if err != nil {
		return nil, err
	}
	return &SourceFile{
		Filename:  sourceFilename,
		Summary:   summary,
		RawSource: data,

		Result: &SearchResult{ParseResult: &result},
	}, nil
}
