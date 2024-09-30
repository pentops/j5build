package j5parse

import (
	"path"
	"strings"

	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func bclPath(strings ...string) *bcl_j5pb.Path {
	return &bcl_j5pb.Path{Path: strings}
}

func ptr[T any](v T) *T {
	return &v
}

func FileStub(sourceFilename string) protoreflect.Message {
	dirName, _ := path.Split(sourceFilename)
	dirName = strings.TrimSuffix(dirName, "/")

	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	file := &sourcedef_j5pb.SourceFile{
		Path: sourceFilename,
		Package: &sourcedef_j5pb.Package{
			Name: pathPackage,
		},
		SourceLocations: &bcl_j5pb.SourceLocation{},
	}
	refl := file.ProtoReflect()

	return refl
}

var J5SchemaSpec = &bcl_j5pb.Schema{
	Blocks: []*bcl_j5pb.Block{{
		SchemaName: "j5.schema.v1.Ref",
		ScalarSplit: &bcl_j5pb.ScalarSplit{
			Delimiter:      ptr("."),
			RightToLeft:    true,
			RequiredFields: []*bcl_j5pb.Path{bclPath("schema")},
			RemainderField: bclPath("package"),
		},
	}, {
		SchemaName: "j5.schema.v1.EntityRef",
		ScalarSplit: &bcl_j5pb.ScalarSplit{
			Delimiter:      ptr("."),
			RightToLeft:    true,
			RequiredFields: []*bcl_j5pb.Path{bclPath("entity")},
			RemainderField: bclPath("package"),
		},
	}, {
		SchemaName: "j5.sourcedef.v1.Import",
		Name: &bcl_j5pb.Tag{
			FieldName: ("path"),
		},
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("alias"),
		},
	}, {
		SchemaName: "j5.schema.v1.KeyFormat",
		TypeSelect: &bcl_j5pb.Tag{
			FieldName: ".",
		},
	}, {
		SchemaName: "j5.schema.v1.AnyField",
		Alias: []*bcl_j5pb.Alias{{
			Name: "type",
			Path: bclPath("types"),
		}},
	}, {
		SchemaName: "j5.schema.v1.ArrayField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("items"),
			IsBlock:   true,
		},
	}, {
		SchemaName: "j5.schema.v1.KeyField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("format"),
			IsBlock:   true,
		},
		Alias: []*bcl_j5pb.Alias{{
			Name: "primary",
			Path: bclPath("entity", "primaryKey"),
		}, {
			Name: "foreign",
			Path: bclPath("entity", "foreignKey"),
		}},
	}, {
		SchemaName: "j5.schema.v1.IntegerField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("format"),
		},
	}, {
		SchemaName: "j5.schema.v1.Field",
		TypeSelect: &bcl_j5pb.Tag{
			FieldName:    ".",
			BangBool:     ptr("required"),
			QuestionBool: ptr("optional"),
		},
	}, {
		SchemaName: "j5.schema.v1.ObjectProperty",
		Name: &bcl_j5pb.Tag{
			FieldName: ("name"),
		},
		TypeSelect: &bcl_j5pb.Tag{
			FieldName:    ("schema"),
			BangBool:     ptr("required"),
			QuestionBool: ptr("optional"),
		},
		Alias: []*bcl_j5pb.Alias{{
			Name: "optional",
			Path: bclPath("explicitlyOptional"),
		}},
	}, {
		SchemaName: "j5.schema.v1.ObjectField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("ref"),
		},
		Alias: []*bcl_j5pb.Alias{{
			Name: "field",
			Path: bclPath("object", "properties"),
		}},
	}, {
		SchemaName: "j5.schema.v1.OneofField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("ref"),
		},
		Alias: []*bcl_j5pb.Alias{{
			Name: "option",
			Path: bclPath("oneof", "properties"),
		}},
	}, {
		SchemaName: "j5.schema.v1.EnumField",
		Qualifier: &bcl_j5pb.Tag{
			FieldName: ("ref"),
		},
	}, {
		SchemaName:       "j5.sourcedef.v1.Object",
		Name:             &bcl_j5pb.Tag{FieldName: ("name")},
		DescriptionField: ptr("description"),
		Alias: []*bcl_j5pb.Alias{{
			Name: "field",
			Path: bclPath("properties"),
		}, {
			Name: "object",
			Path: bclPath("schemas", "object"),
		}},
	}, {
		SchemaName:       "j5.schema.v1.Object",
		Name:             &bcl_j5pb.Tag{FieldName: ("name")},
		DescriptionField: ptr("description"),
		Alias: []*bcl_j5pb.Alias{{
			Name: "field",
			Path: bclPath("properties"),
		}},
	}, {
		SchemaName:       "j5.sourcedef.v1.Oneof",
		Name:             &bcl_j5pb.Tag{FieldName: ("name")},
		DescriptionField: ptr("description"),
		Alias: []*bcl_j5pb.Alias{{
			Name: "option",
			Path: bclPath("properties"),
		}},
	}, {
		SchemaName:       "j5.schema.v1.Oneof",
		Name:             &bcl_j5pb.Tag{FieldName: ("name")},
		DescriptionField: ptr("description"),
		Alias: []*bcl_j5pb.Alias{{
			Name: "option",
			Path: bclPath("properties"),
		}},
	}, {
		SchemaName:       "j5.schema.v1.Enum",
		Name:             &bcl_j5pb.Tag{FieldName: ("name")},
		DescriptionField: ptr("description"),
		Alias: []*bcl_j5pb.Alias{{
			Name: "option",
			Path: bclPath("options"),
		}},
	}, {
		SchemaName: "j5.sourcedef.v1.Topic",
		Name:       &bcl_j5pb.Tag{FieldName: ("name")},
		TypeSelect: &bcl_j5pb.Tag{FieldName: ("type")},
	}, {
		SchemaName: "j5.sourcedef.v1.Entity",
		Name:       &bcl_j5pb.Tag{FieldName: ("name")},
		Alias: []*bcl_j5pb.Alias{{
			Name: "key",
			Path: bclPath("keys"),
		}, {
			Name: "data",
			Path: bclPath("data"),
		}, {
			Name: "status",
			Path: bclPath("status"),
		}, {
			Name: "event",
			Path: bclPath("events"),
		}, {
			Name: "object",
			Path: bclPath("schemas", "object"),
		}, {
			Name: "enum",
			Path: bclPath("schemas", "enum"),
		}, {
			Name: "oneof",
			Path: bclPath("schemas", "oneof"),
		}},
	}, {
		SchemaName: "j5.sourcedef.v1.SourceFile",
		Alias: []*bcl_j5pb.Alias{{
			Name: "object",
			Path: bclPath("elements", "object"),
		}, {
			Name: "package",
			Path: bclPath("package"),
		}, {
			Name: "enum",
			Path: bclPath("elements", "enum"),
		}, {
			Name: "oneof",
			Path: bclPath("elements", "oneof"),
		}, {
			Name: "entity",
			Path: bclPath("elements", "entity"),
		}, {
			Name: "topic",
			Path: bclPath("elements", "topic"),
		}},
	}},
}
