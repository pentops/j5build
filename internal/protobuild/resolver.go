package protobuild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc/sourceinfo"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/j5convert"
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

var ErrNotFound = errors.New("File not found")

type LocalFileSource interface {
	GetLocalFile(context.Context, string) ([]byte, error)
	ListPackages() []string
	ListSourceFiles(ctx context.Context, pkgName string) ([]string, error)
}

type DependencySet interface {
	ListDependencyFiles(root string) []string
	GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error)
}

type Resolver struct {
	ExternalDeps  DependencySet
	BundleFiles   LocalFileSource
	localPrefixes []string

	reporter reporter.Reporter

	protoParser *ProtoParser

	j5Parser *j5parse.Parser

	inbuilt map[string]protocompile.SearchResult
}

func NewResolver(externalDeps DependencySet, localFiles LocalFileSource) (*Resolver, error) {
	packages := localFiles.ListPackages()
	packagePrefixes := make([]string, len(packages))
	for i, p := range packages {
		s := strings.ReplaceAll(p, ".", "/")
		packagePrefixes[i] = s + "/"

	}

	j5Parser, err := j5parse.NewParser()
	if err != nil {
		return nil, err
	}
	rr := &Resolver{
		ExternalDeps:  externalDeps,
		BundleFiles:   localFiles,
		localPrefixes: packagePrefixes,
		inbuilt:       map[string]protocompile.SearchResult{},
		j5Parser:      j5Parser,
	}

	errs := func(err reporter.ErrorWithPos) error {
		rr.Logf("Compiler Error: %s", err.Error())
		return err
	}

	warnings := func(err reporter.ErrorWithPos) {
		rr.Logf("Compiler Warning: %s", err.Error())
	}

	rr.reporter = reporter.NewReporter(errs, warnings)

	pp := NewProtoParser(rr.reporter)
	rr.protoParser = pp

	return rr, nil
}

func (rr *Resolver) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

type SourceFile struct {
	Filename string
	Summary  *j5convert.FileSummary
	J5Source *sourcedef_j5pb.SourceFile
	J5Data   []byte

	Result *protocompile.SearchResult
}

// These types are 'built in' to the J5 package set
var inbuiltPrefixes = []string{
	"google/protobuf/",
	"google/api/",
	"buf/validate/",
	"j5/types/",
	"j5/ext/v1/",
	"j5/list/v1/",
}

func hasAPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func (rr *Resolver) GetInbuilt(filename string) (protocompile.SearchResult, error) {
	if result, ok := rr.inbuilt[filename]; ok {
		return result, nil
	}
	desc, err := sourceinfo.GlobalFiles.FindFileByPath(filename)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	res := protocompile.SearchResult{
		Desc: desc,
	}
	rr.inbuilt[filename] = res
	return res, nil
}

func (rr *Resolver) findFileByPath(filename string) (*SourceFile, error) {
	if hasAPrefix(filename, inbuiltPrefixes) {
		result, err := rr.GetInbuilt(filename)
		if err != nil {
			return nil, err
		}
		summary, err := rr.protoParser.buildSummaryFromReflect(result.Desc)
		if err != nil {
			return nil, err
		}
		return &SourceFile{
			Filename: filename,
			Result:   &result,
			Summary:  summary,
		}, nil
	}

	if hasAPrefix(filename, rr.localPrefixes) {
		res, err := rr.localFile(filename)
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	file, err := rr.ExternalDeps.GetDependencyFile(filename)
	if err != nil {
		return nil, err
	}
	summary, err := rr.protoParser.buildSummaryFromDescriptor(file)
	if err != nil {
		return nil, err
	}
	return &SourceFile{
		Filename: filename,
		Summary:  summary,
		Result:   &protocompile.SearchResult{Proto: file},
	}, nil
}

func (rr *Resolver) PackageFiles(ctx context.Context, pkgName string) ([]*SourceFile, error) {
	filenames, err := rr.listPackageFiles(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	rr.Logf("files for package %s: %v", pkgName, filenames)

	files := make([]*SourceFile, 0)
	for _, filename := range filenames {
		file, err := rr.findFileByPath(filename)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (rr *Resolver) listPackageFiles(ctx context.Context, pkgName string) ([]string, error) {
	root := strings.ReplaceAll(pkgName, ".", "/")

	if hasAPrefix(root, inbuiltPrefixes) {
		return []string{}, nil

	} else if hasAPrefix(root+"/", rr.localPrefixes) {
		files, err := rr.BundleFiles.ListSourceFiles(ctx, root)
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
				rr.Logf("skipping file %s in dir %s", f, dir)
				continue
			}
			filtered = append(filtered, f)
		}
		return filtered, nil
	}

	return rr.ExternalDeps.ListDependencyFiles(root), nil
}

func (rr *Resolver) localFile(sourceFilename string) (*SourceFile, error) {
	ctx := context.Background()

	if strings.HasSuffix(sourceFilename, ".j5s") {
		return rr.parseToSource(ctx, sourceFilename)
	}

	if strings.HasSuffix(sourceFilename, ".proto") {
		data, err := rr.BundleFiles.GetLocalFile(ctx, sourceFilename)
		if err != nil {
			return nil, err
		}
		result, summary, err := rr.protoParser.protoToDescriptor(sourceFilename, data)
		if err != nil {
			return nil, err
		}
		return &SourceFile{
			Filename: sourceFilename,
			Summary:  summary,
			Result:   &protocompile.SearchResult{ParseResult: result},
		}, nil
	}

	return nil, fmt.Errorf("unknown file type for proto compile: %s", sourceFilename)
}

func (rr *Resolver) parseToSource(ctx context.Context, sourceFilename string) (*SourceFile, error) {

	data, err := rr.BundleFiles.GetLocalFile(ctx, sourceFilename)
	if err != nil {
		return nil, err
	}

	sourceFile, err := rr.j5Parser.ParseFile(sourceFilename, string(data))
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(data))
	}

	summary, err := j5convert.SourceSummary(sourceFile)
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(data))
	}

	return &SourceFile{
		Filename: sourceFilename,
		J5Data:   data,
		J5Source: sourceFile,
		Summary:  summary,
	}, nil
}
