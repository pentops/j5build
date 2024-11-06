package j5convert

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/internal/bcl/sourcewalk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func convertEnum(ww *walkContext, node *sourcewalk.EnumNode) {
	eb := emptyEnum(node.Schema.Name, node.Schema.Prefix)
	if node.Schema.Description != "" {
		eb.comment([]int32{}, node.Schema.Description)
	}

	if node.Schema.Info != nil {
		ext := &ext_j5pb.EnumOptions{}

		for _, field := range node.Schema.Info {
			ext.InfoFields = append(ext.InfoFields, &ext_j5pb.EnumInfoField{
				Name:        field.Name,
				Label:       field.Label,
				Description: field.Description,
			})
		}

		eb.desc.Options = &descriptorpb.EnumOptions{}
		proto.SetExtension(eb.desc.Options, ext_j5pb.E_Enum, ext)
	}

	optionsToSet := node.Schema.Options
	if len(optionsToSet) > 0 && optionsToSet[0].Number == 0 && strings.HasSuffix(optionsToSet[0].Name, "UNSPECIFIED") {
		eb.addValue(0, optionsToSet[0])
		optionsToSet = optionsToSet[1:]
	}

	for idx, value := range optionsToSet {
		eb.addValue(int32(idx+1), value)
	}

	ww.parentContext.addEnum(eb)
}

type EnumBuilder struct {
	desc   *descriptorpb.EnumDescriptorProto
	prefix string

	commentSet
}

func emptyEnum(name string, prefix string) *EnumBuilder {
	if prefix == "" {
		prefix = strcase.ToScreamingSnake(name) + "_"
	}
	return &EnumBuilder{
		prefix: prefix,
		desc: &descriptorpb.EnumDescriptorProto{
			Name: ptr(name),
			Value: []*descriptorpb.EnumValueDescriptorProto{{
				Name:   ptr(fmt.Sprintf("%sUNSPECIFIED", prefix)),
				Number: ptr(int32(0)),
			}},
		},
	}
}

func (e *EnumBuilder) addValue(number int32, schema *schema_j5pb.Enum_Option) {
	name := schema.Name
	if !strings.HasPrefix(name, e.prefix) {
		name = e.prefix + name
	}
	value := &descriptorpb.EnumValueDescriptorProto{
		Name:   ptr(name),
		Number: ptr(number),
	}

	if len(schema.Info) > 0 {
		value.Options = &descriptorpb.EnumValueOptions{}
		proto.SetExtension(value.Options, ext_j5pb.E_EnumValue, &ext_j5pb.EnumValueOptions{
			Info: schema.Info,
		})
	}

	if number == 0 {
		e.desc.Value[0] = value
	} else {
		e.desc.Value = append(e.desc.Value, value)
	}
	if schema.Description != "" {
		e.comment([]int32{2, number}, schema.Description)
	}

}
