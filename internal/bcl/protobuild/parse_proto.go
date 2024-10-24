package protobuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	proto_parser "github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func ptr[T any](v T) *T {
	return &v
}

func hasAPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

var ErrNotFound = errors.New("File not found")

func contextReporter(ctx context.Context) reporter.Reporter {

	errs := func(err reporter.ErrorWithPos) error {
		pos := err.GetPosition()
		errWithoutPos := err.Unwrap()
		log.WithFields(ctx, map[string]interface{}{
			"line":   pos.Line,
			"column": pos.Col,
			"file":   pos.Filename,
			"error":  errWithoutPos.Error(),
		}).Error("Compiler Error (BCL)")
		return errpos.AddPosition(err.Unwrap(), errpos.Position{
			Filename: ptr(pos.Filename),
			Start: errpos.Point{
				Line:   pos.Line - 1,
				Column: pos.Col - 1,
			},
		})
	}

	warnings := func(err reporter.ErrorWithPos) {
		pos := err.GetPosition()
		errWithoutPos := err.Unwrap()
		log.WithFields(ctx, map[string]interface{}{
			"line":   pos.Line,
			"column": pos.Col,
			"file":   pos.Filename,
			"error":  errWithoutPos.Error(),
		}).Warn("Compiler Warning (BCL)")
	}

	return reporter.NewReporter(errs, warnings)
}

func protoToDescriptor(ctx context.Context, filename string, data []byte) (proto_parser.Result, *j5convert.FileSummary, error) {
	ctx = log.WithField(ctx, "parseStep", "protoToDescriptor")
	reportHandler := reporter.NewHandler(contextReporter(ctx))
	fileNode, err := proto_parser.Parse(filename, bytes.NewReader(data), reportHandler)
	if err != nil {
		return nil, nil, err
	}
	result, err := proto_parser.ResultFromAST(fileNode, true, reportHandler)
	if err != nil {
		return nil, nil, err
	}

	summary, err := buildSummaryFromDescriptor(result.FileDescriptorProto())
	if err != nil {
		return nil, nil, err
	}
	return result, summary, nil
}

func buildSummaryFromReflect(res protoreflect.FileDescriptor) (*j5convert.FileSummary, error) {
	return buildSummaryFromDescriptor(protodesc.ToFileDescriptorProto(res))
}

func buildSummaryFromDescriptor(res *descriptorpb.FileDescriptorProto) (*j5convert.FileSummary, error) {
	filename := res.GetName()
	exports := map[string]*j5convert.TypeRef{}

	for _, msg := range res.MessageType {
		exports[msg.GetName()] = &j5convert.TypeRef{
			Name:       msg.GetName(),
			File:       filename,
			Package:    res.GetPackage(),
			MessageRef: &j5convert.MessageRef{},
		}
	}
	for _, en := range res.EnumType {
		built, err := buildEnumRef(en)
		if err != nil {
			return nil, err
		}
		exports[en.GetName()] = &j5convert.TypeRef{
			Name:    en.GetName(),
			Package: res.GetPackage(),
			File:    filename,
			EnumRef: built,
		}
	}

	return &j5convert.FileSummary{
		Exports:          exports,
		FileDependencies: res.Dependency,
		ProducesFiles:    []string{filename},
		Package:          res.GetPackage(),

		// No type dependencies for proto files, all deps come from the files.
		TypeDependencies: nil,
	}, nil
}

func buildEnumRef(enumDescriptor *descriptorpb.EnumDescriptorProto) (*j5convert.EnumRef, error) {
	for idx, value := range enumDescriptor.Value {
		if value.Number == nil {
			return nil, fmt.Errorf("enum value[%d] does not have a number", idx)
		}
		if value.Name == nil {
			return nil, fmt.Errorf("enum value[%d] does not have a name", idx)
		}
	}

	if len(enumDescriptor.Value) < 1 {
		return nil, fmt.Errorf("enum has no values")
	}
	if *enumDescriptor.Value[0].Number != 0 {
		return nil, fmt.Errorf("enum does not have a value 0")
	}

	suffix := "UNSPECIFIED"
	unspecifiedVal := *enumDescriptor.Value[0].Name
	if !strings.HasSuffix(unspecifiedVal, suffix) {
		return nil, fmt.Errorf("enum value 0 should have suffix %s", suffix)
	}
	trimPrefix := strings.TrimSuffix(unspecifiedVal, suffix)
	ref := &j5convert.EnumRef{
		Prefix: trimPrefix,
		ValMap: map[string]int32{},
	}

	for _, value := range enumDescriptor.Value {
		ref.ValMap[value.GetName()] = value.GetNumber()
	}
	return ref, nil
}
