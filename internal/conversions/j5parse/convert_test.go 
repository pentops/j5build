package j5parse

import (
	"strings"
	"testing"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func TestObject(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")

	obj.addField("foo_id").
		setRequired().
		setSchema(basicKey())

	obj.addField("bar").
		setSchema(basicString())

	file.run(t, `
		object Foo {
			field foo_id key:uuid {
				required = true
			}

			field bar string
		}`)
}

func TestValidateNested(t *testing.T) {
	t.Skip("This test fails due to https://github.com/bufbuild/protovalidate-go/issues/141")
	t.Run("only parse", func(t *testing.T) {
		runToErrors(t, `
		object Foo {
			field num integer {
			}
		}
		`)
	})
}

func TestEnum(t *testing.T) {
	file := build()
	enum := file.addEnum("Foo")
	enum.addOption("BAR")
	enum.addOption("BAZ")

	file.run(t, `
		enum Foo {
			option BAR
			option BAZ
		}
		`)

}

func TestObjectField(t *testing.T) {
	file := build()
	obj := file.addObject("Foo")
	field := obj.addField("bar")

	// The name isn't explicitly set in the source, it is automaticaly set from
	// the field name when converting
	fieldObj := field.inlineObject("")
	fieldObj.addField("bar_id").setSchema(basicString())

	file.run(t, `
		object Foo {
			field bar object {
				field bar_id string {
				}
			}
		}
	`)

}

func TestEmptyOneofBody(t *testing.T) {

	file := build()
	obj := file.addObject("Foo")
	obj.addField("bar_id").setSchema(basicString())

	file.run(t, `
		object Foo {
			field bar_id string
		}
	`, func(t *testing.T, file *sourcedef_j5pb.SourceFile) {
		fieldProp := file.Elements[0].GetObject().Def.Properties[0]

		if fieldProp == nil {
			t.Fatal("missing field")
		}

		if fieldProp.Schema == nil {
			t.Fatal("missing schema")
		}

		if fieldProp.Schema.Type == nil {
			t.Fatal("missing type")
		}

		stringField, ok := fieldProp.Schema.Type.(*schema_j5pb.Field_String_)
		if !ok {
			t.Fatalf("wrong type %T", fieldProp.Schema.Type)
		}

		if stringField == nil {
			t.Fatal("Field_String_ was nil in oneof, but was set to the type")
		}

		if stringField.String_ == nil {
			t.Fatal("Field_String_.String_ was nil in oneof, but was set to the type")
		}

	})
}

func TestImplicitOneofName(t *testing.T) {
	file := build()
	obj := file.addOneof("Foo")
	field := obj.addOption("bar")
	fieldObj := field.inlineObject("")
	fieldObj.addField("bar_id").setSchema(basicString())

	file.run(t, `
		oneof Foo {
			option bar object {
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
	evt.addField("bar").setSchema(basicString())
	ent.addKey("foo_id").setSchema(basicKey())

	ent.addStatus("S1", "S1 Desc")
	ent.addStatus("S2", "S2 Desc")

	file.run(t, `
		entity Foo {
			event DoThing {
				field bar string
			}

			key foo_id key:uuid

			status S1 | S1 Desc
			status S2 | S2 Desc
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
	obj := file.addObject("Foo")
	obj.obj.Description = "Foo Object Description"
	key := obj.addField("foo_id")

	key.setSchema(basicKey())
	key.setRequired()

	obj.addField("bar_field").
		setRequired().
		setSchema(basicString(func(s *schema_j5pb.StringField) {
			s.Rules = &schema_j5pb.StringField_Rules{MinLength: ptr(uint64(1))}
		}))

	baz := obj.addField("baz_field")
	baz.refObject("path.to", "Type")

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
