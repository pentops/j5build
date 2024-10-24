package protobuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"github.com/pentops/log.go/log"
	"golang.org/x/exp/maps"
)

type SourceFile struct {
	Filename string
	Summary  *j5convert.FileSummary

	J5Source  *sourcedef_j5pb.SourceFile
	RawSource []byte

	Result *SearchResult
}

type Package struct {
	Name        string
	SourceFiles []*SourceFile

	Files              map[string]*SearchResult
	DirectDependencies map[string]*Package
	Exports            map[string]*j5convert.TypeRef
}

func newPackage(name string) *Package {
	pkg := &Package{
		Name:               name,
		DirectDependencies: map[string]*Package{},
		Exports:            map[string]*j5convert.TypeRef{},
		Files:              map[string]*SearchResult{},
	}
	return pkg
}

func (pkg *Package) ResolveType(pkgName string, name string) (*j5convert.TypeRef, error) {
	if pkgName == pkg.Name {
		gotType, ok := pkg.Exports[name]
		if ok {
			return gotType, nil
		}
		return nil, &j5convert.TypeNotFoundError{
			// no package, is own package.
			Name: name,
		}
	}

	pkg, ok := pkg.DirectDependencies[pkgName]
	if !ok {
		return nil, fmt.Errorf("ResolveType: package %s not loaded", pkgName)
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

type Compiler struct {
	Resolver      *Resolver
	localResolver *sourceResolver

	Packages map[string]*Package
}

func NewCompiler(deps DependencySet, localFiles LocalFileSource) (*Compiler, error) {

	resolver, err := newResolver(deps)
	if err != nil {
		return nil, fmt.Errorf("newResolver: %w", err)
	}

	sourceResolver, err := newSourceResolver(localFiles)
	if err != nil {
		return nil, fmt.Errorf("newSourceResolver: %w", err)
	}

	cc := &Compiler{
		Resolver:      resolver,
		localResolver: sourceResolver,
		Packages:      map[string]*Package{},
	}
	return cc, nil
}

func (c *Compiler) findFileByPath(ctx context.Context, filename string) (*SearchResult, error) {
	if filename == "" {
		return nil, errors.New("empty filename")
	}

	pkgName, isLocal, err := c.localResolver.packageForFile(filename)
	if err != nil {
		return nil, fmt.Errorf("packageForFile: %w", err)
	}
	if !isLocal {
		file, err := c.Resolver.readFile(ctx, filename)
		if err != nil {
			return nil, fmt.Errorf("readFile: %w", err)
		}
		return file, nil
	}

	pkg, ok := c.Packages[pkgName]
	if !ok {
		return nil, fmt.Errorf("package %s not found for file %q", pkgName, filename)
	}

	res, ok := pkg.Files[filename]
	if ok {
		return res, nil
	}

	return nil, fmt.Errorf("file %s not found in package %s, have %s", filename, pkgName, strings.Join(maps.Keys(pkg.Files), ", "))

}

func (c *Compiler) loadPackage(ctx context.Context, name string, chain []string) (*Package, error) {
	ctx = log.WithField(ctx, "loadPackage", name)
	for _, ancestor := range chain {
		if name == ancestor {
			return nil, NewCircularDependencyError(chain, name)
		}
	}

	pkg, ok := c.Packages[name]
	if ok {
		return pkg, nil
	}

	var err error
	if c.localResolver.isLocalPackage(name) {
		pkg, err = c.loadLocalPackage(ctx, name, chain)
		if err != nil {
			return nil, fmt.Errorf("loadLocalPackage %s: %w", name, err)
		}
	} else {
		pkg, err = c.loadExternalPackage(ctx, name, chain)
		if err != nil {
			return nil, fmt.Errorf("loadExternalPackage %s: %w", name, err)
		}
	}
	c.Packages[name] = pkg
	return pkg, nil
}

func (pkg *Package) includeIO(summary *j5convert.FileSummary, deps map[string]struct{}) {
	for _, exp := range summary.Exports {
		pkg.Exports[exp.Name] = exp
	}

	for _, ref := range summary.TypeDependencies {
		deps[ref.Package] = struct{}{}
	}
	for _, file := range summary.FileDependencies {
		dependsOn := j5convert.PackageFromFilename(file)
		deps[dependsOn] = struct{}{}
	}
}

func (c *Compiler) resolveDependencies(ctx context.Context, pkg *Package, deps map[string]struct{}, chain []string) error {
	delete(deps, pkg.Name)
	pkg.DirectDependencies = map[string]*Package{}
	for dep := range deps {
		depPkg, err := c.loadPackage(ctx, dep, append(chain, pkg.Name))
		if err != nil {
			return fmt.Errorf("loadPackage %s: %w", dep, err)
		}
		pkg.DirectDependencies[dep] = depPkg
	}
	return nil
}

func (c *Compiler) loadLocalPackage(ctx context.Context, name string, chain []string) (*Package, error) {

	fileNames, err := c.localResolver.listPackageFiles(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("package files for %s: %w", name, err)
	}

	pkg := newPackage(name)

	deps := map[string]struct{}{}
	for _, filename := range fileNames {
		file, err := c.localResolver.getFile(ctx, filename)
		if err != nil {
			return nil, fmt.Errorf("GetLocalFile %s: %w", filename, err)
		}
		pkg.SourceFiles = append(pkg.SourceFiles, file)
		pkg.includeIO(file.Summary, deps)
	}

	err = c.resolveDependencies(ctx, pkg, deps, chain)
	if err != nil {
		return nil, fmt.Errorf("resolveDependencies for %s: %w", name, err)
	}

	return pkg, nil
}

func (c *Compiler) loadExternalPackage(ctx context.Context, name string, chain []string) (*Package, error) {

	files, err := c.Resolver.PackageFiles(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("package files for %s: %w", name, err)
	}

	pkg := newPackage(name)

	deps := map[string]struct{}{}
	for _, file := range files {
		pkg.Files[file.Filename] = file
		pkg.includeIO(file.Summary, deps)
	}

	err = c.resolveDependencies(ctx, pkg, deps, chain)
	if err != nil {
		return nil, fmt.Errorf("resolveDependencies for %s: %w", name, err)
	}

	return pkg, nil
}

type ErrCollector struct {
	Errors   []*errpos.Err //reporter.ErrorWithPos
	Warnings []*errpos.Err //.ErrorWithPos
}

func convertError(err reporter.ErrorWithPos) *errpos.Err {
	start := err.Start()
	end := err.End()
	cause := err.Unwrap()

	return &errpos.Err{
		Pos: &errpos.Position{
			Start: errpos.Point{
				Line:   start.Line - 1,
				Column: start.Col - 1,
			},
			End: errpos.Point{
				Line:   end.Line - 1,
				Column: end.Col - 1,
			},
		},
		Err: cause,
	}
}

func (ec *ErrCollector) Error(err reporter.ErrorWithPos) error {
	ec.Errors = append(ec.Errors, convertError(err))
	return err
}

func (ec *ErrCollector) Warning(err reporter.ErrorWithPos) {
	ec.Warnings = append(ec.Warnings, convertError(err))
}

func (c *Compiler) LintAll(ctx context.Context) (*errpos.ErrorsWithSource, error) {
	allPackages := c.localResolver.listPackages()
	errs := &ErrCollector{}
	cc := searchLinker{
		symbols:  &linker.Symbols{},
		resolver: c,
		Reporter: errs,
	}

	for _, pkgName := range allPackages {
		// LoadLocalPackage parses both BCL and Proto files, but does not fully link.
		pkg, err := c.loadLocalPackage(ctx, pkgName, nil)
		if err != nil {
			if ep, ok := errpos.AsErrorsWithSource(err); ok {
				return ep, nil
			}
			return nil, fmt.Errorf("loadLocalPackage %s: %w", pkgName, err)
		}

		for _, srcFile := range pkg.SourceFiles {
			if srcFile.Result != nil {
				pkg.Files[srcFile.Filename] = srcFile.Result
			} else if srcFile.J5Source != nil {
				descs, err := j5convert.ConvertJ5File(pkg, srcFile.J5Source)
				if err != nil {
					if ep, ok := errpos.AsErrorsWithSource(err); ok {
						return ep, nil
					}
					return nil, fmt.Errorf("convertJ5File %s: %w", srcFile.Filename, err)
				}

				for _, desc := range descs {
					pkg.Files[srcFile.Filename] = &SearchResult{
						Filename: srcFile.Filename,
						Summary:  srcFile.Summary,
						Desc:     desc,
					}
				}
			} else {
				return nil, fmt.Errorf("source file %s has no result and is not j5s", srcFile.Filename)
			}
		}

		for _, file := range pkg.Files {
			_, err := cc.link(ctx, file)
			if err != nil {
				return nil, fmt.Errorf("linking file %s: %w", file.Filename, err)
			}
		}

	}

	return convertLintErrors("", "", errs)
}

func (c *Compiler) LintFile(ctx context.Context, filename string, fileData string) (*errpos.ErrorsWithSource, error) {
	pkgName, ok, err := c.localResolver.packageForFile(filename)
	if err != nil {
		return nil, fmt.Errorf("packageForFile %s: %w", filename, err)
	}
	if !ok {
		return nil, fmt.Errorf("file %s is not a local bundle file", filename)
	}

	// LoadLocalPackage parses both BCL and Proto files, but does not fully link.
	pkg, err := c.loadLocalPackage(ctx, pkgName, nil)
	if err != nil {
		if ep, ok := errpos.AsErrorsWithSource(err); ok {
			return ep, nil
		}
		return nil, fmt.Errorf("loadLocalPackage %s: %w", pkgName, err)
	}

	c.Packages[pkgName] = pkg

	var sourceFile *SourceFile

	for _, search := range pkg.SourceFiles {
		if search.Filename == filename {
			sourceFile = search
			break
		}
	}
	if sourceFile == nil {
		return nil, fmt.Errorf("source file %s not found in package %s", filename, pkgName)
	}

	errs := &ErrCollector{}
	cc := searchLinker{
		symbols:  &linker.Symbols{},
		resolver: c,
		Reporter: errs,
	}

	for _, srcFile := range pkg.SourceFiles {
		if srcFile.Result != nil {
			pkg.Files[srcFile.Filename] = srcFile.Result

			if srcFile.Filename == filename {
				_, err = cc.link(ctx, srcFile.Result)
				if err != nil {
					return nil, fmt.Errorf("linking proto file %s: %w", filename, err)
				}
			}
		} else if srcFile.J5Source != nil {
			descs, err := j5convert.ConvertJ5File(pkg, srcFile.J5Source)
			if err != nil {
				if srcFile.Filename == filename {
					if ep, ok := errpos.AsErrorsWithSource(err); ok {
						return ep, nil
					}
				}
				return nil, fmt.Errorf("convertJ5File %s: %w", srcFile.Filename, err)
			}

			for _, desc := range descs {
				sr := &SearchResult{
					Filename: srcFile.Filename,
					Summary:  srcFile.Summary,
					Desc:     desc,
				}
				pkg.Files[desc.GetName()] = sr
				if srcFile.Filename == filename {
					_, err = cc.link(ctx, sr)
					if err != nil {
						return nil, fmt.Errorf("linking j5 file %s: %w", filename, err)
					}
				}
			}
		} else {
			return nil, fmt.Errorf("source file %s has no result and is not j5s", srcFile.Filename)
		}
	}

	return convertLintErrors(filename, fileData, errs, sourceFile.Summary.Warnings...)
}

func convertLintErrors(filename string, fileData string, errs *ErrCollector, extra ...*errpos.Err) (*errpos.ErrorsWithSource, error) {

	errors := errpos.Errors{}
	for _, err := range errs.Errors {
		errors = append(errors, err)
	}
	for _, err := range errs.Warnings {
		errors = append(errors, err)
	}

	if len(errors) == 0 {
		return nil, nil
	}

	ws := errpos.AddSourceFile(errors, filename, fileData)
	as, ok := errpos.AsErrorsWithSource(ws)
	if !ok {
		return nil, fmt.Errorf("error not valid for source: (%T) %w", ws, ws)
	}
	return as, nil
}

func (c *Compiler) CompilePackage(ctx context.Context, packageName string) (linker.Files, error) {
	pkg, err := c.loadPackage(ctx, packageName, nil)
	if err != nil {
		return nil, fmt.Errorf("loadPackage %s: %w", packageName, err)
	}

	filenames := make([]string, 0)
	for filename := range pkg.Files {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames) // for consistent error ordering

	ctx = log.WithField(ctx, "CompilePackage", packageName)

	errs := &ErrCollector{}
	cc := searchLinker{
		symbols:  &linker.Symbols{},
		resolver: c,
		Reporter: errs,
	}

	lf := make(linker.Files, 0, len(filenames))
	for _, file := range filenames {
		f, err := cc.resolve(ctx, file)
		if err != nil {
			return nil, err
		}
		lf = append(lf, f)
	}

	return lf, nil
}

func listDependencies(files []*SearchResult) map[string]struct{} {
	depPackages := map[string]struct{}{}
	for _, file := range files {
		for _, ref := range file.Summary.TypeDependencies {
			depPackages[ref.Package] = struct{}{}
		}
		for _, file := range file.Summary.FileDependencies {
			pkg := j5convert.PackageFromFilename(file)
			depPackages[pkg] = struct{}{}
		}

	}
	return depPackages
}

func NewCircularDependencyError(chain []string, dep string) error {
	return &CircularDependencyError{
		Chain: chain,
		Dep:   dep,
	}
}

type CircularDependencyError struct {
	Chain []string
	Dep   string
}

func (e *CircularDependencyError) Error() string {
	return fmt.Sprintf("circular dependency detected: %s -> %s", strings.Join(e.Chain, " -> "), e.Dep)
}
