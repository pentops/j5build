package j5convert

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

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

func (e *EnumBuilder) addValue(name string, number int32, description string) {
	if !strings.HasPrefix(name, e.prefix) {
		name = e.prefix + name
	}
	value := &descriptorpb.EnumValueDescriptorProto{
		Name:   ptr(name),
		Number: ptr(number),
	}
	if number == 0 {
		e.desc.Value[0] = value
	} else {
		e.desc.Value = append(e.desc.Value, value)
	}
	e.comment([]int32{2, number}, description)
}

func buildEnum(schema *schema_j5pb.Enum) *EnumBuilder {
	eb := emptyEnum(schema.Name, schema.Prefix)
	if schema.Description != "" {
		eb.comment([]int32{}, schema.Description)
	}

	for _, value := range schema.Options {
		eb.addValue(value.Name, value.Number, value.Description)
	}

	return eb

}
