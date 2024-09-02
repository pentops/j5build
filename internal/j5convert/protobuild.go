package j5convert

import (
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type SchemaCollection interface {
	AddSchema(schema *schema_j5pb.RootSchema) error
}

type Package interface {
	ResolveType(pkg string, name string) (*TypeRef, error)
}

type rootContext interface {
	ensureImport(string)
	resolveType(string, string) (*TypeRef, error)
}

type parentContext interface {
	addMessage(*MessageBuilder)
	addEnum(*EnumBuilder)
}

func PackageFromFilename(filename string) string {
	dirName, _ := path.Split(filename)
	dirName = strings.TrimSuffix(dirName, "/")
	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	return pathPackage
}

type TypeNotFoundError struct {
	Package string
	Name    string
}

func (e *TypeNotFoundError) Error() string {
	return fmt.Sprintf("type %s not found in package %s", e.Name, e.Package)
}

type PackageNotFoundError struct {
	Package string
	Name    string
}

func (e *PackageNotFoundError) Error() string {
	return fmt.Sprintf("namespace %s not found (looking for %s.%s), missing import?", e.Package, e.Package, e.Name)
}

type J5Result struct {
	SourceFile *sourcedef_j5pb.SourceFile
	Summary    *FileSummary
}

type EnumRef struct {
	Prefix string
	ValMap map[string]int32
}

func (er *EnumRef) mapValues(vals []string) ([]int32, error) {
	out := make([]int32, len(vals))
	for idx, in := range vals {
		if !strings.HasPrefix(in, er.Prefix) {
			in = er.Prefix + in
		}
		val, ok := er.ValMap[in]
		if !ok {
			return nil, fmt.Errorf("enum value %q not found", in)
		}
		out[idx] = val
	}
	return out, nil
}

type MessageRef struct{}

func (typeRef TypeRef) protoTypeName() *string {
	return ptr(fmt.Sprintf(".%s.%s", typeRef.Package, typeRef.Name))
}

type sourceLink struct {
	root *sourcedef_j5pb.SourceLocation
}

func (c *sourceLink) getPos(path []string) *errpos.Position {
	loc := c.root
	if loc == nil {
		return nil
	}
	for idx, part := range path {
		next := loc.Children[part]
		if next == nil {
			log.Printf("no location for %q in %q[%d], have %s", part, path, idx, keys(loc.Children))
			return nil
		}
		loc = next
	}
	return &errpos.Position{
		Start: errpos.Point{
			Line:   int(loc.StartLine),
			Column: int(loc.StartColumn),
		},
		End: errpos.Point{
			Line:   int(loc.EndLine),
			Column: int(loc.EndColumn),
		},
	}
}

func ptr[T any](v T) *T {
	return &v
}

func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
