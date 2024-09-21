package j5convert

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/sourcewalk"
)

type FileSummary struct {
	Package          string
	Exports          map[string]*TypeRef
	FileDependencies []string
	TypeDependencies []*schema_j5pb.Ref
}

type PackageSummary struct {
	Exports map[string]*TypeRef
	Files   []*FileSummary
}

type TypeRef struct {
	Package  string
	Name     string
	File     string
	Position *errpos.Position

	// Oneof
	*EnumRef
	*MessageRef
}

// SourceSummary collects the exports and imports for a file
func SourceSummary(sourceFile *sourcedef_j5pb.SourceFile) (*FileSummary, error) {

	cc := &collector{}
	err := cc.collectFileRefs(sourceFile)
	if err != nil {
		return nil, err
	}

	fs := &FileSummary{
		Package: sourceFile.Package.Name,
		Exports: make(map[string]*TypeRef),
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

	importPath := sourceFile.Path + ".proto"
	for _, export := range cc.exports {
		export.Package = sourceFile.Package.Name
		export.File = importPath
		fs.Exports[export.Name] = export
		//fmt.Printf("export from %s: %s\n", export.Package, export.Name)
	}

	return fs, nil

}

type collector struct {
	exports []*TypeRef
	refs    []*sourcewalk.RefNode
}

func (c *collector) addExport(ref *TypeRef) {
	c.exports = append(c.exports, ref)
}

func (c *collector) addRef(ref *sourcewalk.RefNode) {
	c.refs = append(c.refs, ref)
}

func (cc *collector) collectFileRefs(sourceFile *sourcedef_j5pb.SourceFile) error {
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
