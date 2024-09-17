package sourcewalk

import (
	"testing"

	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func walkLoc(walk *bcl_j5pb.SourceLocation, path ...string) *bcl_j5pb.SourceLocation {
	for _, part := range path {
		if walk.Children == nil {
			walk.Children = map[string]*bcl_j5pb.SourceLocation{}
		}
		nextLoc := walk.Children[part]
		if nextLoc == nil {
			nextLoc = &bcl_j5pb.SourceLocation{}
			walk.Children[part] = nextLoc
		}
		walk = nextLoc
	}
	return walk
}

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

					Name: "fooId",
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_String_{},
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
		SourceLocations: &bcl_j5pb.SourceLocation{
			StartLine: 1,
		},
	}

	entSrc := walkLoc(file.SourceLocations, "elements", "0", "entity")
	entSrc.StartLine = 1

	keysSrc := walkLoc(entSrc, "keys")
	fooIdSrc := walkLoc(keysSrc, "0")
	fooIdSrc.StartLine = 3
	walkLoc(fooIdSrc, "schema", "string").StartLine = 5

	walkLoc(entSrc, "data").StartLine = 3

	walkLoc(entSrc, "events", "0", "def").StartLine = 5
	walkLoc(entSrc, "events", "0", "def", "properties", "0", "schema", "string").StartLine = 7

	sources := map[string]SourceNode{}
	objects := map[string]*ObjectNode{}

	visitor := &DefaultVisitor{
		Object: func(obj *ObjectNode) error {
			t.Logf("ADD %s: Object %s at %s", obj.Source.PathString(), obj.Schema.Name, obj.Source.GetPos())
			sources[obj.Source.PathString()] = obj.Source
			objects[obj.Schema.Name] = obj
			return nil
		},
		Property: func(prop *PropertyNode) error {
			t.Logf("ADD %s: Property %s at %s", prop.Source.PathString(), prop.Schema.Name, prop.Source.GetPos())
			sources[prop.Source.PathString()] = prop.Source
			return nil
		},
	}

	err := NewRoot(file).RangeRootElements(visitor)
	if err != nil {
		t.Fatal(err.Error())
	}

	assert := func(req string, line int32) {
		got, ok := sources[req]
		if !ok {
			t.Errorf("Missing %s", req)
			return
		}
		if got.Source == nil {
			t.Errorf("No Source at %s", req)
		}
		if got.Source.StartLine != line {
			t.Errorf("Line %d for %s, want %d", got.Source.StartLine, req, line)
		}
	}

	assert("elements.0.entity.keys.0", 3)

}
