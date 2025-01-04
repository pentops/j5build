package j5convert

import (
	"errors"
	"fmt"

	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/bcl/sourcewalk"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ConvertJ5File(deps TypeResolver, source *sourcedef_j5pb.SourceFile) ([]*descriptorpb.FileDescriptorProto, error) {

	importMap, err := j5Imports(source)
	if err != nil {
		return nil, err
	}

	file := newFileContext(source.Path + ".proto")
	root := newRootContext(deps, importMap, file)

	walker := &conversionVisitor{
		root:          root,
		file:          file,
		parentContext: file,
	}

	fileNode := sourcewalk.NewRoot(source)

	if err := walker.visitFileNode(fileNode); err != nil {
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
