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

func TestObjectField(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")
	field := obj.addField("bar")
	field.prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Object{
					Object: &schema_j5pb.Object{
						Properties: []*schema_j5pb.ObjectProperty{{
							ProtoField: []int32{1},
							Name:       "bar_id",
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_String_{String_: &schema_j5pb.StringField{}},
							},
						}},
					},
				},
			},
		},
	}

	file.run(t, `
		object Foo {
			field bar object {
				field bar_id string {
				}
			}
		}
	`)
}

func TestEntity(t *testing.T) {
	file := build()
	ent := file.addEntity("Foo")

	evt := ent.addEvent("DoThing")
	evt.addField("bar").prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_String_{
			String_: &schema_j5pb.StringField{},
		},
	}

	file.run(t, `
		entity Foo {
			event DoThing {
				field bar string
			}
		}
	`)
}

func TestArrayOfObject(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")
	field := obj.addField("bar")
	field.setRequired()
	field.prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Array{
			Array: &schema_j5pb.ArrayField{
				Rules: &schema_j5pb.ArrayField_Rules{
					MinItems: ptr(uint64(1)),
				},
				Items: &schema_j5pb.Field{
					Type: &schema_j5pb.Field_Object{
						Object: &schema_j5pb.ObjectField{
							Rules: &schema_j5pb.ObjectField_Rules{
								MinProperties: ptr(uint64(1)),
							},
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
	/*
		This one demonstrates the scoping mechanism.
	*/

	t.Run("inputA", func(t *testing.T) {
		a := buildSource()
		blk := a.block("object", "Foo")
		// Fresh Scope: j5.sourcedef.v1.Object
		// Object has one tag, name
		// [tag name]: .name = "Foo" (no scope change)

		array := blk.block("field", "bar", "array")

		// Fresh Scope: j5.schema.v1.ObjectProperty

		// OP has name tag
		// tag name: .name = "bar" (no scope change)

		// Type tag at path 'schema', is a oneof, set
		// to an empty j5.schema.v1.ArrayField
		// Stepping 'past' .schema, so that the ObjectProperty.Schema
		// oneof never enters scope itself, otherwise all of the types
		// will be valid within the block.

		// New Scope is *merged*:
		// - j5.schema.v1.ObjectProperty
		// - j5.schema.v1.ArrayField
		// But does not contain j5.schema.v1.Field

		/// Scope is searched in order, so the Property takes precidence
		// over the Field.
		// In this case, there is no conflict.

		array.attr("required", "true")
		// sets the scalar in Property

		array.attr("rules.minItems", "1")
		// sets the scalar in ArrayField.Rules

		objItem := array.block("items", "object")
		// Fresh Scope: j5.schema.v1.ObjectField

		objItem.attr("rules.minProperties", "1")
		objItem.block("ref", "path.to.Type")

		inputA := buildString(blk)
		t.Log(inputA)
		file.run(t, inputA)
	})

	t.Run("inputB", func(t *testing.T) {
		// Same thing but shorter syntax
		b := buildSource()
		blk := b.block("object", "Foo")

		array := blk.block("field", "bar", "array")
		// So far is the same

		array.qualifiers("object", "path.to.Type")
		// Printed as 'field bar array:object: path.to.Type {'
		// Brings the 'items object{' block inline
		// So the object is in scope
		// Also sets object.ref, but it 'acts like' a scalar
		// (tag IsBlock: false in the spec)

		array.attr("required", "true")
		// Exists only in the ObjectProperty, no conflict

		array.attr("rules.minItems", "1")
		// Rules exists in both the ArrayField and the ObjectField,
		// the OUTER wins, so it is set in ArrayField.Rules

		array.attr("items.object.rules.minProperties", "1")
		// To access the object rules, needs to be specified in full

		inputB := buildString(blk)
		t.Log(inputB)
		file.run(t, inputB)
	})
}

func TestConvert(t *testing.T) {

	file := build()
	file.file.Version = "v1"
	obj := file.addObject("Foo")
	obj.obj.Def.Description = "Foo Object Description"
	key := obj.addField("foo_id")

	key.setSchema(&schema_j5pb.Field_Key{Key: &schema_j5pb.KeyField{
		Format: &schema_j5pb.KeyFormat{
			Type: &schema_j5pb.KeyFormat_Uuid{
				Uuid: &schema_j5pb.KeyFormat_UUID{},
			},
		},
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
