package j5parse

import (
	"github.com/pentops/bcl.go/internal/walker/schema"
)

var globalDefs = map[string]*schema.BlockSpec{
	"j5.schema.v1.Ref": {
		Name: &schema.Tag{
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
	},
	"j5.schema.v1.KeyFormat": {
		TypeSelect: &schema.Tag{
			Path: []string{},
		},
	},
	"j5.schema.v1.ArrayField": {
		Qualifier: &schema.Tag{
			Path:    []string{"items"},
			IsBlock: true,
		},
	},
	"j5.schema.v1.KeyField": {
		Qualifier: &schema.Tag{
			Path:    []string{"format"},
			IsBlock: true,
		},
	},
	"j5.schema.v1.Field": {
		TypeSelect: &schema.Tag{
			Path: []string{}, // self
		},
	},
	"j5.schema.v1.ObjectProperty": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		TypeSelect: &schema.Tag{
			Path: []string{"schema"},
		},
	},
	"j5.schema.v1.ObjectField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:        schema.PathSpec{"object", "properties"},
				IsContainer: true,
			},
		},
	},
	"j5.schema.v1.OneofField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:        schema.PathSpec{"oneof", "properties"},
				IsContainer: true,
			},
		},
	},
	"j5.sourcedef.v1.Object": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
			"object": {
				Path:         schema.PathSpec{"schemas", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.schema.v1.Object": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"field": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.sourcedef.v1.Oneof": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:         schema.PathSpec{"properties"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.schema.v1.Enum": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"option": {
				Path:         schema.PathSpec{"options"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.sourcedef.v1.Entity": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Children: map[string]schema.ChildSpec{
			"key": {
				Path:         schema.PathSpec{"keys"},
				IsContainer:  true,
				IsCollection: true,
			},
			"data": {
				Path:        schema.PathSpec{"data"},
				IsContainer: true,
			},
			"status": {
				Path:         schema.PathSpec{"status"},
				IsCollection: true,
				IsScalar:     true,
			},
			"event": {
				Path:         schema.PathSpec{"events"},
				IsContainer:  true,
				IsCollection: true,
			},
			"object": {
				Path:         schema.PathSpec{"schemas", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
			"enum": {
				Path:         schema.PathSpec{"schemas", "enum"},
				IsContainer:  true,
				IsCollection: true,
			},
			"oneof": {
				Path:         schema.PathSpec{"schemas", "oneof"},
				IsContainer:  true,
				IsCollection: true,
			},
		},
	},
	"j5.sourcedef.v1.Partial_Field": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		TypeSelect: &schema.Tag{
			Path: []string{"schema"},
		},
	},
	"j5.sourcedef.v1.Partial": {
		Description: []string{"description"},
		TypeSelect: &schema.Tag{
			Path: []string{},
		},
	},

	"j5.sourcedef.v1.SourceFile": {
		Description: []string{"description"},
		//OnlyDefined: true,

		Children: map[string]schema.ChildSpec{
			"object": {
				Path:         schema.PathSpec{"elements", "object"},
				IsContainer:  true,
				IsCollection: true,
			},
			"enum": {
				Path:         schema.PathSpec{"elements", "enum"},
				IsContainer:  true,
				IsCollection: true,
			},
			"oneof": {
				Path:         schema.PathSpec{"elements", "oneof"},
				IsContainer:  true,
				IsCollection: true,
			},
			"entity": {
				Path:         schema.PathSpec{"elements", "entity"},
				IsContainer:  true,
				IsCollection: true,
			},

			"partial": {
				Path: schema.PathSpec{"elements", "partial"},
			},
			//"import": {
			//	Path: schema.PathSpec{"imports"},
			//},

			"version": {
				Path:     schema.PathSpec{"version"},
				IsScalar: true,
			},
		},
	},
}

var Spec = &schema.ConversionSpec{
	GlobalDefs: globalDefs,
}
