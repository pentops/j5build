package j5client

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/j5/gen/j5/auth/v1/auth_j5pb"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
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
	got.Schemas = nil

	assertEqualProto(t, want, got)
}

func assertEqualProto(t *testing.T, want, got proto.Message) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Error(diff)
	}
}

func wantAPI() *client_j5pb.API {

	objectRef := func(pkg, schema string) *schema_j5pb.Field {
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

	array := func(of *schema_j5pb.Field) *schema_j5pb.Field {
		return &schema_j5pb.Field{
			Type: &schema_j5pb.Field_Array{
				Array: &schema_j5pb.ArrayField{
					Items: of,
				},
			},
		}
	}

	authJWT := &auth_j5pb.MethodAuthType{
		Type: &auth_j5pb.MethodAuthType_JwtBearer{
			JwtBearer: &auth_j5pb.MethodAuthType_JWTBearer{},
		},
	}
	authNone := &auth_j5pb.MethodAuthType{
		Type: &auth_j5pb.MethodAuthType_None_{
			None: &auth_j5pb.MethodAuthType_None{},
		},
	}

	getFoo := &client_j5pb.Method{
		Name:         "GetFoo",
		Auth:         authNone,
		FullGrpcName: "/test.foo.v1.FooQueryService/GetFoo",
		HttpMethod:   client_j5pb.HTTPMethod_GET,
		HttpPath:     "/test/v1/foo/:id",
		Request: &client_j5pb.Method_Request{
			PathParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "id",
				ProtoField: []int32{1},
				Required:   true,
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_String_{
						String_: &schema_j5pb.StringField{},
					},
				},
			}},
			QueryParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "number",
				ProtoField: []int32{2},
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Integer{
						Integer: &schema_j5pb.IntegerField{
							Format: schema_j5pb.IntegerField_FORMAT_INT64,
						},
					},
				},
			}, {
				Name:       "numbers",
				ProtoField: []int32{3},
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Array{
						Array: &schema_j5pb.ArrayField{
							Items: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_Float{
									Float: &schema_j5pb.FloatField{
										Format: schema_j5pb.FloatField_FORMAT_FLOAT32,
									},
								},
							},
						},
					},
				},
			}, {
				Name:       "ab",
				ProtoField: []int32{4},
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Object{
						Object: &schema_j5pb.ObjectField{
							Schema: &schema_j5pb.ObjectField_Ref{
								Ref: &schema_j5pb.Ref{
									Package: "test.foo.v1.service",
									Schema:  "ABMessage",
								},
							},
						},
					},
				},
			}, {
				Name:       "multipleWord",
				ProtoField: []int32{5},
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_String_{
						String_: &schema_j5pb.StringField{},
					},
				},
			}},
		},
		ResponseBody: &schema_j5pb.Object{
			Name: "GetFooResponse",
			Properties: []*schema_j5pb.ObjectProperty{{
				Name:       "foo",
				ProtoField: []int32{1},
				Schema:     objectRef("test.foo.v1", "FooState"),
			}},
		},
	}

	listFoos := &client_j5pb.Method{
		Name:         "ListFoos",
		Auth:         authJWT,
		FullGrpcName: "/test.foo.v1.FooQueryService/ListFoos",
		HttpMethod:   client_j5pb.HTTPMethod_GET,

		HttpPath: "/test/v1/foos",
		Request: &client_j5pb.Method_Request{
			QueryParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "page",
				ProtoField: []int32{100},
				Schema:     objectRef("j5.list.v1", "PageRequest"),
			}, {
				Name:       "query",
				ProtoField: []int32{101},
				Schema:     objectRef("j5.list.v1", "QueryRequest"),
			}},
			List: &client_j5pb.ListRequest{
				SearchableFields: []*client_j5pb.ListRequest_SearchField{{
					Name: "name",
				}, {
					Name: "bar.field",
				}},
				SortableFields: []*client_j5pb.ListRequest_SortField{{
					Name: "createdAt",
				}},
				FilterableFields: []*client_j5pb.ListRequest_FilterField{{
					Name:           "status",
					DefaultFilters: []string{"ACTIVE"},
				}, {
					Name: "bar.id",
				}, {
					Name: "createdAt",
				}},
			},
		},
		ResponseBody: &schema_j5pb.Object{
			Name: "ListFoosResponse",
			Properties: []*schema_j5pb.ObjectProperty{{
				Name:       "foos",
				ProtoField: []int32{1},
				Schema:     array(objectRef("test.foo.v1", "FooState")),
			}},
		},
	}

	listFooEvents := &client_j5pb.Method{
		Name:         "ListFooEvents",
		Auth:         authNone,
		FullGrpcName: "/test.foo.v1.FooQueryService/ListFooEvents",
		HttpMethod:   client_j5pb.HTTPMethod_GET,
		HttpPath:     "/test/v1/foo/:id/events",
		Request: &client_j5pb.Method_Request{
			PathParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "id",
				ProtoField: []int32{1},
				Required:   true,
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Key{
						Key: &schema_j5pb.KeyField{
							Ext: &schema_j5pb.KeyField_Ext{},
							Format: &schema_j5pb.KeyFormat{
								Type: &schema_j5pb.KeyFormat_Uuid{
									Uuid: &schema_j5pb.KeyFormat_UUID{},
								},
							},
						},
					},
				},
			}},
			QueryParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "page",
				ProtoField: []int32{100},
				Schema:     objectRef("j5.list.v1", "PageRequest"),
			}, {
				Name:       "query",
				ProtoField: []int32{101},
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Object{
						Object: &schema_j5pb.ObjectField{
							Schema: &schema_j5pb.ObjectField_Ref{
								Ref: &schema_j5pb.Ref{
									Package: "j5.list.v1",
									Schema:  "QueryRequest",
								},
							},
						},
					},
				},
			}},
			List: &client_j5pb.ListRequest{
				// empty object because it is a list, but no fields.
			},
		},
		ResponseBody: &schema_j5pb.Object{
			Name: "ListFooEventsResponse",
			Properties: []*schema_j5pb.ObjectProperty{{
				Name:       "events",
				ProtoField: []int32{1},
				Schema:     array(objectRef("test.foo.v1", "FooEvent")),
			}},
		},
	}

	fooQueryService := &client_j5pb.Service{
		Name: "FooQueryService",
		Methods: []*client_j5pb.Method{
			getFoo,
			listFoos,
			listFooEvents,
		},
	}

	postFoo := &client_j5pb.Method{
		Name:         "PostFoo",
		Auth:         authJWT,
		FullGrpcName: "/test.foo.v1.FooCommandService/PostFoo",
		HttpMethod:   client_j5pb.HTTPMethod_POST,
		HttpPath:     "/test/v1/foo",
		Request: &client_j5pb.Method_Request{
			Body: &schema_j5pb.Object{
				Name: "PostFooRequest",
				Properties: []*schema_j5pb.ObjectProperty{{
					Name:       "id",
					ProtoField: []int32{1},
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_String_{
							String_: &schema_j5pb.StringField{},
						},
					},
				}},
			},
		},
		ResponseBody: &schema_j5pb.Object{
			Name: "PostFooResponse",
			Properties: []*schema_j5pb.ObjectProperty{{
				Name:       "foo",
				ProtoField: []int32{1},
				Schema:     objectRef("test.foo.v1", "FooState"),
			}},
		},
	}

	fooCommandService := &client_j5pb.Service{
		Name: "FooCommandService",
		Methods: []*client_j5pb.Method{
			postFoo,
		},
	}

	downloadFoo := &client_j5pb.Method{
		Name:         "DownloadRaw",
		FullGrpcName: "/test.foo.v1.FooDownloadService/DownloadRaw",
		HttpMethod:   client_j5pb.HTTPMethod_GET,
		HttpPath:     "/test/v1/foo/:id/raw",
		Request: &client_j5pb.Method_Request{
			PathParameters: []*schema_j5pb.ObjectProperty{{
				Name:       "id",
				ProtoField: []int32{1},
				Required:   true,
				Schema: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_String_{
						String_: &schema_j5pb.StringField{},
					},
				},
			}},
		},
		ResponseBody: nil,
	}

	fooDownloadService := &client_j5pb.Service{
		Name: "FooDownloadService",
		Methods: []*client_j5pb.Method{
			downloadFoo,
		},
	}

	return &client_j5pb.API{
		Packages: []*client_j5pb.Package{{
			Name: "test.foo.v1",

			Services: []*client_j5pb.Service{
				fooDownloadService,
			},
			StateEntities: []*client_j5pb.StateEntity{{
				Name:         "foo",
				FullName:     "test.foo.v1/foo",
				SchemaName:   "test.foo.v1.FooState",
				PrimaryKey:   []string{"fooId"},
				QueryService: fooQueryService,
				CommandServices: []*client_j5pb.Service{
					fooCommandService,
				},
				Events: []*client_j5pb.StateEvent{{
					Name:        "created",
					FullName:    "test.foo.v1/foo.created",
					Description: "Comment on Created",
				}, {
					Name:        "updated",
					FullName:    "test.foo.v1/foo.updated",
					Description: "Comment on Updated",
				}},
			}},
		}},
	}
}
