package j5convert

import (
	"github.com/pentops/golib/gl"
	"google.golang.org/protobuf/types/descriptorpb"
)

type serviceBuilder struct {
	desc *descriptorpb.ServiceDescriptorProto
	commentSet
}

func blankService(name string) *serviceBuilder {
	return &serviceBuilder{
		desc: &descriptorpb.ServiceDescriptorProto{
			Name: gl.Ptr(name),
		},
	}
}

type MethodBuilder struct {
	desc *descriptorpb.MethodDescriptorProto
	commentSet
}

func blankMethod(name string) *MethodBuilder {
	return &MethodBuilder{
		desc: &descriptorpb.MethodDescriptorProto{
			Name:    gl.Ptr(name),
			Options: &descriptorpb.MethodOptions{},
		},
	}
}
