package j5convert

import (
	"errors"
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/google/go-cmp/cmp"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestPackageParse(t *testing.T) {

	for _, tc := range []struct {
		input   string
		wantPkg string
		wantSub string
		wantErr bool
	}{{
		input:   "test/v1/foo.j5s",
		wantPkg: "test.v1",
		wantSub: "",
	}, {
		input:   "test/v1/sub/foo.j5s",
		wantPkg: "test.v1",
		wantSub: "sub",
	}, {
		input:   "test/v1/sub/subsub/foo.j5s",
		wantErr: true,
	}, {
		input:   "test",
		wantErr: true,
	}, {
		input:   "foo/bar/v1/foo.j5s",
		wantPkg: "foo.bar.v1",
		wantSub: "",
	}, {
		input:   "foo/bar/v1/sub/foo.j5s",
		wantPkg: "foo.bar.v1",
		wantSub: "sub",
	}} {
		t.Run(tc.input, func(t *testing.T) {
			gotPkg, gotSub, err := SplitPackageFromFilename(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePackage(%q) = %q, %q, nil, want error", tc.input, gotPkg, gotSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("returned error: %s", err)
			}
			if gotPkg != tc.wantPkg || gotSub != tc.wantSub {
				t.Fatalf("%q -> %q, %q want %q, %q", tc.input, gotPkg, gotSub, tc.wantPkg, tc.wantSub)

			}
		})
	}

}
func withOption[T protoreflect.ProtoMessage](opt T, extType protoreflect.ExtensionType, extVal proto.Message) T {
	proto.SetExtension(opt, extType, extVal)
	return opt
}

type testDeps struct {
	pkg   string
	types map[string]*TypeRef
}

func (d *testDeps) PackageName() string {
	return d.pkg
}

func (d *testDeps) ResolveType(pkg string, name string) (*TypeRef, error) {
	if pkg == "" {
		pkg = d.pkg
	}

	if tr, ok := d.types[pkg+"."+name]; ok {
		return tr, nil
	}

	return nil, &TypeNotFoundError{
		Package: pkg,
		Name:    name,
	}
}
func assertIsTypeNotFound(t *testing.T, err error, want *TypeNotFoundError) {
	gotNotFound := &TypeNotFoundError{}
	if !errors.As(err, &gotNotFound) {
		t.Fatalf("got error %v, want TypeNotFoundError", err)
	}
	if gotNotFound.Package != want.Package || gotNotFound.Name != want.Name {
		t.Fatalf("got error %v, want %v", gotNotFound, want)
	}
}

func assertIsPackageNotFound(t *testing.T, err error, want *PackageNotFoundError) {
	gotErr := &PackageNotFoundError{}
	if !errors.As(err, &gotErr) {
		t.Fatalf("got error %v, want TypeNotFoundError", err)
	}
	if gotErr.Package != want.Package {
		t.Fatalf("got error %v, want %v", gotErr, want)
	}
}

