package j5convert

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
)

type PackageSummary struct {
	Exports map[string]*TypeRef
	Files   []*FileSummary
}

type FileSummary struct {
	SourceFilename   string
	Package          string
	Exports          map[string]*TypeRef
	FileDependencies []string
	TypeDependencies []*schema_j5pb.Ref

	ProducesFiles []string
}

// MessageRef is the summary of a message definition (Object or Oneof)
type MessageRef struct {
	Oneof bool
}

// EnumRef is the summary of an enum definition
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

// TypeRef is the summary of an exported type
type TypeRef struct {
	Package  string
	Name     string
	File     string
	Position *errpos.Position

	// Oneof
	*EnumRef
	*MessageRef
}

func (typeRef TypeRef) protoTypeName() *string {
	if typeRef.Package == "" {
		return gl.Ptr(typeRef.Name)
	}
	return gl.Ptr(fmt.Sprintf(".%s.%s", typeRef.Package, typeRef.Name))
}

type TypeResolver interface {
	ResolveType(pkg string, name string) (*TypeRef, error)
}
