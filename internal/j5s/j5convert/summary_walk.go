package j5convert

import (
	"fmt"
	"strings"

	"github.com/pentops/j5build/internal/bcl/errpos"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/j5s/sourcewalk"
)

type ErrCollector interface {
	WarnPos(pos *errpos.Position, err error)
}

// SourceSummary collects the exports and imports for a j5 source file
func SourceSummary(sourceFile *sourcedef_j5pb.SourceFile, ec ErrCollector) (*FileSummary, error) {

	cc := &summaryWalker{}
	err := cc.collectFileRefs(sourceFile)
	if err != nil {
		return nil, err
	}

	importPath := sourceFile.Path + ".proto"

	allFilenames := []string{importPath}
	for _, subPackage := range cc.subPackageFiles {
		allFilenames = append(allFilenames, subPackageFileName(sourceFile.Path, subPackage))
	}

	fs := &FileSummary{
		SourceFilename: sourceFile.Path,
		Package:        sourceFile.Package.Name,
		Exports:        make(map[string]*TypeRef),
		ProducesFiles:  allFilenames,
	}

	importMap, err := j5Imports(sourceFile)
	if err != nil {
		return nil, err
	}

	for _, refSrc := range cc.refs {
		expanded := importMap.expand(refSrc.Ref)
		if expanded == nil {
			err := fmt.Errorf("package %q not imported (for schema %s)", refSrc.Ref.Package, refSrc.Ref.Schema)
			err = errpos.AddContext(err, strings.Join(refSrc.Source.Path, "."))
			loc := refSrc.Source.GetPos()
			if loc != nil {
				err = errpos.AddPosition(err, *loc)
			}
			return nil, err
		}

		fs.TypeDependencies = append(fs.TypeDependencies, expanded.ref)
	}

	for _, export := range cc.exports {
		export.Package = sourceFile.Package.Name
		export.File = importPath
		fs.Exports[export.Name] = export
		//fmt.Printf("export from %s: %s\n", export.Package, export.Name)
	}

	for _, ref := range importMap.vals {
		if ref.used {
			continue
		}
		err := fmt.Errorf("import %q not used", ref.fullPath)
		var pos *errpos.Position
		if ref.source != nil {
			pos = &errpos.Position{
				Start: errpos.Point{
					Line:   int(ref.source.StartLine),
					Column: int(ref.source.StartColumn),
				},
				End: errpos.Point{
					Line:   int(ref.source.EndLine),
					Column: int(ref.source.EndColumn),
				},
			}
		}
		ec.WarnPos(pos, err)
	}

	return fs, nil

}

type summaryWalker struct {
	exports         []*TypeRef
	refs            []*sourcewalk.RefNode
	subPackageFiles []string
}

func (c *summaryWalker) includeSubFile(subPackage string) {
	for _, file := range c.subPackageFiles {
		if file == subPackage {
			return
		}
	}
	c.subPackageFiles = append(c.subPackageFiles, subPackage)
}

func (c *summaryWalker) addExport(ref *TypeRef) {
	c.exports = append(c.exports, ref)
}

func (c *summaryWalker) addRef(ref *sourcewalk.RefNode) {
	c.refs = append(c.refs, ref)
}

func (cc *summaryWalker) collectFileRefs(sourceFile *sourcedef_j5pb.SourceFile) error {
	file := sourcewalk.NewRoot(sourceFile)

	visitor := &sourcewalk.DefaultVisitor{
		Property: func(node *sourcewalk.PropertyNode) error {
			if node.Field.Ref != nil {
				cc.addRef(node.Field.Ref)
			} else if node.Field.Items != nil && node.Field.Items.Ref != nil {
				cc.addRef(node.Field.Items.Ref)
			}
			return nil
		},
		Object: func(node *sourcewalk.ObjectNode) error {
			cc.addExport(objectTypeRef(node))
			return nil
		},
		Oneof: func(node *sourcewalk.OneofNode) error {
			cc.addExport(oneofTypeRef(node))
			return nil
		},
		Enum: func(node *sourcewalk.EnumNode) error {
			valMap := make(map[string]int32)
			for _, value := range node.Schema.Options {
				valMap[node.Schema.Prefix+value.Name] = value.Number
			}
			cc.addExport(enumTypeRef(node))
			return nil
		},
		Service: func(node *sourcewalk.ServiceNode) error {
			cc.includeSubFile("service")
			return nil
		},
		Topic: func(node *sourcewalk.TopicNode) error {
			cc.includeSubFile("topic")
			return nil
		},
	}

	return file.RangeRootElements(visitor)

}

func objectTypeRef(node *sourcewalk.ObjectNode) *TypeRef {
	return &TypeRef{
		Name:       node.NameInPackage(),
		Position:   node.Source.GetPos(),
		MessageRef: &MessageRef{},
	}
}

func oneofTypeRef(node *sourcewalk.OneofNode) *TypeRef {
	return &TypeRef{
		Name:     node.NameInPackage(),
		Position: node.Source.GetPos(),
		MessageRef: &MessageRef{
			Oneof: true,
		},
	}
}

func enumTypeRef(node *sourcewalk.EnumNode) *TypeRef {
	valMap := make(map[string]int32)
	for _, value := range node.Schema.Options {
		valMap[node.Schema.Prefix+value.Name] = value.Number
	}
	return &TypeRef{
		Name:     node.NameInPackage(),
		Position: node.Source.GetPos(),

		EnumRef: &EnumRef{
			Prefix: node.Schema.Prefix,
			ValMap: valMap,
		},
	}
}
