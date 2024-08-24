package j5parse

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type fileBuild struct {
	file *sourcedef_j5pb.SourceFile
}

func (fb *fileBuild) run(t *testing.T, input string) {
	t.Helper()
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

func (f *fieldBuild) setRequired() *fieldBuild {
	f.prop.Required = true
	return f
}

func (f *fieldBuild) setSchema(sch schema_j5pb.IsField_Type) {
	f.prop.Schema = &schema_j5pb.Field{}
	f.prop.Schema.Type = sch
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

func ptr[T any](v T) *T {
	return &v
}

func cmpProto(t testing.TB, want, got proto.Message) {
	t.Helper()
	// a '-' means Removed From, a '+' means Added To
	// - means the 'got' was missing something
	// + means the 'got' had something extra
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Log("Diffs Found. + means the 'got' had something extra, - means the 'got' was missing something")
		t.Log(diff)
		t.Error("Found Diffs")
	}

}
