package protobuild

import (
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/j5build/internal/bcl/errpos"
	"github.com/pentops/golib/gl"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ErrCollector struct {
	Errors   []*errpos.Err //reporter.ErrorWithPos
	Warnings []*errpos.Err //.ErrorWithPos
}

func convertError(err reporter.ErrorWithPos) *errpos.Err {
	start := err.Start()
	end := err.End()
	cause := err.Unwrap()

	return &errpos.Err{
		Pos: &errpos.Position{
			Filename: &start.Filename,
			Start: errpos.Point{
				Line:   start.Line - 1,
				Column: start.Col - 1,
			},
			End: errpos.Point{
				Line:   end.Line - 1,
				Column: end.Col - 1,
			},
		},
		Err: cause,
	}
}

func pathsEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func (ec *ErrCollector) WarnProtoDesc(file *descriptorpb.FileDescriptorProto, path []int32, err error) {
	var loc *descriptorpb.SourceCodeInfo_Location
	if file.SourceCodeInfo != nil {
		for _, l := range file.SourceCodeInfo.Location {
			if pathsEqual(l.Path, path) {
				loc = l
				break
			}
		}
	}

	pos := &errpos.Position{
		Filename: gl.Ptr(file.GetName()),
	}

	if loc != nil {
		if len(loc.Span) == 4 {
			pos.Start.Line = int(loc.Span[0])
			pos.Start.Column = int(loc.Span[1])
			pos.End.Line = int(loc.Span[2])
			pos.End.Column = int(loc.Span[3])
		} else if len(loc.Span) == 3 {
			pos.Start.Line = int(loc.Span[0])
			pos.Start.Column = int(loc.Span[1])
			pos.End.Line = int(loc.Span[0])
			pos.End.Column = int(loc.Span[2])
		}
	}

	ec.Warnings = append(ec.Warnings, &errpos.Err{
		Pos: pos,
		Err: err,
	})
}

func (ec *ErrCollector) WarnPos(pos *errpos.Position, err error) {
	ec.Warnings = append(ec.Warnings, &errpos.Err{
		Pos: pos,
		Err: err,
	})
}

func (ec *ErrCollector) WarnProto(desc protoreflect.Descriptor, err error) {
	file := desc.ParentFile()
	loc := file.SourceLocations().ByDescriptor(desc)
	// may be zero value
	ec.Warnings = append(ec.Warnings, &errpos.Err{
		Pos: &errpos.Position{
			Filename: gl.Ptr(file.Path()),
			Start: errpos.Point{
				Line:   int(loc.StartLine),
				Column: int(loc.StartColumn),
			},
			End: errpos.Point{
				Line:   int(loc.EndLine),
				Column: int(loc.EndColumn),
			},
		},
		Err: err,
	})

}

func (ec *ErrCollector) Handler() *reporter.Handler {
	return reporter.NewHandler(ec)
}

// Error implements reporter.Reporter
func (ec *ErrCollector) Error(err reporter.ErrorWithPos) error {
	ec.Errors = append(ec.Errors, convertError(err))
	return err
}

// Warning implements reporter.Reporter
func (ec *ErrCollector) Warning(err reporter.ErrorWithPos) {
	ec.Warnings = append(ec.Warnings, convertError(err))
}

func (ec *ErrCollector) HasAny() bool {
	return len(ec.Errors) > 0 || len(ec.Warnings) > 0
}
