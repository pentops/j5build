package j5convert

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/internal/bcl/sourcewalk"
)

type rootContext interface {
	resolveTypeNoImport(ref *schema_j5pb.Ref) (*TypeRef, error)
	addError(error)
	sourceFor(path []string) *bcl_j5pb.SourceLocation
	sourcePosition(path []string) *errpos.Position
	subPackageFile(string) fileContext
}

type Root struct {
	packageName string
	deps        Package
	source      sourceLink
	errors      []error

	importAliases *importMap

	mainFile *FileBuilder
	files    []*FileBuilder
}

func newRoot(deps Package, imports *importMap, file *FileBuilder, source *bcl_j5pb.SourceLocation) *Root {
	return &Root{
		packageName:   file.fdp.GetPackage(),
		deps:          deps,
		source:        sourceLink{root: source},
		mainFile:      file,
		importAliases: imports,
		files:         []*FileBuilder{file},
	}
}

var _ rootContext = &Root{}

func (rr *Root) subPackageFile(subPackage string) fileContext {
	fullPackage := fmt.Sprintf("%s.%s", rr.packageName, subPackage)

	for _, search := range rr.files {
		if search.fdp.GetPackage() == fullPackage {
			return search
		}
	}

	rootName := *rr.mainFile.fdp.Name
	dirName, baseName := path.Split(rootName)

	baseRoot := strings.TrimSuffix(baseName, ".j5s.proto")
	newBase := fmt.Sprintf("%s.p.j5s.proto", baseRoot)

	subName := path.Join(dirName, subPackage, newBase)
	found := newFileBuilder(subName)

	found.fdp.Package = &fullPackage
	rr.files = append(rr.files, found)
	return found
}

func (rr *Root) addError(err error) {
	rr.errors = append(rr.errors, err)
}

func (rr *Root) sourceFor(path []string) *bcl_j5pb.SourceLocation {
	return rr.source.getSource(path)
}

func (rr *Root) sourcePosition(path []string) *errpos.Position {
	return rr.source.getPos(path)
}

type fileContext interface {
	parentContext
	ensureImport(string)
	addService(*ServiceBuilder)
}

type parentContext interface {
	addMessage(*MessageBuilder)
	addEnum(*EnumBuilder)
}

type fieldContext struct {
	name string
}

type walkContext struct {
	root          rootContext
	file          fileContext
	field         *fieldContext
	parentContext parentContext
}

func (ww *walkContext) _clone() *walkContext {
	return &walkContext{
		root:          ww.root,
		file:          ww.file,
		field:         ww.field,
		parentContext: ww.parentContext,
	}
}

/*
func (ww *walkContext) at(path ...string) *walkContext {
	walk := ww._clone()
	walk.path = append(ww.path, path...)
	return walk
}*/

func (ww *walkContext) inMessage(msg *MessageBuilder) *walkContext {
	walk := ww._clone()
	walk.parentContext = msg
	walk.field = nil
	return walk
}

func (ww *walkContext) subPackageFile(subPackage string) *walkContext {
	file := ww.root.subPackageFile(subPackage)
	walk := ww._clone()
	walk.file = file
	walk.parentContext = file
	return walk
}

func (ww *walkContext) resolveType(ref *sourcewalk.RefNode) (*TypeRef, error) {
	if ref == nil {
		return nil, fmt.Errorf("missing ref")
	}

	if ref.Inline {
		// Inline conversions will already exist, they were converted from

		if ref.InlineEnum != nil {
			return enumTypeRef(ref.InlineEnum), nil
		} else if ref.InlineOneof != nil {
			return oneofTypeRef(ref.InlineOneof), nil
		} else if ref.InlineObject != nil {
			return objectTypeRef(ref.InlineObject), nil
		} else {
			return nil, fmt.Errorf("unhandled inline conversion")
		}
	}

	typeRef, err := ww.root.resolveTypeNoImport(ref.Ref)
	if err != nil {
		pos := ref.Source.GetPos()
		if pos != nil {
			err = errpos.AddPosition(err, *pos)
		}
		log.Printf("resolveType error at %s: %v", strings.Join(ref.Source.Path, "."), err)
		return nil, err
	}

	ww.file.ensureImport(typeRef.File)
	return typeRef, nil
}

func (ww *walkContext) errorf(node sourcewalk.SourceNode, format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	ww.error(node, err)
}

func (ww *walkContext) error(node sourcewalk.SourceNode, err error) {
	loc := node.GetPos()
	if loc != nil {
		err = errpos.AddPosition(err, *loc)
	}
	log.Printf("walker error at %s: %v", strings.Join(node.Path, "."), err)
	ww.root.addError(err)
}
