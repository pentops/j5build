package j5convert

import (
	"fmt"
	"strconv"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

type FileBuilder struct {
	Name string
	Package

	fdp    *descriptorpb.FileDescriptorProto
	source sourceLink
	errors []error

	importAliases map[string]string
}

func newFileBuilder(deps Package, source *sourcedef_j5pb.SourceLocation, name string) *FileBuilder {
	pkgName := PackageFromFilename(name)
	return &FileBuilder{
		Name:          name,
		Package:       deps,
		source:        sourceLink{root: source},
		importAliases: map[string]string{},
		fdp: &descriptorpb.FileDescriptorProto{
			Syntax:  ptr("proto3"),
			Package: ptr(pkgName),
			Name:    ptr(name),
			Options: &descriptorpb.FileOptions{},
			SourceCodeInfo: &descriptorpb.SourceCodeInfo{
				Location: []*descriptorpb.SourceCodeInfo_Location{},
			},
		},
	}
}

func (fb *FileBuilder) AddRoot(idx int, schema *sourcedef_j5pb.RootElement) {
	ww := &walkNode{
		root:          fb,
		path:          []string{"elements", strconv.Itoa(idx)},
		parentContext: fb,
	}
	ww.addRoot(schema)
}

func (fb *FileBuilder) File() *descriptorpb.FileDescriptorProto {
	last := int32(1)
	for _, loc := range fb.fdp.SourceCodeInfo.Location {
		last += 2
		loc.Span = []int32{last, 1, 1}
	}
	return fb.fdp
}

func (fb *FileBuilder) addMessage(message *MessageBuilder) {
	idx := int32(len(fb.fdp.MessageType))
	path := []int32{4, idx}

	for _, comment := range message.commentSet {
		fb.fdp.SourceCodeInfo.Location = append(fb.fdp.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path:             append(path, comment.Path...),
			LeadingComments:  comment.LeadingComments,
			TrailingComments: comment.TrailingComments,
		})
	}

	fmt.Printf("appending message type %s\n", message.descriptor.GetName())
	fb.fdp.MessageType = append(fb.fdp.MessageType, message.descriptor)
}

func (fb *FileBuilder) addEnum(enum *EnumBuilder) {
	idx := int32(len(fb.fdp.EnumType))
	path := []int32{5, idx}

	for _, comment := range enum.commentSet {
		fb.fdp.SourceCodeInfo.Location = append(fb.fdp.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
			Path:             append(path, comment.Path...),
			LeadingComments:  comment.LeadingComments,
			TrailingComments: comment.TrailingComments,
		})
	}

	fb.fdp.EnumType = append(fb.fdp.EnumType, enum.desc)
}
