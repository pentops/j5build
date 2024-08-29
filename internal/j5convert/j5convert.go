package j5convert

import (
	"path"
	"strings"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ConvertJ5File(pkg string, source *sourcedef_j5pb.SourceFile) (*descriptorpb.FileDescriptorProto, error) {
	fb := NewFileBuilder(pkg, source.Path)

	for _, dep := range source.Imports {
		filename := dep.Path
		ext := path.Ext(filename)
		if ext == ".j5s" {
			filename = strings.TrimSuffix(filename, ext) + ".gen.proto"
		}
		fb.ensureImport(filename)
	}

	for _, element := range source.Elements {
		if err := fb.AddRoot(element); err != nil {
			return nil, err
		}
	}

	return fb.File(), nil
}
