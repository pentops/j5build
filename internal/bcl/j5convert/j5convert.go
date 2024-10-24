package j5convert

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ConvertJ5File(deps Package, source *sourcedef_j5pb.SourceFile) ([]*descriptorpb.FileDescriptorProto, error) {

	importMap, err := j5Imports(source)
	if err != nil {
		return nil, err
	}

	file := newFileBuilder(source.Path + ".proto")
	root := newRoot(deps, importMap, file, source.SourceLocations)

	walker := &walkContext{
		root:          root,
		file:          file,
		parentContext: file,
	}

	if err := convertFile(walker, source); err != nil {
		// Errors returned here from the sourcewalk code, not the visitors
		return nil, fmt.Errorf("schema error: %w", err)
	}

	if len(root.errors) > 0 {
		return nil, errors.Join(root.errors...)
	}

	descriptors := []*descriptorpb.FileDescriptorProto{}
	for _, extra := range root.files {
		descriptors = append(descriptors, extra.File())
	}

	return descriptors, nil
}

type commentSet []*descriptorpb.SourceCodeInfo_Location

func sourceLoc(path []int32, description string) *descriptorpb.SourceCodeInfo_Location {
	loc := &descriptorpb.SourceCodeInfo_Location{
		Path: path,
	}

	if description != "" {
		lines := strings.Split(description, "\n")
		joined := " " + strings.Join(lines, "\n ") + "\n"
		loc.LeadingComments = ptr(joined)
	}

	return loc
}

func (cs *commentSet) comment(path []int32, description string) {
	*cs = append(*cs, sourceLoc(path, description))
}

func (cs *commentSet) mergeAt(path []int32, other commentSet) {
	for _, comment := range other {
		comment.Path = append(path, comment.Path...)
		*cs = append(*cs, comment)
	}
}
