package j5convert

import (
	"strings"

	"github.com/pentops/golib/gl"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type enumBuilder struct {
	desc   *descriptorpb.EnumDescriptorProto
	prefix string

	commentSet
}

func (e *enumBuilder) addValue(number int32, schema *schema_j5pb.Enum_Option) {
	name := schema.Name
	if !strings.HasPrefix(name, e.prefix) {
		name = e.prefix + name
	}
	value := &descriptorpb.EnumValueDescriptorProto{
		Name:   gl.Ptr(name),
		Number: gl.Ptr(number),
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
