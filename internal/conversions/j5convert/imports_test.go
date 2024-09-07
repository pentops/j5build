package j5convert

import (
	"testing"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func runField(t *testing.T,
	deps *testDeps,
	inputProp *schema_j5pb.ObjectProperty,
	wantField *descriptorpb.FieldDescriptorProto,
) {
	t.Helper()
	obj := tSimpleObject(inputProp)
	file := &sourcedef_j5pb.SourceFile{
		Path:     "test/v1/test.proto",
		Package:  &sourcedef_j5pb.Package{Name: "test.v1"},
		Elements: []*sourcedef_j5pb.RootElement{obj},
	}

	gotFile, err := ConvertJ5File(deps, file)
	if err != nil {
		t.Fatalf("ConvertJ5File failed: %v", err)
	}
	if len(gotFile[0].MessageType) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(gotFile[0].MessageType))
	}
	if len(gotFile[0].MessageType[0].Field) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(gotFile[0].MessageType[0].Field))
	}

	gotField := gotFile[0].MessageType[0].Field[0]
	equal(t, wantField, gotField)
}

func tSimpleObject(prop *schema_j5pb.ObjectProperty) *sourcedef_j5pb.RootElement {
	objectSchema := &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Object{
			Object: &sourcedef_j5pb.Object{
				Def: &schema_j5pb.Object{
					Name: "Test",
					Properties: []*schema_j5pb.ObjectProperty{
						prop,
					},
				},
			},
		},
	}
	return objectSchema
}

func tObjectRef(ref *schema_j5pb.Ref) *schema_j5pb.ObjectProperty {
	return &schema_j5pb.ObjectProperty{
		Name: "field",
		Schema: &schema_j5pb.Field{
			Type: &schema_j5pb.Field_Object{
				Object: &schema_j5pb.ObjectField{
					Schema: &schema_j5pb.ObjectField_Ref{
						Ref: ref,
					},
				},
			},
		},
	}
}

func TestImports(t *testing.T) {

	// Run various ways of importing the same object.
	// The object is baz.v1.Referenced

	wantField := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String("field"),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		Number:   proto.Int32(1),
		TypeName: proto.String(".baz.v1.Referenced"),
		JsonName: proto.String("field"),
		Options:  &descriptorpb.FieldOptions{},
	}

	type happy struct {
		shouldImport bool
	}

	type sad struct {
		Type    *TypeNotFoundError
		Package *PackageNotFoundError
	}

	for _, tc := range []struct {
		name            string
		filename        string
		ref             *schema_j5pb.Ref
		explicitImports []*sourcedef_j5pb.Import
		happy           *happy
		sad             *sad
	}{{
		name:     "same file",
		filename: "baz/v1/referenced.j5s",
		ref: &schema_j5pb.Ref{
			Package: "",
			Schema:  "Referenced",
		},
		happy: &happy{
			shouldImport: false,
		},
	}, {
		name:     "same package",
		filename: "baz/v1/other.j5s",
		ref: &schema_j5pb.Ref{
			Package: "",
			Schema:  "Referenced",
		},
		happy: &happy{
			shouldImport: true,
		},
	}, {
		name:     "qualified same file",
		filename: "baz/v1/referenced.j5s",
		happy: &happy{
			shouldImport: false,
		},
	}, {
		name:     "qualified same package",
		filename: "baz/v1/other.j5s",
		happy: &happy{
			shouldImport: true,
		},
	}, {
		name:     "missing local",
		filename: "baz/v1/other.j5s",
		ref: &schema_j5pb.Ref{
			Package: "baz.v1",
			Schema:  "Missing",
		},
		sad: &sad{
			Type: &TypeNotFoundError{
				Package: "baz.v1",
				Name:    "Missing",
			},
		},
	}, {
		name: "cross package",
		explicitImports: []*sourcedef_j5pb.Import{{
			Path: "baz/v1/referenced.j5s.proto",
		}},
		happy: &happy{
			shouldImport: true,
		},
	}, {
		name:            "without import",
		explicitImports: []*sourcedef_j5pb.Import{},
		sad: &sad{
			Package: &PackageNotFoundError{
				Package: "baz.v1",
			},
		},
	}, {
		name: "package import",
		explicitImports: []*sourcedef_j5pb.Import{{
			Path: "baz.v1",
		}},
		happy: &happy{shouldImport: true},
	}, {
		name: "package import no version",
		explicitImports: []*sourcedef_j5pb.Import{{
			Path: "baz.v1",
		}},
		ref:   &schema_j5pb.Ref{Package: "baz", Schema: "Referenced"},
		happy: &happy{shouldImport: true},
	}, {
		name: "package import alias",
		explicitImports: []*sourcedef_j5pb.Import{{
			Path: "baz.v1", Alias: "quz",
		}},
		ref:   &schema_j5pb.Ref{Package: "quz", Schema: "Referenced"},
		happy: &happy{shouldImport: true},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ref == nil {
				// default to fully qualified
				tc.ref = &schema_j5pb.Ref{
					Package: "baz.v1",
					Schema:  "Referenced",
				}
			}

			if tc.filename == "" {
				tc.filename = "test/v1/test.proto"
			}

			if tc.happy == nil && tc.sad == nil {
				t.Fatalf("must specify happy or sad")
			}

			pkgName := PackageFromFilename(tc.filename)

			input := &sourcedef_j5pb.SourceFile{
				Path:    tc.filename,
				Package: &sourcedef_j5pb.Package{Name: pkgName},
				Imports: tc.explicitImports,
				Elements: []*sourcedef_j5pb.RootElement{
					tSimpleObject(
						tObjectRef(tc.ref),
					),
				},
			}

			deps := &testDeps{
				pkg: pkgName,
				types: map[string]*TypeRef{
					"baz.v1.Referenced": {
						Package:    "baz.v1",
						Name:       "Referenced",
						File:       "baz/v1/referenced.j5s.proto",
						MessageRef: &MessageRef{},
					},
				},
			}
			gotFile, gotErr := ConvertJ5File(deps, input)

			if tc.sad != nil {
				if gotErr == nil {
					t.Fatalf("Expected error, got nil")
				}
				if tc.sad.Type != nil {
					assertIsTypeNotFound(t, gotErr, tc.sad.Type)
				}
				if tc.sad.Package != nil {
					assertIsPackageNotFound(t, gotErr, tc.sad.Package)
				}
			}

			if tc.happy != nil {
				if gotErr != nil {
					t.Fatalf("FATAL: tSimpleObject failed: %v", gotErr)
				}

				if len(gotFile[0].MessageType) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(gotFile[0].MessageType))
				}
				if len(gotFile[0].MessageType[0].Field) != 1 {
					t.Fatalf("Expected 1 field, got %d", len(gotFile[0].MessageType[0].Field))
				}

				gotField := gotFile[0].MessageType[0].Field[0]
				equal(t, wantField, gotField)
				wantImports := []string{}
				if tc.happy.shouldImport {
					wantImports = []string{"baz/v1/referenced.j5s.proto"}
				}

				assertImportsMatch(t, wantImports, gotFile[0].Dependency)
			}

		})
	}

}

func assertImportsMatch(t *testing.T, wantImports, gotImports []string) {
	t.Helper()
	for idx := 0; idx < len(wantImports) || idx < len(gotImports); idx++ {
		want := "<nothing>"
		got := "<missing>"
		if idx < len(gotImports) {
			got = gotImports[idx]
		}
		if idx < len(wantImports) {
			want = wantImports[idx]
		}
		if got != want {
			t.Errorf("Import %d: got %q, want %q", idx, got, want)
		}
	}
}
