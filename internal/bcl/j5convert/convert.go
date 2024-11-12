package j5convert

import (
	"fmt"
	"log"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type SchemaCollection interface {
	AddSchema(schema *schema_j5pb.RootSchema) error
}

type Package interface {
	ResolveType(pkg string, name string) (*TypeRef, error)
}

func PackageFromFilename(filename string) string {
	dirName, _ := path.Split(filename)
	dirName = strings.TrimSuffix(dirName, "/")
	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	return pathPackage
}

var reVersion = regexp.MustCompile(`^v\d+$`)

func SplitPackageFromFilename(filename string) (string, string, error) {
	pkg := PackageFromFilename(filename)
	parts := strings.Split(pkg, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid package %q for file %q", pkg, filename)
	}

	// foo.v1 -> foo, v1
	// foo.v1.service -> foo.v1, service
	// foo.bar.v1.service -> foo.bar.v1, service

	if reVersion.MatchString(parts[len(parts)-1]) {
		return pkg, "", nil
	}
	if reVersion.MatchString(parts[len(parts)-2]) {
		upToVersion := parts[:len(parts)-1]
		return strings.Join(upToVersion, "."), parts[len(parts)-1], nil
	}
	return pkg, "", fmt.Errorf("no version in package %q", pkg)
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

type MessageRef struct {
	Oneof bool
}

func (typeRef TypeRef) protoTypeName() *string {
	if typeRef.Package == "" {
		return gl.Ptr(typeRef.Name)
	}
	return gl.Ptr(fmt.Sprintf(".%s.%s", typeRef.Package, typeRef.Name))
}

type sourceLink struct {
	root *bcl_j5pb.SourceLocation
}

func (c *sourceLink) getSource(path []string) *bcl_j5pb.SourceLocation {
	loc := c.root
	if loc == nil {
		return nil
	}
	for idx, part := range path {

		next := loc.Children[part]
		if next == nil {
			if part != "<virtual>" {
				didSee := path[:idx]
				after := path[idx:]
				log.Printf("no location at %q for %q have %q", strings.Join(didSee, "."), after, slices.Sorted(maps.Keys(loc.Children)))
			}
			return loc // last known
		}
		loc = next
	}
	return loc
}

func (c *sourceLink) getPos(path []string) *errpos.Position {
	loc := c.getSource(path)
	if loc == nil {
		return nil
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
