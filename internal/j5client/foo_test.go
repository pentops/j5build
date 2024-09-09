package j5client

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/j5/gen/j5/list/v1/list_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/j5build/internal/structure"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestFooSchema(t *testing.T) {

	ctx := context.Background()
	rootFS := os.DirFS("../../")
	envResolver, err := source.NewEnvResolver()
	if err != nil {
		t.Fatalf("NewEnvResolver: %v", err)
	}

	thisRoot, err := source.NewFSSource(ctx, rootFS, envResolver)
	if err != nil {
		t.Fatalf("ReadLocalSource: %v", err)
	}

	srcImg, _, err := thisRoot.BundleImageSource(ctx, "test")
	if err != nil {
		t.Fatalf("NamedInput: %v", err)
	}

	sourceAPI, err := structure.APIFromImage(srcImg)
	if err != nil {
		t.Fatalf("APIFromImage: %v", err)
	}

	for _, pkg := range sourceAPI.Packages {
		t.Logf("Package: %s", pkg.Name)
		for name := range pkg.Schemas {
			t.Logf("Schema: %s", name)
		}
	}

	clientAPI, err := APIFromSource(sourceAPI)
	if err != nil {
		t.Fatalf("APIFromSource: %v", err)
	}

	t.Logf("ClientAPI: %s", prototext.Format(clientAPI))

	want := wantAPI().Packages[0]

	got := clientAPI.Packages[0]

	// test schemas separately.
	schemas := got.Schemas
	got.Schemas = nil

	assertEqualProto(t, want, got)

	gotFooState := schemas["FooState"]
	wantFooState := wantFooState()
	assertEqualProto(t, wantFooState, gotFooState)
}

func assertEqualProto(t *testing.T, want, got proto.Message) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Error(diff)
	}
}

func tObjectRef(pkg, schema string) *schema_j5pb.Field {
	return &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Ref{
					Ref: &schema_j5pb.Ref{
						Package: pkg,
						Schema:  schema,
					},
				},
			},
		},
	}
}

func tArrayOf(of *schema_j5pb.Field) *schema_j5pb.Field {
	return &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Array{
			Array: &schema_j5pb.ArrayField{
				Items: of,
			},
		},
	}
}

func wantFooState() *schema_j5pb.RootSchema {

	object := &schema_j5pb.Object{
		Name: "FooState",
		Entity: &schema_j5pb.EntityObject{
			Entity: "foo",
			Part:   schema_j5pb.EntityPart_STATE,
		},
		Properties: []*schema_j5pb.ObjectProperty{{
			Name:       "fooId",
			Required:   true,
			ProtoField: []int32{1, 1}, // flattened
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Key{
					Key: &schema_j5pb.KeyField{
						Ext: &schema_j5pb.KeyField_Ext{
							PrimaryKey: true,
						},
						ListRules: &list_j5pb.KeyRules{
							Filtering: &list_j5pb.FilteringConstraint{
								Filterable: true,
							},
						},
						Format: &schema_j5pb.KeyFormat{
							Type: &schema_j5pb.KeyFormat_Uuid{
								Uuid: &schema_j5pb.KeyFormat_UUID{},
							},
						},
					},
				},
			},
		}, {
			Name:       "status",
			Required:   true,
			ProtoField: []int32{2},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Enum{
					Enum: &schema_j5pb.EnumField{
						Schema: &schema_j5pb.EnumField_Ref{
							Ref: &schema_j5pb.Ref{
								Package: "test.foo.v1",
								Schema:  "FooStatus",
							},
						},
						ListRules: &list_j5pb.EnumRules{
							Filtering: &list_j5pb.FilteringConstraint{
								Filterable: true,
								DefaultFilters: []string{
									"ACTIVE",
								},
							},
						},
					},
				},
			},
		}, {
			Name:       "name",
			ProtoField: []int32{3},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_String_{
					String_: &schema_j5pb.StringField{
						ListRules: &list_j5pb.OpenTextRules{
							Searching: &list_j5pb.SearchingConstraint{
								Searchable: true,
							},
						},
					},
				},
			},
		}, {
			Name:       "bar",
			ProtoField: []int32{4},
			Schema:     tObjectRef("test.foo.v1", "Bar"),
		}, {
			Name:       "createdAt",
			ProtoField: []int32{5},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Timestamp{
					Timestamp: &schema_j5pb.TimestampField{
						ListRules: &list_j5pb.TimestampRules{
							Sorting: &list_j5pb.SortingConstraint{
								Sortable: true,
							},
							Filtering: &list_j5pb.FilteringConstraint{
								Filterable: true,
							},
						},
					},
				},
			},
		}},
	}

	return &schema_j5pb.RootSchema{
		Type: &schema_j5pb.RootSchema_Object{
			Object: object,
		},
	}

}
