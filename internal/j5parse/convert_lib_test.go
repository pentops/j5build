package j5parse

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

type fileBuild struct {
	file *sourcedef_j5pb.SourceFile
}

func (fb *fileBuild) run(t *testing.T, input string, cb ...func(*testing.T, *sourcedef_j5pb.SourceFile)) {
	t.Helper()
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("FATAL: %s", err)
	}
	//	parser.Verbose = true
	got, err := parser.ParseFile("pentops/j5lang/example/example.ext", input)
	if err != nil {
		if pe, ok := errpos.AsErrorsWithSource(err); ok {
			t.Logf(pe.HumanString(3))
		}
		t.Fatalf("FATAL: %s", err)
	}

	got.SourceLocations = nil
	got.Path = ""

	cmpProto(t, fb.file, got)
	for _, c := range cb {
		c(t, got)
	}
}

func runToErrors(t *testing.T, input string, cb ...func(*testing.T, *errpos.ErrorsWithSource)) {
	t.Helper()
	parser, err := NewParser()
	if err != nil {
		t.Fatalf("FATAL: %s", err)
	}
	parser.Verbose = true
	got, err := parser.ParseFile("pentops/j5lang/example/example.ext", input)
	if err == nil {
		got.SourceLocations = nil
		t.Logf("Expected error, but got this result: \n%s", prototext.Format(got))
		t.Fatalf("Expected an error, in runToErrors, got nil")
	}
	pe, ok := errpos.AsErrorsWithSource(err)
	if !ok {
		t.Fatalf("FATAL: %s", err)
	}

	for _, c := range cb {
		c(t, pe)
	}

}

func build() *fileBuild {
	return &fileBuild{
		file: &sourcedef_j5pb.SourceFile{
			Package: "pentops.j5lang.example",
		},
	}
}

type objBuild struct {
	obj *schema_j5pb.Object
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
	return &objBuild{obj: obj.Def}
}

func (o *objBuild) addField(name string) *fieldBuild {
	prop := &schema_j5pb.ObjectProperty{
		Name: name,
	}
	num := int32(len(o.obj.Properties) + 1)
	o.obj.Properties = append(o.obj.Properties, prop)
	prop.ProtoField = []int32{num}

	return &fieldBuild{prop: prop}
}

type oneofBuild struct {
	obj *sourcedef_j5pb.Oneof
}

func (o *oneofBuild) addOption(name string) *fieldBuild {
	prop := &schema_j5pb.ObjectProperty{
		Name: name,
	}
	num := int32(len(o.obj.Def.Properties) + 1)
	o.obj.Def.Properties = append(o.obj.Def.Properties, prop)
	prop.ProtoField = []int32{num}

	return &fieldBuild{prop: prop}
}

func (f *fileBuild) addOneof(name string) *oneofBuild {
	obj := &sourcedef_j5pb.Oneof{
		Def: &schema_j5pb.Oneof{
			Name: name,
		},
	}
	f.file.Elements = append(f.file.Elements, &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Oneof{
			Oneof: obj,
		},
	})
	return &oneofBuild{obj: obj}
}

func (f *fileBuild) addEnum(name string) *enumBuild {
	enum := &schema_j5pb.Enum{
		Name: name,
	}
	f.file.Elements = append(f.file.Elements, &sourcedef_j5pb.RootElement{
		Type: &sourcedef_j5pb.RootElement_Enum{
			Enum: enum,
		},
	})
	return &enumBuild{enum: enum}
}

type enumBuild struct {
	enum *schema_j5pb.Enum
}

func (e *enumBuild) addOption(name string) {
	e.enum.Options = append(e.enum.Options, &schema_j5pb.Enum_Option{
		Name:   name,
		Number: int32(len(e.enum.Options) + 1),
	})
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

func basicString(mod ...func(s *schema_j5pb.StringField)) *schema_j5pb.Field_String_ {
	base := &schema_j5pb.Field_String_{
		String_: &schema_j5pb.StringField{},
	}
	for _, m := range mod {
		m(base.String_)
	}
	return base
}

func basicKey() *schema_j5pb.Field_Key {
	return &schema_j5pb.Field_Key{
		Key: &schema_j5pb.KeyField{
			Format: &schema_j5pb.KeyFormat{
				Type: &schema_j5pb.KeyFormat_Uuid{
					Uuid: &schema_j5pb.KeyFormat_UUID{},
				},
			},
		},
	}
}

func (f *fieldBuild) refObject(pkg string, name string) {
	f.prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Ref{
					Ref: &schema_j5pb.Ref{
						Package: pkg,
						Schema:  name,
					},
				},
			},
		},
	}
}

