package protobuild

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/prototools/protoprint"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestSchemaToProto(t *testing.T) {

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
						ProtoField: []int32{1},
					}, {
						Name: "enum",
						Schema: &schema_j5pb.Field{
							Type: &schema_j5pb.Field_Enum{
								Enum: &schema_j5pb.EnumField{
									Schema: &schema_j5pb.EnumField_Ref{
										Ref: &schema_j5pb.Ref{
											Package: "test.v1",
											Schema:  "TestEnum",
										},
									},
								},
							},
						},
						ProtoField: []int32{3},
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
				Options: []*schema_j5pb.Enum_Value{{
					Name:   "UNSPECIFIED",
					Number: 0,
				}, {
					Name:   "FOO",
					Number: 1,
				}},
			},
		},
	}

	fb := NewFileBuilder("test.v1", "test/v1/test.proto")

	if err := fb.AddRoot(objectSchema); err != nil {
		t.Fatalf("AddSchema failed: %v", err)
	}
	if err := fb.AddRoot(enumSchema); err != nil {
		t.Fatalf("AddSchema failed: %v", err)
	}

	gotFile := fb.File()

	wantFile := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Options: &descriptorpb.FileOptions{
			//GoPackage: proto.String("github.com/pentops/j5/test/v1/test_pb"),
		},
		Name:    proto.String("test/v1/test.proto"),
		Package: proto.String("test.v1"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Referenced"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:    proto.String("field_1"),
				Type:    descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				Number:  proto.Int32(1),
				Options: &descriptorpb.FieldOptions{},
			}, {
				Name:     proto.String("enum"),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				Number:   proto.Int32(3),
				TypeName: proto.String(".test.v1.TestEnum"),
				Options:  &descriptorpb.FieldOptions{},
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

	gotFile.SourceCodeInfo = nil
	equal(t, wantFile, gotFile)

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

}
func equal(t testing.TB, want, got proto.Message) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Errorf("Mismatch (-want +got):\n%s", diff)
	}

}

type fileMap map[string][]byte

func NewFileMap() fileMap {
	return make(fileMap)
}

func (fm fileMap) GetFile(filename string) ([]byte, error) {
	if b, ok := fm[filename]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}

func (fm fileMap) PutFile(ctx context.Context, filename string, content []byte) error {

	fm[filename] = content
	return nil
}
