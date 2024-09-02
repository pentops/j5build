package j5convert

import (
	"errors"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/types/descriptorpb"
)

func sourceLoc(path []int32, description string) *descriptorpb.SourceCodeInfo_Location {
	loc := &descriptorpb.SourceCodeInfo_Location{
		Path: path,
	}

	if description != "" {
		loc.LeadingComments = ptr(" " + description + "\n")
	}

	return loc
}

const (
	bufValidateImport = "buf/validate/validate.proto"
	j5ExtImport       = "j5/ext/v1/annotations.proto"
	j5DateImport      = "j5/types/date/v1/date.proto"
	j5DecimalImport   = "j5/types/decimal/v1/decimal.proto"
	pbTimestamp       = "google/protobuf/timestamp.proto"
	pbAnyImport       = "google/protobuf/any.proto"
	psmStateImport    = "j5/state/v1/metadata.proto"
)

type commentSet []*descriptorpb.SourceCodeInfo_Location

func (cs *commentSet) comment(path []int32, description string) {
	*cs = append(*cs, sourceLoc(path, description))
}

func ConvertJ5File(pkg Package, source *sourcedef_j5pb.SourceFile) (*descriptorpb.FileDescriptorProto, error) {

	fb := newFileBuilder(pkg, source.SourceLocations, source.Path+".proto")

	err := fb.AddImports(source.Imports...)
	if err != nil {
		return nil, err
	}

	for idx, element := range source.Elements {
		fb.AddRoot(idx, element)
	}

	if len(fb.errors) > 0 {
		return nil, errors.Join(fb.errors...)
	}

	return fb.File(), nil
}
