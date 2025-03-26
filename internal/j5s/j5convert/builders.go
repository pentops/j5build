package j5convert

import (
	"sort"
	"strings"

	"github.com/pentops/golib/gl"
	"google.golang.org/protobuf/types/descriptorpb"
)

type fileContext struct {
	fdp *descriptorpb.FileDescriptorProto
	commentSet
}

func newFileContext(name string) *fileContext {
	pkgName := PackageFromFilename(name)
	return &fileContext{
		fdp: &descriptorpb.FileDescriptorProto{
			Syntax:  gl.Ptr("proto3"),
			Package: gl.Ptr(pkgName),
			Name:    gl.Ptr(name),
			Options: &descriptorpb.FileOptions{},
			SourceCodeInfo: &descriptorpb.SourceCodeInfo{
				Location: []*descriptorpb.SourceCodeInfo_Location{},
			},
		},
	}
}

func (fb *fileContext) File() *descriptorpb.FileDescriptorProto {
	last := int32(1)
	for _, comment := range fb.commentSet {
		last += 2
		loc := &descriptorpb.SourceCodeInfo_Location{
			Span: []int32{last, 1, 1},
			Path: comment.path,
		}

		if comment.description != nil {
			loc.LeadingComments = comment.description
		}

		fb.fdp.SourceCodeInfo.Location = append(fb.fdp.SourceCodeInfo.Location, loc)
	}

	return fb.fdp
}

func (fb *fileContext) ensureImport(importPath string) {

	if importPath == "" {
		panic("empty alias")
	}
	if !strings.Contains(importPath, "/") {
		panic("invalid import path " + importPath)
	}

	if importPath == *fb.fdp.Name {
		return
	}
	for _, imp := range fb.fdp.Dependency {
		if imp == importPath {
			return
		}
	}
	fb.fdp.Dependency = append(fb.fdp.Dependency, importPath)
	sort.Strings(fb.fdp.Dependency)
}

func (fb *fileContext) addMessage(message *MessageBuilder) {
	idx := int32(len(fb.fdp.MessageType))
	path := []int32{4, idx}
	fb.commentSet.mergeAt(path, message.commentSet)
	fb.fdp.MessageType = append(fb.fdp.MessageType, message.descriptor)
}

func (fb *fileContext) addEnum(enum *enumBuilder) {
	idx := int32(len(fb.fdp.EnumType))
	path := []int32{5, idx}
	fb.commentSet.mergeAt(path, enum.commentSet)
	fb.fdp.EnumType = append(fb.fdp.EnumType, enum.desc)
}

func (fb *fileContext) addService(service *serviceBuilder) {
	idx := int32(len(fb.fdp.Service))
	path := []int32{6, idx}
	fb.commentSet.mergeAt(path, service.commentSet)
	fb.fdp.Service = append(fb.fdp.Service, service.desc)
}

type MessageBuilder struct {
	descriptor *descriptorpb.DescriptorProto
	commentSet
}

func blankMessage(name string) *MessageBuilder {
	message := &MessageBuilder{
		descriptor: &descriptorpb.DescriptorProto{
			Name:    gl.Ptr(name),
			Options: &descriptorpb.MessageOptions{},
		},
	}
	return message
}

func (msg *MessageBuilder) addMessage(message *MessageBuilder) {
	msg.commentSet.mergeAt([]int32{3, int32(len(msg.descriptor.NestedType))}, message.commentSet)
	msg.descriptor.NestedType = append(msg.descriptor.NestedType, message.descriptor)
}

func (msg *MessageBuilder) addEnum(enum *enumBuilder) {
	msg.commentSet.mergeAt([]int32{4, int32(len(msg.descriptor.EnumType))}, enum.commentSet)
	msg.descriptor.EnumType = append(msg.descriptor.EnumType, enum.desc)
}
