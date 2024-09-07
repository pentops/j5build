package j5convert

import "google.golang.org/protobuf/types/descriptorpb"

type ServiceBuilder struct {
	root     fileContext // service always belongs to a file
	desc     *descriptorpb.ServiceDescriptorProto
	basePath string
	commentSet
}

func blankService(root fileContext, name string) *ServiceBuilder {
	return &ServiceBuilder{
		root: root,
		desc: &descriptorpb.ServiceDescriptorProto{
			Name: ptr(name),
		},
	}
}

func (sb *ServiceBuilder) addMethod(method *MethodBuilder) {
	sb.commentSet.mergeAt([]int32{2, int32(len(sb.desc.Method))}, method.commentSet)
	sb.desc.Method = append(sb.desc.Method, method.desc)
}

type MethodBuilder struct {
	desc *descriptorpb.MethodDescriptorProto
	commentSet
}

func blankMethod(name string) *MethodBuilder {
	return &MethodBuilder{
		desc: &descriptorpb.MethodDescriptorProto{
			Name:    ptr(name),
			Options: &descriptorpb.MethodOptions{},
		},
	}
}
