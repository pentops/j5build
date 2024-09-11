package j5parse

import (
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
)

func bclPath(strings ...string) *bcl_j5pb.Path {
	return &bcl_j5pb.Path{Path: strings}
}

func ptr[T any](v T) *T {
	return &v
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
			Path: bclPath("path"),
		},
		Qualifier: &bcl_j5pb.Tag{
			Path: bclPath("alias"),
		},
	}, {
		SchemaName: "j5.schema.v1.KeyFormat",
		TypeSelect: &bcl_j5pb.Tag{
			Path: bclPath(),
		},
	}, {
		SchemaName: "j5.schema.v1.ArrayField",
		Qualifier: &bcl_j5pb.Tag{
			Path:    bclPath("items"),
			IsBlock: true,
		},
	}, {
		SchemaName: "j5.schema.v1.KeyField",
		Qualifier: &bcl_j5pb.Tag{
			Path:    bclPath("format"),
			IsBlock: true,
		},
		Children: []*bcl_j5pb.Child{{
			Name: "primary",
			Path: bclPath("entity", "primaryKey"),
		}, {
			Name: "foreign",
			Path: bclPath("entity", "foreignKey"),
		}},
	}, {
		SchemaName: "j5.schema.v1.IntegerField",
		Qualifier: &bcl_j5pb.Tag{
			Path: bclPath("format"),
		},
	}, {
		SchemaName: "j5.schema.v1.Field",
		TypeSelect: &bcl_j5pb.Tag{
			Path:         bclPath(),
			BangBool:     bclPath("required"),
			QuestionBool: bclPath("optional"),
		},
	}, {
		SchemaName: "j5.schema.v1.ObjectProperty",
		Name: &bcl_j5pb.Tag{
			Path: bclPath("name"),
		},
		TypeSelect: &bcl_j5pb.Tag{
			Path:         bclPath("schema"),
			BangBool:     bclPath("required"),
			QuestionBool: bclPath("optional"),
		},
		Children: []*bcl_j5pb.Child{{
			Name: "optional",
			Path: bclPath("explicitlyOptional"),
		}},
	}, {
		SchemaName: "j5.schema.v1.ObjectField",
		Qualifier: &bcl_j5pb.Tag{
			Path: bclPath("ref"),
		},
		Children: []*bcl_j5pb.Child{{
			Name:        "field",
			Path:        bclPath("object", "properties"),
			IsContainer: true,
		}},
	}, {
		SchemaName: "j5.schema.v1.OneofField",
		Qualifier: &bcl_j5pb.Tag{
			Path: bclPath("ref"),
		},
		Children: []*bcl_j5pb.Child{{
			Name:        "option",
			Path:        bclPath("oneof", "properties"),
			IsContainer: true,
		}},
	}, {
		SchemaName: "j5.schema.v1.EnumField",
		Qualifier: &bcl_j5pb.Tag{
			Path: bclPath("ref"),
		},
	}, {
		SchemaName:  "j5.sourcedef.v1.Object",
		Name:        &bcl_j5pb.Tag{Path: bclPath("name")},
		Description: bclPath("description"),
		Children: []*bcl_j5pb.Child{{
			Name:         "field",
			Path:         bclPath("properties"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "object",
			Path:         bclPath("schemas", "object"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName:  "j5.schema.v1.Object",
		Name:        &bcl_j5pb.Tag{Path: bclPath("name")},
		Description: bclPath("description"),
		Children: []*bcl_j5pb.Child{{
			Name:         "field",
			Path:         bclPath("properties"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName:  "j5.sourcedef.v1.Oneof",
		Name:        &bcl_j5pb.Tag{Path: bclPath("name")},
		Description: bclPath("description"),
		Children: []*bcl_j5pb.Child{{
			Name:         "option",
			Path:         bclPath("properties"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName:  "j5.schema.v1.Oneof",
		Name:        &bcl_j5pb.Tag{Path: bclPath("name")},
		Description: bclPath("description"),
		Children: []*bcl_j5pb.Child{{
			Name:         "option",
			Path:         bclPath("properties"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName:  "j5.schema.v1.Enum",
		Name:        &bcl_j5pb.Tag{Path: bclPath("name")},
		Description: bclPath("description"),
		Children: []*bcl_j5pb.Child{{
			Name:         "option",
			Path:         bclPath("options"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName: "j5.sourcedef.v1.Topic",
		Name:       &bcl_j5pb.Tag{Path: bclPath("name")},
		TypeSelect: &bcl_j5pb.Tag{Path: bclPath("type")},
	}, {
		SchemaName: "j5.sourcedef.v1.Entity",
		Name:       &bcl_j5pb.Tag{Path: bclPath("name")},
		Children: []*bcl_j5pb.Child{{
			Name:         "key",
			Path:         bclPath("keys"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:        "data",
			Path:        bclPath("data"),
			IsContainer: true,
		}, {
			Name:         "status",
			Path:         bclPath("status"),
			IsCollection: true,
			IsScalar:     true,
		}, {
			Name:         "event",
			Path:         bclPath("events"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "object",
			Path:         bclPath("schemas", "object"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "enum",
			Path:         bclPath("schemas", "enum"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "oneof",
			Path:         bclPath("schemas", "oneof"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}, {
		SchemaName: "j5.sourcedef.v1.SourceFile",
		Children: []*bcl_j5pb.Child{{
			Name:         "object",
			Path:         bclPath("elements", "object"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:     "package",
			Path:     bclPath("package"),
			IsScalar: true,
		}, {
			Name:         "enum",
			Path:         bclPath("elements", "enum"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "oneof",
			Path:         bclPath("elements", "oneof"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "entity",
			Path:         bclPath("elements", "entity"),
			IsContainer:  true,
			IsCollection: true,
		}, {
			Name:         "topic",
			Path:         bclPath("elements", "topic"),
			IsContainer:  true,
			IsCollection: true,
		}},
	}},
}
