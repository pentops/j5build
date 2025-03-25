package integration

import (
	"strings"
	"testing"

	"github.com/pentops/j5build/internal/bcl"
	"github.com/pentops/j5build/internal/bcl/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5build/internal/bcl/gen/test/v1/test_j5pb"
	"github.com/stretchr/testify/assert"
)

func TestEndToEnd(t *testing.T) {

	schema := &bcl_j5pb.Schema{
		Blocks: []*bcl_j5pb.Block{{
			SchemaName: "test.v1.File",
			Alias: []*bcl_j5pb.Alias{{
				Name: "foo",
				Path: &bcl_j5pb.Path{Path: []string{"elements", "foo"}},
			}, {
				Name: "bar",
				Path: &bcl_j5pb.Path{Path: []string{"elements", "bar"}},
			}},
		}},
	}

	pp, err := bcl.NewParser(schema)
	if err != nil {
		t.Fatal(err)
	}
	pp.Verbose = true

	run := func(t testing.TB, input string) *test_j5pb.File {
		msg := &test_j5pb.File{}
		locs, err := pp.ParseFile("in.bcl", input, msg.ProtoReflect())
		if err != nil {
			t.Fatal(err)
		}
		msg.SourceLocation = locs
		return msg
	}

	t.Run("basic", func(t *testing.T) {
		msg := run(t, fb(
			`sString = "foo"`,
			`rString = ["a","b"]`,
			`foo Name {`,
			`  | Description Text`,
			`}`,
		))

		assert.Equal(t, "foo", msg.SString)
		assert.Equal(t, []string{"a", "b"}, msg.RString)
		if len(msg.Elements) != 1 {
			t.Fatalf("expected 1 element, got %d", len(msg.Elements))
		}

		foo := msg.Elements[0].GetFoo()
		if foo == nil {
			t.Fatalf("expected foo type")
		}
		assert.Equal(t, "Name", foo.Name)
		assert.Equal(t, "Description Text", foo.Description)

		locs := msg.SourceLocation
		assertLoc(t, locs, "sString", 0)
		assertLoc(t, locs, "rString", 1)
	})

	t.Run("map", func(t *testing.T) {
		msg := run(t, fb(
			`tag.a = "a-val"`,
			`tag.b = "b-val"`,
		))

		assert.Equal(t, "a-val", msg.Tags["a"])
		assert.Equal(t, "b-val", msg.Tags["b"])
	})

	t.Run("array flat", func(t *testing.T) {
		msg := run(t, fb(
			`rString = ["a", "b"]`,
			`rString += "c"`,
		))

		assert.Equal(t, []string{"a", "b", "c"}, msg.RString)
	})

}

func assertLoc(t *testing.T, walk *bcl_j5pb.SourceLocation, name string, startLine int32) {
	parts := strings.Split(name, ".")
	for _, part := range parts {
		child, ok := walk.Children[part]
		if !ok {
			t.Errorf("could not find loc for %s", name)
			return
		}
		walk = child
	}

	if walk.StartLine != startLine {

		t.Errorf("expected line %d, got %d", startLine, walk.StartLine)
	}
}
func fb(s ...string) string {
	return strings.Join(s, "\n")
}
