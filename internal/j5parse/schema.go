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
	"j5.schema.v1.ArrayField": {
		Qualifier: &schema.Tag{
			Path: []string{"items"},
		},
	},
	"j5.schema.v1.KeyField": {
		Qualifier: &schema.Tag{
			Path: []string{"format"},
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
		Blocks: map[string]schema.PathSpec{
			"field": {"object", "properties"},
		},
	},
	"j5.schema.v1.OneofField": {
		Qualifier: &schema.Tag{
			Path:     []string{"ref"},
			SplitRef: [][]string{{"package"}, {"schema"}},
		},
		Blocks: map[string]schema.PathSpec{
			"option": {"oneof", "properties"},
		},
	},
	"j5.sourcedef.v1.Object": {
		Description: []string{"description"},
		Location:    []string{"location"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]schema.PathSpec{
			"field": {"properties"},
		},
	},
	"j5.sourcedef.v1.Oneof": {
		Description: []string{"description"},
		Location:    []string{"location"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]schema.PathSpec{
			"option": {"properties"},
		},
	},
	"j5.schema.v1.Enum": {
		Description: []string{"description"},
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Blocks: map[string]schema.PathSpec{
			"option": {"options"},
		},
	},
	"j5.sourcedef.v1.Entity": {
		Name: &schema.Tag{
			Path: []string{"name"},
		},
		Location: []string{"location"},
		Blocks: map[string]schema.PathSpec{
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
		Attributes: map[string]schema.PathSpec{
			"version": {"version"},
		},
		Description: []string{"description"},
		Blocks: map[string]schema.PathSpec{
			"object": {"elements", "object"},
			"enum":   {"elements", "enum"},
			"oneof":  {"elements", "enum"},
			"entity": {"elements", "entity"},
		},
	},
}

var Spec = &schema.ConversionSpec{
	GlobalDefs: globalDefs,
}