func (f *fieldBuild) inlineObject(name string) *objBuild {
	obj := &schema_j5pb.Object{
		Name: name,
	}
	f.prop.Schema = &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Object{
					Object: obj,
				},
			},
		},
	}
	return &objBuild{obj: obj}
}

func (f *fileBuild) addEntity(name string) *entityBuild {
	obj := &sourcedef_j5pb.Entity{
		Name: name,
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

func (eb *entityBuild) addKey(name string) *fieldBuild {
	prop := &schema_j5pb.ObjectProperty{
		Name: name,
	}
	num := int32(len(eb.obj.Keys) + 1)
	eb.obj.Keys = append(eb.obj.Keys, prop)
	prop.ProtoField = []int32{num}
	return &fieldBuild{prop: prop}
}

func (eb *entityBuild) addEvent(name string) *objBuild {
	evt := &sourcedef_j5pb.Object{
		Def: &schema_j5pb.Object{
			Name: name,
		},
	}

	eb.obj.Events = append(eb.obj.Events, evt)
	return &objBuild{obj: evt.Def}
}

func (eb *entityBuild) addStatus(name string, description string) {
	eb.obj.Status = append(eb.obj.Status, &schema_j5pb.Enum_Option{
		Name:        name,
		Description: description,
		Number:      int32(len(eb.obj.Status) + 1),
	})
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

type rootBuilder struct {
	lines []string
	sourceContext
}

func (sb *rootBuilder) addLine(s string) {
	sb.lines = append(sb.lines, s)
}

type sourceBuilder interface {
	p(...string)
	indent() sourceBuilder
}

type sourceContext struct {
	collector interface{ addLine(string) }
	prefix    string
}

func (sb *sourceContext) p(s ...string) {
	sb.collector.addLine(sb.prefix + strings.Join(s, ""))
}

func (sb *sourceContext) indent() sourceBuilder {
	return &sourceContext{
		collector: sb.collector,
		prefix:    sb.prefix + "  ",
	}
}

func buildSource() *blockBuilder {
	return &blockBuilder{}
}

func (sb *rootBuilder) toString() string {
	return strings.Join(sb.lines, "\n")
}

type blockBuilder struct {
	_tags []string
	_qual []string
	body  []statementBuilder
}

func (bb *blockBuilder) qualifiers(s ...string) {
	bb._qual = append(bb._qual, s...)
}

func (bb *blockBuilder) write(p sourceBuilder) {
	suffix := " {"
	if len(bb.body) == 0 {
		suffix = ""
	}
	if len(bb._qual) > 0 {
		p.p(
			strings.Join(bb._tags, " "),
			":",
			strings.Join(bb._qual, ":"),
			suffix,
		)
	} else {

		p.p(strings.Join(bb._tags, " "), suffix)
	}
	if len(bb.body) == 0 {
		return
	}
	ind := p.indent()
	for _, stmt := range bb.body {
		stmt.write(ind)
	}
	p.p("}")

}

type attributeBuilder struct {
	name  string
	value string
}

func (ab *attributeBuilder) write(p sourceBuilder) {
	p.p(ab.name + " = " + ab.value)
}

type statementBuilder interface {
	write(p sourceBuilder)
}

func (sb *blockBuilder) block(tags ...string) *blockBuilder {
	block := &blockBuilder{
		_tags: tags,
	}
	sb.body = append(sb.body, block)
	return block
}

func (sb *blockBuilder) attr(name, value string) {
	sb.body = append(sb.body, &attributeBuilder{
		name:  name,
		value: value,
	})
}

func buildString(blk *blockBuilder) string {
	sb := &rootBuilder{}
	blk.write(&sourceContext{collector: sb})
	return sb.toString()
}
