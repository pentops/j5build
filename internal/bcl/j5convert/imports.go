package j5convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

const (
	bufValidateImport          = "buf/validate/validate.proto"
	j5ExtImport                = "j5/ext/v1/annotations.proto"
	j5DateImport               = "j5/types/date/v1/date.proto"
	j5DecimalImport            = "j5/types/decimal/v1/decimal.proto"
	j5ListAnnotationsImport    = "j5/list/v1/annotations.proto"
	pbTimestamp                = "google/protobuf/timestamp.proto"
	j5AnyImport                = "j5/types/any/v1/any.proto"
	psmStateImport             = "j5/state/v1/metadata.proto"
	googleApiHttpBodyImport    = "google/api/httpbody.proto"
	googleApiAnnotationsImport = "google/api/annotations.proto"
	googleProtoEmptyImport     = "google/protobuf/empty.proto"
	messagingAnnotationsImport = "j5/messaging/v1/annotations.proto"
	messagingReqResImport      = "j5/messaging/v1/reqres.proto"
	messagingUpsertImport      = "j5/messaging/v1/upsert.proto"
)

const googleProtoEmptyType = ".google.protobuf.Empty"

var implicitImports = map[string]*PackageSummary{
	"j5.state.v1": {
		Exports: map[string]*TypeRef{
			"StateMetadata": {
				Package:    "j5.state.v1",
				Name:       "StateMetadata",
				File:       "j5/state/v1/metadata.proto",
				MessageRef: &MessageRef{},
			},
			"EventMetadata": {
				Package:    "j5.state.v1",
				Name:       "EventMetadata",
				File:       "j5/state/v1/metadata.proto",
				MessageRef: &MessageRef{},
			},
		},
	},
	"j5.list.v1": {
		Exports: map[string]*TypeRef{
			"PageRequest": {
				Package:    "j5.list.v1",
				Name:       "PageRequest",
				File:       "j5/list/v1/page.proto",
				MessageRef: &MessageRef{},
			},
			"PageResponse": {
				Package:    "j5.list.v1",
				Name:       "PageResponse",
				File:       "j5/list/v1/page.proto",
				MessageRef: &MessageRef{},
			},
			"QueryRequest": {
				Package:    "j5.list.v1",
				Name:       "QueryRequest",
				File:       "j5/list/v1/query.proto",
				MessageRef: &MessageRef{},
			},
		},
	},
	"j5.messaging.v1": {
		Exports: map[string]*TypeRef{
			"UpsertMetadata": {
				Package:    "j5.messaging.v1",
				Name:       "UpsertMetadata",
				File:       "j5/messaging/v1/upsert.proto",
				MessageRef: &MessageRef{},
			},
			"RequestMetadata": {
				Package:    "j5.messaging.v1",
				Name:       "RequestMetadata",
				File:       "j5/messaging/v1/reqres.proto",
				MessageRef: &MessageRef{},
			},
		},
	},
}

type TypeNotFoundError struct {
	Package string
	Name    string
}

func (e *TypeNotFoundError) Error() string {
	if e.Package == "" {
		return fmt.Sprintf("type %s not found", e.Name)
	}
	return fmt.Sprintf("type %s not found in package %s", e.Name, e.Package)
}

type PackageNotFoundError struct {
	Package string
	Name    string
}

func (e *PackageNotFoundError) Error() string {
	return fmt.Sprintf("namespace %s not found (looking for %s.%s), missing import?", e.Package, e.Package, e.Name)
}

type importDef struct {
	fullPath string
	used     bool
	source   *bcl_j5pb.SourceLocation
}

type importMap struct {
	vals        map[string]*importDef
	thisPackage string
}

func j5Imports(file *sourcedef_j5pb.SourceFile) (*importMap, error) {
	out := map[string]*importDef{}
	errs := []error{}

	var importSources *bcl_j5pb.SourceLocation
	if file.SourceLocations != nil && file.SourceLocations.Children != nil {
		importSources = file.SourceLocations.Children["imports"]
	}

	for idx, imp := range file.Imports {
		if imp.Path == "" {
			return nil, fmt.Errorf("AddImports: empty import")
		}
		var src *bcl_j5pb.SourceLocation
		if importSources != nil {
			src = importSources.Children[fmt.Sprint(idx)]
		}

		if strings.Contains(imp.Path, "/") {
			pkg := PackageFromFilename(imp.Path)
			out[pkg] = &importDef{
				fullPath: pkg,
				source:   src,
			}
			continue
		}

		pkg := imp.Path
		if imp.Alias != "" {
			out[imp.Alias] = &importDef{
				fullPath: pkg,
				source:   src,
			}
			continue
		}
		parts := strings.Split(pkg, ".")
		if len(parts) < 2 {
			errs = append(errs, fmt.Errorf("invalid package name in import %q", pkg))
			continue
		}
		withoutVersion := parts[len(parts)-2]
		def := &importDef{
			fullPath: pkg,
			source:   src,
		}

		out[withoutVersion] = def
		out[pkg] = def
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &importMap{
		vals:        out,
		thisPackage: file.Package.Name,
	}, nil
}

type expandedRef struct {
	ref      *schema_j5pb.Ref
	implicit *TypeRef
	local    bool
}

func implicitRef(ref *schema_j5pb.Ref) *expandedRef {
	if implicit, ok := implicitImports[ref.Package]; ok {
		typeDef, ok := implicit.Exports[ref.Schema]
		if ok {
			return &expandedRef{
				ref: &schema_j5pb.Ref{
					Package: typeDef.Package,
					Schema:  typeDef.Name,
				},
				implicit: typeDef,
			}
		}
	}
	return nil
}
func (im importMap) expand(ref *schema_j5pb.Ref) *expandedRef {
	spec := ref.Package
	if spec == "" || spec == im.thisPackage {
		return &expandedRef{
			ref: &schema_j5pb.Ref{
				Package: im.thisPackage,
				Schema:  ref.Schema,
			},
			local: true,
		}
	}

	// Try once for specified package
	if implicit := implicitRef(ref); implicit != nil {
		return implicit
	}

	newPackage, ok := im.vals[spec]
	if !ok {
		return nil
	}
	newPackage.used = true
	newRef := &schema_j5pb.Ref{
		Package: newPackage.fullPath,
		Schema:  ref.Schema,
	}

	if implicit := implicitRef(newRef); implicit != nil {
		return implicit
	}

	return &expandedRef{
		local: spec == im.thisPackage,
		ref:   newRef,
	}

}

func (fb *rootContext) resolveTypeNoImport(refSrc *schema_j5pb.Ref) (*TypeRef, error) {

	ref := fb.importAliases.expand(refSrc)
	if ref == nil {
		return nil, &PackageNotFoundError{
			Package: refSrc.Package,
			Name:    refSrc.Schema,
		}
	}

	if ref.implicit != nil {
		return ref.implicit, nil
	}

	typeRef, err := fb.deps.ResolveType(ref.ref.Package, ref.ref.Schema)
	if err != nil {
		return nil, err
	}
	return typeRef, nil

}
