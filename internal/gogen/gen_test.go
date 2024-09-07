package gogen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/stretchr/testify/assert"
)

type TestOutput struct {
	Files map[string]string
}

func (o TestOutput) WriteFile(name string, data []byte) error {
	if _, ok := o.Files[name]; ok {
		return fmt.Errorf("file %q already exists", name)
	}
	fmt.Printf("writing file %q\n", name)
	o.Files[name] = string(data)
	return nil
}

func TestTestProtoGen(t *testing.T) {

	api := &client_j5pb.API{
		Packages: []*client_j5pb.Package{{
			Label: "package label",
			Name:  "test.v1",
			Prose: "FOOBAR",
			Services: []*client_j5pb.Service{{
				Name: "TestService",
				Methods: []*client_j5pb.Method{{
					Name:         "PostFoo",
					FullGrpcName: "test.v1.TestService/PostFoo",
					HttpMethod:   client_j5pb.HTTPMethod_GET,
					HttpPath:     "/test/v1/foo/:foo_id",
					Request: &client_j5pb.Method_Request{
						PathParameters: []*schema_j5pb.ObjectProperty{{
							Name:     "foo_id",
							Required: true,
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_String_{
									String_: &schema_j5pb.StringField{},
								},
							},
							ProtoField: []int32{1},
						}},
						Body: &schema_j5pb.Object{
							Name: "PostFooRequest",
							Properties: []*schema_j5pb.ObjectProperty{{
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
					ResponseBody: &schema_j5pb.Object{
						Name: "PostFooRequest",
						Properties: []*schema_j5pb.ObjectProperty{{
							Name: "foo_id",
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_String_{
									String_: &schema_j5pb.StringField{},
								},
							},
							ProtoField: []int32{1},
						}},
					},
				}},
			}},
			Schemas: map[string]*schema_j5pb.RootSchema{
				"test.v1.TestEnum": {
					Type: &schema_j5pb.RootSchema_Enum{
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
				},
			},
		}},
	}

	output := TestOutput{
		Files: map[string]string{},
	}

	options := Options{
		TrimPackagePrefix: "",
		GoPackagePrefix:   "github.com/pentops/j5/testproto/clientgen",
	}

	if err := WriteGoCode(api, output, options); err != nil {
		t.Fatal(err)
	}

	outFile, ok := output.Files["/test/v1/test/generated.go"]
	if !ok {
		t.Fatal("file test/v1/generated.go not found")
	}

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "", outFile, 0)
	if err != nil {
		for idx, line := range strings.Split(outFile, "\n") {
			t.Logf("%d: %s", idx+1, line)
		}
		t.Fatal(err)
	}

	structTypes := map[string]*ast.StructType{}

	for _, decl := range parsed.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			t.Logf("func: %#v", decl.Name.Name)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					t.Logf("type: %#v", spec.Name.Name)
					switch specType := spec.Type.(type) {
					case *ast.StructType:
						structTypes[spec.Name.Name] = specType
					}
				}
			}
		}
	}

	posString := func(thing interface {
		Pos() token.Pos
		End() token.Pos
	}) string {
		return outFile[fset.Position(thing.Pos()).Offset:fset.Position(thing.End()).Offset]
	}

	assertField := func(typeName string, name string, wantTypeName, wantTag string) {
		structType, ok := structTypes[typeName]
		if !ok {
			t.Fatalf("type %q not found", typeName)
		}

		for _, field := range structType.Fields.List {
			for _, fieldName := range field.Names {
				if fieldName.Name == name {
					gotTypeName := posString(field.Type)
					assert.Equal(t, wantTypeName, gotTypeName, "field %q", name)

					gotTag := field.Tag.Value

					assert.Equal(t, "`"+wantTag+"`", gotTag, "field %q tag:", name)
					return
				}
			}
		}
	}

	assertField("PostFooRequest", "SString", "string", `json:"sString,omitempty"`)
	assertField("PostFooRequest", "OString", "*string", `json:"oString,omitempty"`)
	assertField("PostFooRequest", "RString", "[]string", `json:"rString,omitempty"`)
	assertField("PostFooRequest", "MapStringString", "map[string]string", `json:"mapStringString,omitempty"`)

}
