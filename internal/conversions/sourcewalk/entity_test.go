package sourcewalk

import (
	"testing"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/stretchr/testify/assert"
)

func TestEntity(t *testing.T) {

	entitySrc := &sourcedef_j5pb.Entity{
		Name: "foo",
		Keys: []*schema_j5pb.ObjectProperty{{
			Name: "fooId",
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_String_{},
			},
		}},

		Events: []*sourcedef_j5pb.Object{{
			Def: &schema_j5pb.Object{
				Name: "FooCreated",
				Properties: []*schema_j5pb.ObjectProperty{{
					Name: "baz",
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_String_{},
					},
				}, {
					Name: "inline",
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_Object{
							Object: &schema_j5pb.ObjectField{
								Schema: &schema_j5pb.ObjectField_Object{
									Object: &schema_j5pb.Object{
										Name: "Qux",
									},
								},
							},
						},
					},
				}},
			},
		}},
	}

	file := &sourcedef_j5pb.SourceFile{
		Package: &sourcedef_j5pb.Package{
			Name: "test.v1",
		},
		Elements: []*sourcedef_j5pb.RootElement{{
			Type: &sourcedef_j5pb.RootElement_Entity{
				Entity: entitySrc,
			},
		}},
	}

	objects := map[string]*ObjectNode{}
	oneofs := map[string]*OneofNode{}
	properties := map[string]*PropertyNode{}

	visitor := &DefaultVisitor{
		Oneof: func(oneof *OneofNode) error {
			t.Logf("ADD %s: Oneof at %s", oneof.NameInPackage(), oneof.Source.PathString())
			oneofs[oneof.NameInPackage()] = oneof
			return nil
		},
		Object: func(obj *ObjectNode) error {
			t.Logf("ADD %s: Object at %s", obj.NameInPackage(), obj.Source.PathString())
			objects[obj.NameInPackage()] = obj
			return nil
		},
		Property: func(prop *PropertyNode) error {
			t.Logf("ADD %s: Property at %s", prop.NameInPackage(), prop.Source.PathString())
			properties[prop.NameInPackage()] = prop
			return nil
		},
	}

	err := NewRoot(file).RangeRootElements(visitor)
	if err != nil {
		t.Fatal(err.Error())
	}

	if fooKeys, ok := objects["FooKeys"]; !ok {
		t.Errorf("expected keys object")
	} else {
		assert.Equal(t, "elements.0.entity.keys", fooKeys.Source.PathString())
		if fooKeys.Entity == nil {
			t.Fatalf("expected entity schema")
		} else {
			assert.Equal(t, "foo", fooKeys.Entity.Entity)
			assert.Equal(t, "KEYS", fooKeys.Entity.Part.ShortString())
		}
	}

	if prop := properties["FooState.keys"]; prop == nil {
		t.Errorf("expected state keys")
	} else {
		obj := prop.Field.Schema.(*schema_j5pb.Field_Object).Object
		if !obj.Flatten {
			t.Errorf("expected flatten")
		}
	}

	if prop := properties["FooEvent.keys"]; prop == nil {
		t.Errorf("expected state keys")
	} else {
		obj := prop.Field.Schema.(*schema_j5pb.Field_Object).Object
		if !obj.Flatten {
			t.Errorf("expected flatten")
		}
	}

	if fooEventOneof, ok := oneofs["FooEventType"]; !ok {
		t.Errorf("expected event type oneof")
	} else {
		t.Logf("EVENT ONEOF %s", fooEventOneof.Source.PathString())
	}

	if fooCreatedObj, ok := objects["FooEventType.FooCreated"]; !ok {
		t.Errorf("expected FooCreated object")
	} else {
		t.Logf("EVENT OBJECT %s", fooCreatedObj.Source.PathString())
	}

	if fooCreatedField, ok := properties["FooEventType.fooCreated"]; !ok {
		t.Errorf("expected FooCreated field")
	} else {
		t.Logf("EVENT FIELD %s", fooCreatedField.Source.PathString())
		ref := fooCreatedField.Field.Ref
		if ref == nil {
			t.Fatalf("expected ref")
		}
		// not an inline ref, kind-of
		assert.Equal(t, "FooEventType.FooCreated", ref.Ref.Schema)
	}

	if prop, ok := properties["FooEventType.FooCreated.inline"]; !ok {
		t.Errorf("expected inline field")
	} else {
		ref := prop.Field.Ref
		if ref == nil {
			t.Fatalf("expected ref")
		}
		if !ref.Inline {
			t.Fatalf("expected inline ref")
		}

		assert.Equal(t, "FooEventType.FooCreated.Qux", ref.Ref.Schema)
	}

	if obj, ok := objects["FooEventType.FooCreated.Qux"]; !ok {
		t.Errorf("expected inline object")
	} else {
		t.Logf("INLINE OBJECT %s", obj.Source.PathString())
	}

}
