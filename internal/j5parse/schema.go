package j5parse

import (
	"github.com/pentops/bcl.go/internal/walker"
)

var globalDefs = map[string]*walker.BlockSpec{
	"j5.schema.v1.Ref": {
		Name: &walker.Tag{
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
	},
	"j5.schema.v1.ArrayField": {
		Qualifier: &walker.Tag{
			Path: []string{"items"},
		},
	},
	"j5.schema.v1.KeyField": {
		Qualifier: &walker.Tag{
			Path: []string{"format"},
		},
	},
	"j5.schema.v1.ObjectProperty": {
		Name: &walker.Tag{
			Path: []string{"name"},
		},
		TypeSelect: &walker.Tag{
			Path: []string{"schema"},
		},
		/*
			}, {
				Type: walker.TagTypeAppendContext,
				Path: []string{"schema"},
			}},*/
	},
	"j5.schema.v1.ObjectField": {
		Qualifier: &walker.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Blocks: map[string]walker.PathSpec{
			"field": {"object", "properties"},
		},
	},
	"j5.schema.v1.OneofField": {
		Qualifier: &walker.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Blocks: map[string]walker.PathSpec{
			"option": {"oneof", "properties"},
			//	"ref": {
			//		Path: []string{"ref"},
			//	},
		},
	},
	"j5.sourcedef.v1.Object": {
		Description: []string{"description"},
		Location:    []string{"location"},
		Name: &walker.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]walker.PathSpec{
			"field": {"properties"},
		},
	},
	"j5.sourcedef.v1.Oneof": {
		Description: []string{"description"},
		Location:    []string{"location"},
		Name: &walker.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]walker.PathSpec{
			"option": {"properties"},
		},
	},
	"j5.schema.v1.Enum": {
		Description: []string{"description"},
		Name: &walker.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]walker.PathSpec{
			"option": {"options"},
		},
	},
	"j5.sourcedef.v1.Entity": {
		Name: &walker.Tag{
			Path: []string{"name"},
		},
		Location: []string{"location"},
		Blocks: map[string]walker.PathSpec{
			"key":    {"keys", "properties"},
			"data":   {"data"},
			"status": {"status"},
			"event":  {"events"},

			// Plus ordinary body objects
			"object": {"schemas", "object"},
			"enum":   {"schemas", "enum"},
			"oneof":  {"schemas", "enum"},
		},
	},
	"j5.sourcedef.v1.SourceFile": {
		Attributes: map[string]walker.PathSpec{
			"version": {"version"},
		},
		Description: []string{"description"},
		Blocks: map[string]walker.PathSpec{
			"object": {"elements", "object"},
			"enum":   {"elements", "enum"},
			"oneof":  {"elements", "enum"},
			"entity": {"entities", "entity"},
		},
	},
}

var Spec = &walker.ConversionSpec{
	GlobalDefs: globalDefs,
}
