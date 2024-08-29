package protobuild

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/parser"
	"github.com/pentops/bcl.go/internal/j5convert"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5source"
)

type BasicType int

const (
	BasicTypeMessage BasicType = iota
	BasicTypeEnum

	PartialTypeField
	PartialTypeMessage
)

func (bt BasicType) String() string {
	switch bt {
	case BasicTypeMessage:
		return "message"
	case BasicTypeEnum:
		return "enum"
	default:
		return fmt.Sprintf("unknown(%d)", bt)
	}
}

type Export struct {
	Type BasicType
	Name string
	File *SourceFile
}

type Package struct {
	root     *PackageSet
	Name     string
	Files    []*SourceFile
	Exported map[string]*Export
	Deps     map[string]*Package
}

func (pkg *Package) addFile(file *SourceFile) error {
	pkg.Files = append(pkg.Files, file)
	for _, exp := range file.ExportsTypes {
		name := exp.Name
		if _, ok := pkg.Exported[name]; ok {
			return fmt.Errorf("duplicate export %s in package %s", name, pkg.Name)
		}
		pkg.Exported[name] = exp
	}

	for _, dep := range file.PackageDeps {
		if _, ok := pkg.Deps[dep]; ok {
			// package already required
			continue
		}
		depPackage := pkg.root.getOrCreatePackage(dep)
		if err := checkDependencyLoop(pkg.Name, depPackage, []string{}); err != nil {
			return err
		}
		pkg.Deps[dep] = depPackage

	}
	return nil
}

func checkDependencyLoop(thisPackageName string, dependsOn *Package, path []string) error {
	if thisPackageName == dependsOn.Name {
		return fmt.Errorf("package %s depends on itself %q", thisPackageName, path)
	}
	for _, dep := range dependsOn.Deps {
		if err := checkDependencyLoop(thisPackageName, dep, append(path, dep.Name)); err != nil {
			return err
		}
	}
	return nil
}

type PackageSet struct {
	Packages map[string]*Package

	Deps j5source.DependencySet
}

func New(deps j5source.DependencySet) *PackageSet {
	return &PackageSet{
		Packages: map[string]*Package{},
		Deps:     deps,
	}
}

func (ps *PackageSet) Logf(format string, args ...interface{}) {
	log.Printf("PS: "+format, args...)
}

func (ps *PackageSet) getOrCreatePackage(pkgName string) *Package {
	pkg, ok := ps.Packages[pkgName]
	if !ok {
		pkg = &Package{
			root:     ps,
			Name:     pkgName,
			Exported: map[string]*Export{},
			Deps:     map[string]*Package{},
		}
		ps.Packages[pkgName] = pkg
	}
	return pkg
}

func (ps *PackageSet) ConvertJ5() error {
	for _, pkg := range ps.Packages {
		for _, file := range pkg.Files {
			if file.J5 != nil {
				desc, err := j5convert.ConvertJ5File(pkg.Name, file.J5.SourceFile)
				if err != nil {
					return err
				}
				file.SearchResult = &protocompile.SearchResult{Proto: desc}
				file.ProtoFilename = desc.GetName()
				ps.Logf("Converted %s to %s", file.J5.SourceFile.Path, file.ProtoFilename)
			}
		}
	}
	return nil
}

func (ps *PackageSet) AddJ5File(file *sourcedef_j5pb.SourceFile) error {
	pkgName := file.Package
	if pkgName == "" {
		return fmt.Errorf("no package name in file %s", file.Path)
	}

	pkg := ps.getOrCreatePackage(pkgName)

	fi := &SourceFile{
		J5: &J5File{
			SourceFile: file,
		},
	}

	for _, dep := range file.Imports {
		fi.PackageDeps = append(fi.PackageDeps, dep.Path)
	}

	for _, child := range file.Elements {
		switch elem := child.Type.(type) {
		case *sourcedef_j5pb.RootElement_Enum:
			fi.addExport(elem.Enum.Name, BasicTypeEnum)
		case *sourcedef_j5pb.RootElement_Object:
			fi.addExport(elem.Object.Def.Name, BasicTypeMessage)
		case *sourcedef_j5pb.RootElement_Oneof:
			fi.addExport(elem.Oneof.Def.Name, BasicTypeMessage)
		case *sourcedef_j5pb.RootElement_Partial:
			switch pt := elem.Partial.Type.(type) {
			case *sourcedef_j5pb.Partial_Field_:
				fi.addExport(pt.Field.Def.Name, PartialTypeField)
			}
		}
	}

	return pkg.addFile(fi)
}

func packageFromFilename(filename string) string {
	dirName, _ := path.Split(filename)
	dirName = strings.TrimSuffix(dirName, "/")
	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	return pathPackage
}

func (ps *PackageSet) AddProtoFile(file parser.Result) error {
	sourceFilename := file.AST().Name()
	packageName := packageFromFilename(sourceFilename)

	pkg := ps.getOrCreatePackage(packageName)

	fds := file.FileDescriptorProto()
	fi := &SourceFile{
		ProtoFilename: sourceFilename,
		PB: &PBFile{
			Result: file,
		},
		SearchResult: &protocompile.SearchResult{ParseResult: file},
	}

	for _, dep := range fds.Dependency {
		depPackage := packageFromFilename(dep)
		if depPackage == packageName {
			continue
		}
		fi.PackageDeps = append(fi.PackageDeps, depPackage)
	}

	for _, msg := range fds.MessageType {
		fi.addExport(msg.GetName(), BasicTypeMessage)
	}

	for _, enum := range fds.EnumType {
		fi.addExport(enum.GetName(), BasicTypeEnum)
	}

	return pkg.addFile(fi)
}

type SourceFile struct {
	ProtoFilename string
	SearchResult  *protocompile.SearchResult
	PackageDeps   []string
	ExportsTypes  []*Export

	J5 *J5File
	PB *PBFile
}

func (sf *SourceFile) addExport(name string, t BasicType) {
	sf.ExportsTypes = append(sf.ExportsTypes, &Export{
		Name: name,
		File: sf,
		Type: t,
	})
}

type PBFile struct {
	parser.Result
}

type J5File struct {
	*sourcedef_j5pb.SourceFile
}

func sortMap[T any](m map[string]T) []T {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]T, 0, len(m))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return out
}
