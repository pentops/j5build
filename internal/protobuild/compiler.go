package protobuild

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/pentops/bcl.go/internal/j5convert"
)

type Package struct {
	Files              []*SourceFile
	DirectDependencies map[string]*Package

	Exports map[string]*j5convert.TypeRef
}

func (pkg *Package) ResolveType(pkgName string, name string) (*j5convert.TypeRef, error) {
	pkg, ok := pkg.DirectDependencies[pkgName]
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

func (c *Compiler) FindFileByPath(filename string) (protocompile.SearchResult, error) {
	pkgName := j5convert.PackageFromFilename(filename)
	pkg, ok := c.Packages[pkgName]
	if !ok {
		return protocompile.SearchResult{}, fmt.Errorf("package %s not found", pkgName)
	}

	if strings.HasSuffix(filename, ".j5s.proto") {
		filename = strings.TrimSuffix(filename, ".proto")
	}

	for _, file := range pkg.Files {
		if file.Filename == filename {
			if file.Result == nil {
				return protocompile.SearchResult{}, fmt.Errorf("file %s not compiled", filename)
			}
			return *file.Result, nil
		}
	}

	return protocompile.SearchResult{}, fmt.Errorf("file %s not found in package %s", filename, pkgName)

}

func (c *Compiler) loadPackage(ctx context.Context, name string, chain []string) (*Package, error) {

	log.Printf("loading package %s, chain %s", name, strings.Join(chain, " -> "))

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
		return nil, err
	}

	pkg := &Package{
		Files:              files,
		DirectDependencies: map[string]*Package{},
		Exports:            map[string]*j5convert.TypeRef{},
	}

	for _, file := range files {
		if file.Summary == nil {
			return nil, fmt.Errorf("file %s has no summary", file.Filename)
		}
		for _, exp := range file.Summary.Exports {
			log.Printf("file %s exports %s as %s", file.Filename, exp.Name, exp.File)
			pkg.Exports[exp.Name] = exp
		}
	}

	depPackages := listDependencies(files)
	for dep := range depPackages {
		depPkg, err := c.loadPackage(ctx, dep, append(chain, name))
		if err != nil {
			return nil, err
		}
		pkg.DirectDependencies[dep] = depPkg
	}

	c.Packages[name] = pkg

	for _, file := range files {
		if file.Result == nil {
			if strings.HasSuffix(file.Filename, ".j5s") {
				searchResult, err := j5convert.ConvertJ5File(pkg, file.J5Source)
				if err != nil {
					return nil, err
				}
				file.Result = &protocompile.SearchResult{
					Proto: searchResult,
				}
			} else {
				return nil, fmt.Errorf("unknown file type: %s", file.Filename)
			}
		}

	}

	return pkg, nil
}

func (c *Compiler) CompilePackage(ctx context.Context, packageName string) (linker.Files, error) {
	pkg, err := c.loadPackage(ctx, packageName, nil)
	if err != nil {
		return nil, err
	}

	cc := protocompile.Compiler{
		Resolver:       c,
		MaxParallelism: 1,
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter:       c.Resolver.reporter,
	}

	filenames := make([]string, len(pkg.Files))
	for idx, file := range pkg.Files {
		if strings.HasSuffix(file.Filename, ".j5s") {
			filenames[idx] = file.Filename + ".proto"
		} else {
			filenames[idx] = file.Filename
		}
	}

	files, err := cc.Compile(ctx, filenames...)
	if err != nil {
		return nil, err
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
