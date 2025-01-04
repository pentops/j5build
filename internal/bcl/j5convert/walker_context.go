package j5convert

import (
	"fmt"
	"path"
	"strings"
)

type rootContext struct {
	packageName string
	deps        TypeResolver
	//source      sourceLink
	errors []error

	importAliases *importMap

	mainFile *fileContext
	files    []*fileContext
}

func newRootContext(deps TypeResolver, imports *importMap, file *fileContext) *rootContext {
	return &rootContext{
		packageName:   file.fdp.GetPackage(),
		deps:          deps,
		mainFile:      file,
		importAliases: imports,
		files:         []*fileContext{file},
	}
}

func subPackageFileName(sourceFilename, subPackage string) string {
	dirName, baseName := path.Split(sourceFilename)
	baseRoot := strings.TrimSuffix(baseName, ".j5s.proto")
	newBase := fmt.Sprintf("%s.p.j5s.proto", baseRoot)
	subName := path.Join(dirName, subPackage, newBase)
	return subName
}

func (rr *rootContext) subPackageFile(subPackage string) *fileContext {
	fullPackage := fmt.Sprintf("%s.%s", rr.packageName, subPackage)

	for _, search := range rr.files {
		if search.fdp.GetPackage() == fullPackage {
			return search
		}
	}
	rootName := *rr.mainFile.fdp.Name
	subName := subPackageFileName(rootName, subPackage)

	found := newFileContext(subName)

	found.fdp.Package = &fullPackage
	rr.files = append(rr.files, found)
	return found
}

// parentContext is a file's root, or message, which can hold messages and
// enums. Implemented by FileBuilder and MessageBuilder.
type parentContext interface {
	addMessage(*MessageBuilder)
	addEnum(*enumBuilder)
}

type fieldContext struct {
	name string
}
