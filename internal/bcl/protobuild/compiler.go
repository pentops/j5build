package protobuild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"github.com/pentops/j5build/internal/protosrc"
)

type Package struct {
	Name               string
	SourceFiles        []*SourceFile
	BuildFiles         map[string]protocompile.SearchResult
	DirectDependencies map[string]*Package

	Exports map[string]*j5convert.TypeRef
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
	Resolver *Resolver
	Packages map[string]*Package
}

func NewCompiler(resolver *Resolver) *Compiler {
	cc := &Compiler{
		Resolver: resolver,
		Packages: map[string]*Package{},
	}

	return cc
}

// FindFileByPath implements protocompile.Resolver
func (c *Compiler) FindFileByPath(filename string) (protocompile.SearchResult, error) {
	if filename == "" {
		return protocompile.SearchResult{}, errors.New("FindFileByPath: empty filename")
	}

	if protosrc.IsBuiltInProto(filename) {
		return c.Resolver.GetInbuilt(filename)
	}

	pkgName, _, err := j5convert.SplitPackageFromFilename(filename)
	if err != nil {
		return protocompile.SearchResult{}, err
	}

	pkg, ok := c.Packages[pkgName]
	if !ok {
		return protocompile.SearchResult{}, fmt.Errorf("FindFileByPath: package %s not found", pkgName)
	}

	res, ok := pkg.BuildFiles[filename]
	if ok {
		return res, nil
	}

	return protocompile.SearchResult{}, fmt.Errorf("FindPackageByPath: file %s not found in package %s", filename, pkgName)

}

func (c *Compiler) loadPackage(ctx context.Context, name string, chain []string) (*Package, error) {

	for _, ancestor := range chain {
		if name == ancestor {
			return nil, NewCircularDependencyError(chain, name)
		}
	}

	if pkg, ok := c.Packages[name]; ok {
		return pkg, nil
	}

	files, err := c.Resolver.PackageFiles(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("package files for %s: %w", name, err)
	}

	pkg := &Package{
		Name:               name,
		SourceFiles:        files,
		DirectDependencies: map[string]*Package{},
		Exports:            map[string]*j5convert.TypeRef{},
	}

	for _, file := range files {
		if file.Summary == nil {
			return nil, fmt.Errorf("file %s has no summary", file.Filename)
		}
		for _, exp := range file.Summary.Exports {
			pkg.Exports[exp.Name] = exp
		}
	}

	depPackages := listDependencies(files)
	for dep := range depPackages {
		if dep == name {
			continue
		}
		depPkg, err := c.loadPackage(ctx, dep, append(chain, name))
		if err != nil {
			return nil, fmt.Errorf("loadPackage %s: %w", dep, err)
		}
		pkg.DirectDependencies[dep] = depPkg
	}

	c.Packages[name] = pkg

	pkg.BuildFiles = map[string]protocompile.SearchResult{}
	for _, file := range files {
		if file.Result != nil {
			pkg.BuildFiles[file.Filename] = *file.Result
			continue
		}
		if !strings.HasSuffix(file.Filename, ".j5s") {
			return nil, fmt.Errorf("file %s has no result", file.Filename)
		}

		builtFiles, err := j5convert.ConvertJ5File(pkg, file.J5Source)
		if err != nil {
			return nil, errpos.AddFilename(err, file.Filename)
		}

		for _, file := range builtFiles {

			pkg.BuildFiles[*file.Name] = protocompile.SearchResult{
				Proto: file,
			}
		}
	}

	return pkg, nil
}

func (c *Compiler) CompilePackage(ctx context.Context, packageName string) (linker.Files, error) {
	pkg, err := c.loadPackage(ctx, packageName, nil)
	if err != nil {
		return nil, fmt.Errorf("loadPackage %s: %w", packageName, err)
	}

	cc := protocompile.Compiler{
		Resolver:       protocompile.WithStandardImports(c),
		MaxParallelism: 1,
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter:       c.Resolver.reporter,
	}

	filenames := make([]string, 0)
	for filename := range pkg.BuildFiles {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames) // for consistent error ordering

	files, err := cc.Compile(ctx, filenames...)
	if err != nil {
		panicErr := protocompile.PanicError{}
		if ok := errors.As(err, &panicErr); ok {
			log.Printf("STACK\n%s", panicErr.Stack)
			return nil, panicErr
		}
		return nil, fmt.Errorf("compile package %s: %w", packageName, err)
	}

	return files, nil
}

func listDependencies(files []*SourceFile) map[string]struct{} {
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
