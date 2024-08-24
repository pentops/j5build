package j5parse

import (
	"strings"
	"testing"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
)

func TestObject(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")

	field := obj.addField("foo_id")
	field.setRequired()
	field.prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Key{
			Key: &schema_j5pb.KeyField{
				Format: &schema_j5pb.KeyFormat{
					Type: &schema_j5pb.KeyFormat_Uuid{
						Uuid: &schema_j5pb.KeyFormat_UUID{},
					},
				},
			},
		},
	}

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

func TestConvert(t *testing.T) {

	file := build()
	file.file.Version = "v1"
	obj := file.addObject("Foo")
	obj.obj.Def.Description = "Foo Object Description"
	key := obj.addField("foo_id")

	key.setSchema(&schema_j5pb.Field_Key{Key: &schema_j5pb.KeyField{
		Format: &schema_j5pb.KeyFormat{Type: &schema_j5pb.KeyFormat_Uuid{Uuid: &schema_j5pb.KeyFormat_UUID{}}},
	}})
	key.setRequired()

	obj.addField("bar_field").
		setRequired().
		prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_String_{String_: &schema_j5pb.StringField{
			Rules: &schema_j5pb.StringField_Rules{MinLength: ptr(uint64(1))},
		}},
	}

	baz := obj.addField("baz_field")
	baz.prop.Schema = &schema_j5pb.Field{
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
	}

	baz2 := obj.addField("baz_2")
	baz2.prop.Schema = &schema_j5pb.Field{
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
	}

	file.run(t, strings.Join([]string{
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
	}, "\n"))

}