func TestSchemaToProto(t *testing.T) {

	deps := &testDeps{
		pkg: "test.v1",
		types: map[string]*TypeRef{
			"test.v1.TestEnum": {
				Package: "test.v1",
				Name:    "TestEnum",
				File:    "test/v1/test.j5s.proto",
				EnumRef: &EnumRef{
					Prefix: "TEST_ENUM_",
					ValMap: map[string]int32{
						"TEST_ENUM_FOO": 1,
					},
				},
			},
		},
	}

	objectSchema := &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Object{
			Object: &sourcedef_j5pb.Object{
				Def: &schema_j5pb.Object{
					Name:        "Referenced",
					Description: "Message Comment",
					Properties: []*schema_j5pb.ObjectProperty{{
						Name:        "field1",
						Description: "Field Comment",
						Schema: &schema_j5pb.Field{
							Type: &schema_j5pb.Field_String_{
								String_: &schema_j5pb.StringField{},
							},
						},
					}, {
						Name: "enum",
						Schema: &schema_j5pb.Field{
							Type: &schema_j5pb.Field_Enum{
								Enum: &schema_j5pb.EnumField{
									Schema: &schema_j5pb.EnumField_Ref{
										Ref: &schema_j5pb.Ref{
											Package: "",
											Schema:  "TestEnum",
										},
									},
								},
							},
						},
					}},
				},
			},
		},
	}

	enumSchema := &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Enum{
			Enum: &schema_j5pb.Enum{
				Name:   "TestEnum",
				Prefix: "TEST_ENUM_",
				Options: []*schema_j5pb.Enum_Option{{
					Name:   "UNSPECIFIED",
					Number: 0,
				}, {
					Name:   "FOO",
					Number: 1,
				}},
			},
		},
	}

	gotFile, err := ConvertJ5File(deps, &sourcedef_j5pb.SourceFile{
		Package:  &sourcedef_j5pb.Package{Name: "test.v1"},
		Path:     "test/v1/test.j5s",
		Elements: []*sourcedef_j5pb.RootElement{objectSchema, enumSchema},
	})
	if err != nil {
		t.Fatalf("ConvertJ5File failed: %v", err)
	}

	wantFile := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{
			//GoPackage: proto.String("github.com/pentops/j5/test/v1/test_pb"),
		},
		Dependency: []string{"j5/ext/v1/annotations.proto"},
		Name:       proto.String("test/v1/test.j5s.proto"),
		Package:    proto.String("test.v1"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Referenced"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     proto.String("field_1"),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Number:   proto.Int32(1),
				Options:  tEmptyTypeExt(t, "string"),
				JsonName: proto.String("field1"),
			}, {
				Name:     proto.String("enum"),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				Number:   proto.Int32(2),
				TypeName: proto.String(".test.v1.TestEnum"),
				Options: withOption(tEmptyTypeExt(t, "enum"), validate.E_Field, &validate.FieldConstraints{
					Type: &validate.FieldConstraints_Enum{
						Enum: &validate.EnumRules{
							DefinedOnly: ptr(true),
						},
					},
				}),
				JsonName: proto.String("enum"),
			}},
			Options: &descriptorpb.MessageOptions{},
		}},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("TestEnum"),
			Value: []*descriptorpb.EnumValueDescriptorProto{{
				Name:   proto.String("TEST_ENUM_UNSPECIFIED"),
				Number: proto.Int32(0),
			}, {
				Name:   proto.String("TEST_ENUM_FOO"),
				Number: proto.Int32(1),
			}},
		}},
	}

	gotFile[0].SourceCodeInfo = nil
	equal(t, wantFile, gotFile[0])

	/*
		fds := &descriptorpb.FileDescriptorSet{
			File: []*descriptorpb.FileDescriptorProto{gotFile},
		}

		fm := NewFileMap()

		if err := protoprint.PrintProtoFiles(context.Background(), fm, fds, protoprint.Options{}); err != nil {
			t.Fatalf("PrintProtoFiles failed: %v", err)
		}

		for filename, content := range fm {
			t.Logf("\n====== %s ======\n%s", filename, content)
		}
	*/

}

// Copies the J5 extension object to the equivalent protoreflect extension type
// by field names.
func tEmptyTypeExt(t testing.TB, fieldType protoreflect.Name) *descriptorpb.FieldOptions {

	// Options in the *proto* representation.
	extOptions := &ext_j5pb.FieldOptions{}
	extOptionsRefl := extOptions.ProtoReflect()

	// The proto extension is a oneof to each field type, which should match the
	// specified type.

	typeField := extOptionsRefl.Descriptor().Fields().ByName(fieldType)
	if typeField == nil {
		t.Fatalf("Field %s does not have a type field", fieldType)
	}

	extTypedRefl := extOptionsRefl.Mutable(typeField).Message()
	if extTypedRefl == nil {
		t.Fatalf("Field %s type field is not a message", fieldType)
	}

	fieldOptions := &descriptorpb.FieldOptions{}

	proto.SetExtension(fieldOptions, ext_j5pb.E_Field, extOptions)
	return fieldOptions
}

func equal(t testing.TB, want, got proto.Message) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Errorf("Mismatch (-want +got):\n%s", diff)
	}

}
