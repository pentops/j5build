package j5parse

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func TestObject(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")

	field := obj.addField("foo_id")
	field.setRequired()

	file.run(t, `
	object Foo {
		field foo_id key:uuid {
			required = true
		}
	}`)

}

func TestEntity(t *testing.T) {

	file := build()
	ent := file.addEntity("Foo")

	evt := ent.addEvent("doThing")
	evt.addField("bar").prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_String_{
			String_: &schema_j5pb.StringField{},
		},
	}

	file.run(t, `
entity Foo {
	event doThing {
		field bar string
	}
}
`)

}

type fileBuild struct {
	file *sourcedef_j5pb.SourceFile
}

func (fb *fileBuild) run(t *testing.T, input string) {
	parser := NewParser()
	got, err := parser.ParseFile("pentops/j5lang/example/example.ext", input)
	if err != nil {
		if pe, ok := errpos.AsErrorsWithSource(err); ok {
			t.Logf(pe.HumanString(3))
		}
		t.Fatalf("FATAL: %s", err)
	}

	cmpProto(t, fb.file, got)
}

func build() *fileBuild {
	return &fileBuild{
		file: &sourcedef_j5pb.SourceFile{
			Package: "pentops.j5lang.example",
		},
	}
}

type objBuild struct {
	obj *sourcedef_j5pb.Object
}

func (f *fileBuild) addObject(name string) *objBuild {
	obj := &sourcedef_j5pb.Object{
		Def: &schema_j5pb.Object{
			Name: name,
		},
	}
	f.file.Elements = append(f.file.Elements, &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Object{
			Object: obj,
		},
	})
	return &objBuild{obj: obj}
}

func (o *objBuild) addField(name string) *fieldBuild {
	prop := &schema_j5pb.ObjectProperty{
		Name: name,
	}
	num := int32(len(o.obj.Def.Properties) + 1)
	o.obj.Def.Properties = append(o.obj.Def.Properties, prop)
	prop.ProtoField = []int32{num}

	return &fieldBuild{prop: prop}
}

type fieldBuild struct {
	prop *schema_j5pb.ObjectProperty
}

func (f *fieldBuild) setRequired() {
	f.prop.Required = true
}

func (f *fileBuild) addEntity(name string) *entityBuild {
	obj := &sourcedef_j5pb.Entity{
		Name: name,
		Data: &sourcedef_j5pb.Object{
			Def: &schema_j5pb.Object{
				Name: name + "Data",
			},
		},
	}
	f.file.Elements = append(f.file.Elements, &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Entity{
			Entity: obj,
		},
	})
	return &entityBuild{
		obj: obj,
	}
}

type entityBuild struct {
	obj *sourcedef_j5pb.Entity
}

func (eb *entityBuild) addEvent(name string) *objBuild {
	evt := &sourcedef_j5pb.Object{
		Def: &schema_j5pb.Object{
			Name: name,
		},
	}

	eb.obj.Events = append(eb.obj.Events, evt)
	return &objBuild{obj: evt}
}

func TestConvert(t *testing.T) {

	input := strings.Join([]string{
		`version = "v1"`,
		`object Foo {`,
		`| Foo Object Description`,
		`  `,
		`  field foo_id key:uuid {`,
		`    required = true`,
		`  }`,
		`  `,
		`  field bar_field string {`,
		`    required = true`,
		`    rules.minLength = 1`,
		`  }`,
		`  `,
		`  field baz_field object {`,
		`	 ref path.to.Type`,
		`  }`,
		`  `,
		`  field baz_2 array:object {`,
		`	 ref path.to.Type`,
		`  }`,
		`}`,
	}, "\n")

	parser := NewParser()
	got, err := parser.ParseFile("pentops/j5lang/example/example.ext", input)
	if err != nil {
		if pe, ok := errpos.AsErrorsWithSource(err); ok {
			t.Logf(pe.HumanString(3))
		}
		t.Fatalf("FATAL: %s", err)
	}

	want := &sourcedef_j5pb.SourceFile{
		Package: "pentops.j5lang.example",
		Version: "v1",
		Elements: []*sourcedef_j5pb.RootElement{{
			Type: &sourcedef_j5pb.RootElement_Object{
				Object: &sourcedef_j5pb.Object{
					Def: &schema_j5pb.Object{

						Name:        "Foo",
						Description: "Foo Object Description",
						Properties: []*schema_j5pb.ObjectProperty{{
							Name:     "foo_id",
							Required: true,
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_Key{
									Key: &schema_j5pb.KeyField{
										Format: &schema_j5pb.KeyFormat{
											Type: &schema_j5pb.KeyFormat_Uuid{
												Uuid: &schema_j5pb.KeyFormat_UUID{},
											},
										},
									},
								},
							},
						}, {
							Name:     "bar_field",
							Required: true,
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_String_{
									String_: &schema_j5pb.StringField{
										Rules: &schema_j5pb.StringField_Rules{
											MinLength: ptr(uint64(1)),
										},
									},
								},
							},
						}, {
							Name: "baz_field",
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_Object{
									Object: &schema_j5pb.ObjectField{
										Schema: &schema_j5pb.ObjectField_Ref{
											Ref: &schema_j5pb.Ref{
												Package: "path.to",
												Schema:  "Type",
											},
										},
									},
								},
							},
						}, {

							Name: "baz_2",
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_Array{
									Array: &schema_j5pb.ArrayField{
										Items: &schema_j5pb.Field{
											Type: &schema_j5pb.Field_Object{
												Object: &schema_j5pb.ObjectField{

													Schema: &schema_j5pb.ObjectField_Ref{
														Ref: &schema_j5pb.Ref{
															Package: "path.to",
															Schema:  "Type",
														},
													},
												},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		}},
	}

	cmpProto(t, want, got)

}

func ptr[T any](v T) *T {
	return &v
}

func cmpProto(t testing.TB, want, got proto.Message) {
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Error(diff)
	}

}
