package sourcewalk

import (
	"testing"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func TestNestedObject(t *testing.T) {

	file := &sourcedef_j5pb.SourceFile{
		Package: &sourcedef_j5pb.Package{
			Name: "test.v1",
		},
		Elements: []*sourcedef_j5pb.RootElement{{
			Type: &sourcedef_j5pb.RootElement_Object{
				Object: &sourcedef_j5pb.Object{
					Def: &schema_j5pb.Object{
						Name: "Foo",
						Properties: []*schema_j5pb.ObjectProperty{{
							Name: "prop",
							Schema: &schema_j5pb.Field{
								Type: &schema_j5pb.Field_Object{
									Object: &schema_j5pb.ObjectField{
										Schema: &schema_j5pb.ObjectField_Object{
											Object: &schema_j5pb.Object{
												Name: "Bar",
												Properties: []*schema_j5pb.ObjectProperty{{
													Name:   "nestedProp",
													Schema: &schema_j5pb.Field{Type: &schema_j5pb.Field_String_{}},
												}},
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

	objects := map[string]*ObjectNode{}

	visitor := &DefaultVisitor{
		Object: func(obj *ObjectNode) error {
			t.Logf("ADD %s: Object at %s", obj.NameInPackage(), obj.Source.PathString())
			objects[obj.NameInPackage()] = obj
			return nil
		},
	}
	err := NewRoot(file).RangeRootElements(visitor)
	if err != nil {
		t.Fatal(err.Error())
	}

	foo := objects["Foo"]
	if foo == nil {
		t.Fatal("Foo not found")
	}
	bar := objects["Foo.Bar"]
	if bar == nil {
		t.Fatal("Bar not found")
	}
	if bar.Name != "Bar" {
		t.Errorf("Bar.Name() = %s, want Bar", bar.Name)
	}

}
