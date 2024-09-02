package protobuild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/jhump/protoreflect/desc/sourceinfo"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/j5convert"
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
	//compiler protocompile.Compiler

	protoParser *ProtoParser
	j5Parser    *J5Parser

	packages map[string]*j5convert.PackageSummary
}

func NewResolver(externalDeps DependencySet, localFiles LocalFileSource) (*Resolver, error) {
	packages := localFiles.ListPackages()
	packagePrefixes := make([]string, len(packages))
	for i, p := range packages {
		s := strings.ReplaceAll(p, ".", "/")
		packagePrefixes[i] = s + "/"

	}

	rr := &Resolver{
		ExternalDeps:  externalDeps,
		BundleFiles:   localFiles,
		localPrefixes: packagePrefixes,
		packages:      map[string]*j5convert.PackageSummary{},
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

	jp, err := NewJ5Parser()
	if err != nil {
		return nil, err
	}
	rr.j5Parser = jp
	/*
		rr.compiler = protocompile.Compiler{
			Resolver:       rr,
			MaxParallelism: 1,
			SourceInfoMode: protocompile.SourceInfoStandard,
			Reporter:       rr.reporter,
		}*/

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
	"j5/ext/",
}

func hasAPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func (rr *Resolver) findFileByPath(filename string) (*SourceFile, error) {
	res, err := rr.readFile(filename)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (rr *Resolver) readFile(filename string) (*SourceFile, error) {
	if hasAPrefix(filename, inbuiltPrefixes) {
		desc, err := sourceinfo.GlobalFiles.FindFileByPath(filename)
		if err != nil {
			return nil, err
		}
		return &SourceFile{
			Filename: filename,
			Result: &protocompile.SearchResult{
				Desc: desc,
			},
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
	return &SourceFile{
		Filename: filename,
		Result:   &protocompile.SearchResult{Proto: file},
	}, nil
}

/*
// FindFileByPath implements protocompile.Resolver

	func (rr *Resolver) FindFileByPath(filename string) (search protocompile.SearchResult, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in FindFileByPath: %v", r)
				fmt.Println(err)
				fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
			}
		}()

		result, err := rr.findFileByPath(filename)
		if err != nil {
			return protocompile.SearchResult{}, err
		}

		if result.Result != nil {
			return *result.Result, nil
		}

		if result.J5Source != nil {
			return *result.Result, nil
		}

		err = fmt.Errorf("unknown result for: %s", filename)
		return
	}

	func (rr *Resolver) getFileInfo(ctx context.Context, filename string) (*j5convert.FileSummary, error) {
		result, err := rr.findFileByPath(filename)
		if err != nil {
			return nil, err
		}

		if result.Summary != nil {
			return result.Summary, nil
		}

		if result.Result != nil && result.Result.Proto != nil {
			summary, err := rr.protoParser.buildSummaryFromDescriptor(result.Result.Proto)
			if err != nil {
				return nil, err
			}
			result.Summary = summary
			return summary, nil
		}

		if result.J5Source != nil {
			summary, err := j5convert.SourceSummary(result.J5Source)
			if err != nil {
				return nil, err
			}
			return summary, nil
		}

		return nil, fmt.Errorf("unknown result for source info: %s", filename)
	}
*/
func (rr *Resolver) PackageFiles(ctx context.Context, pkgName string) ([]*SourceFile, error) {
	filenames, err := rr.listPackageFiles(ctx, pkgName)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("cannot list package files for inbuilt package %s", pkgName)
	} else if hasAPrefix(root+"/", rr.localPrefixes) {
		return rr.BundleFiles.ListSourceFiles(ctx, root)
	}

	return rr.ExternalDeps.ListDependencyFiles(root), nil
}

/*
func (rr *Resolver) buildPackageSummary(ctx context.Context, pkgName string) (*j5convert.PackageSummary, error) {
	rr.Logf("build package summary for %s", pkgName)

	summary := &j5convert.PackageSummary{
		Exports: map[string]*j5convert.TypeRef{},
	}

	filenames, err := rr.listPackageFiles(ctx, pkgName)
	if err != nil {
		return nil, err
	}

	for _, filename := range filenames {
		fileSummary, err := rr.getFileInfo(ctx, filename)
		if err != nil {
			return nil, err
		}
		summary.Files = append(summary.Files, fileSummary)
		for k, v := range fileSummary.Exports {
			v.Package = pkgName
			if other, ok := summary.Exports[k]; ok {
				return nil, fmt.Errorf("duplicate type %s in package %s (%s and %s)", k, pkgName, v.File, other.File)
			}
			summary.Exports[k] = v
			rr.Logf("export %s.%s", pkgName, k)
		}
	}

	return summary, nil
}
*/

func (rr *Resolver) localFile(sourceFilename string) (*SourceFile, error) {
	ctx := context.Background()

	if strings.HasSuffix(sourceFilename, ".j5s") {
		return rr.ParseToSource(ctx, sourceFilename)
	}

	if strings.HasSuffix(sourceFilename, ".proto") {
		data, err := rr.BundleFiles.GetLocalFile(ctx, sourceFilename)
		if err != nil {
			return nil, err
		}
		desc, err := rr.protoParser.protoToDescriptor(sourceFilename, data)
		if err != nil {
			return nil, err
		}
		summary, err := rr.protoParser.buildSummaryFromDescriptor(desc)
		if err != nil {
			return nil, err
		}
		return &SourceFile{
			Filename: sourceFilename,
			Summary:  summary,
			Result:   &protocompile.SearchResult{Proto: desc},
		}, nil
	}

	return nil, fmt.Errorf("unknown file type for proto compile: %s", sourceFilename)
}

func (rr *Resolver) ParseToSource(ctx context.Context, sourceFilename string) (*SourceFile, error) {

	data, err := rr.BundleFiles.GetLocalFile(ctx, sourceFilename)
	if err != nil {
		return nil, err
	}

	sourceFile, err := rr.j5Parser.parseToSourceDescriptor(sourceFilename, data)
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

/*
func (rr *Resolver) ParseToDescriptor(ctx context.Context, sourceFilename string) (*descriptorpb.FileDescriptorProto, error) {

	file, err := rr.findFileByPath(sourceFilename)
	if err != nil {
		return nil, err
	}

	deps, err := rr.prepareDependencies(ctx, file.Summary)
	if err != nil {
		return nil, err
	}

	protoFile, err := j5convert.ConvertJ5File(deps, file.J5Source)
	if err != nil {
		return nil, errpos.AddSourceFile(err, sourceFilename, string(file.J5Data))
	}
	file.Result = &protocompile.SearchResult{Proto: protoFile}
	rr.Logf("parsed %s to descriptor", sourceFilename)

	return protoFile, nil
}*/

/*
type PackageDepSet map[string]*j5convert.PackageSummary

func (pds PackageDepSet) ResolveType(pkgName string, name string) (*j5convert.TypeRef, error) {
	pkg, ok := pds[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %s not found", pkgName)
	}

	gotType, ok := pkg.Exports[name]
	if ok {
		return gotType, nil
	}

	return nil, &j5convert.TypeNotFoundError{
		Package: pkgName,
		Name:    name,
	}
}
*/
/*
func (rr *Resolver) prepareDependencies(ctx context.Context, summary *j5convert.FileSummary) (PackageDepSet, error) {
	depPackages := map[string]struct{}{}
	for _, ref := range summary.TypeDependencies {
		rr.Logf("%s depends on %s.%s", summary.Package, ref.Package, ref.Schema)
		depPackages[ref.Package] = struct{}{}
	}
	for _, file := range summary.FileDependencies {
		pkg := j5convert.PackageFromFilename(file)
		depPackages[pkg] = struct{}{}
	}

	summaries := map[string]*j5convert.PackageSummary{}
	for pkgName := range depPackages {
		pkg, err := rr.buildPackageSummary(ctx, pkgName)
		if err != nil {
			return nil, err
		}
		summaries[pkgName] = pkg
	}
	return PackageDepSet(summaries), nil
}*/
