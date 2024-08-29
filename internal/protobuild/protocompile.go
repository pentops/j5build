package protobuild

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc/sourceinfo"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ProtoParser struct {
	reporter reporter.Reporter
}

func NewProtoParser() *ProtoParser {

	errs := func(err reporter.ErrorWithPos) error {
		log.Println(err)
		return err
	}

	warnings := func(err reporter.ErrorWithPos) {
		log.Println(err)
	}

	rep := reporter.NewReporter(errs, warnings)
	return &ProtoParser{reporter: rep}
}

func (pp ProtoParser) ParseFile(filename string, data []byte) (parser.Result, error) {
	reportHandler := reporter.NewHandler(pp.reporter)

	fileNode, err := parser.Parse(filename, bytes.NewReader(data), reportHandler)
	if err != nil {
		return nil, err
	}
	result, err := parser.ResultFromAST(fileNode, true, reportHandler)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (ps *PackageSet) tryDep(path string) (protocompile.SearchResult, error) {
	ps.Logf("Try Dep: %s", path)
	file, err := ps.Deps.GetFile(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Proto: file}, nil
}

func (ps *PackageSet) FindFileByPath(filename string) (protocompile.SearchResult, error) {
	ps.Logf("Find File: %s", filename)

	for _, prefix := range []string{"google/protobuf/", "google/api/", "buf/validate/", "j5/types"} {
		if strings.HasPrefix(filename, prefix) {
			desc, err := sourceinfo.GlobalFiles.FindFileByPath(filename)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Desc: desc}, nil
		}
	}

	packageName := packageFromFilename(filename)
	if pkg, ok := ps.Packages[packageName]; ok {
		for _, file := range pkg.Files {
			ps.Logf("File: %s", file.ProtoFilename)
			if file.ProtoFilename == filename {
				if file.SearchResult == nil {
					return protocompile.SearchResult{}, fmt.Errorf("SEARCH RESULT not built: %s", filename)
				}
				return *file.SearchResult, nil
			}
		}
	}
	return ps.tryDep(filename)

}

func (ps *PackageSet) BuildDescriptors(ctx context.Context, filenames []string) ([]protoreflect.FileDescriptor, error) {

	errs := func(err reporter.ErrorWithPos) error {
		log.Println(err)
		return err
	}

	warnings := func(err reporter.ErrorWithPos) {
		log.Println(err)
	}

	rep := reporter.NewReporter(errs, warnings)

	compiler := protocompile.Compiler{
		Resolver:       ps,
		MaxParallelism: 1,
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter:       rep,
	}

	if len(filenames) == 0 {
		filenames = make([]string, 0)
		for _, pkg := range ps.Packages {
			for _, file := range pkg.Files {
				if file.J5 != nil {
					filenames = append(filenames, file.ProtoFilename)
				}
			}
		}
	}

	results, err := compiler.Compile(ctx, filenames...)
	if err != nil {
		return nil, err
	}

	resultDescriptors := make([]protoreflect.FileDescriptor, len(results))
	for i, result := range results {
		resultDescriptors[i] = result
	}

	return resultDescriptors, nil

}
