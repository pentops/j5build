package j5client

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/test/foo/v1/foo_testspb"
	"github.com/pentops/j5/lib/j5schema"
)

func TestTestListRequest(t *testing.T) {

	ss := j5schema.NewSchemaCache()

	fooDesc := (&foo_testspb.ListFoosResponse{}).ProtoReflect().Descriptor()

	t.Log(protojson.Format(protodesc.ToDescriptorProto(fooDesc)))

	schemaItem, err := ss.Schema(fooDesc)
	if err != nil {
		t.Fatal(err.Error())
	}

	listRequest, err := buildListRequest(schemaItem)
	if err != nil {
		t.Fatal(err.Error())
	}

	want := &client_j5pb.ListRequest{
		SearchableFields: []*client_j5pb.ListRequest_SearchField{
			{
				Name: "name",
			}, {
				Name: "bar.field",
			},
		},
		SortableFields: []*client_j5pb.ListRequest_SortField{{
			Name: "createdAt",
		}},
		FilterableFields: []*client_j5pb.ListRequest_FilterField{
			{
				Name:           "status",
				DefaultFilters: []string{"ACTIVE"},
			},
			{
				Name: "bar.id",
			}, {
				Name: "createdAt",
			},
		},
	}

	if !proto.Equal(listRequest, want) {
		t.Logf("got: %s", protojson.Format(listRequest))
		t.Logf("want: %s", protojson.Format(want))
		t.Fatal("List method did not return expected ListRequest")
	}

}
