package protobuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	proto_parser "github.com/bufbuild/protocompile/parser"
	"github.com/pentops/j5build/internal/j5s/j5convert"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func hasAPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

var ErrNotFound = errors.New("File not found")

func protoToDescriptor(_ context.Context, filename string, data []byte, errs *ErrCollector) (proto_parser.Result, *j5convert.FileSummary, error) {

	fileNode, err := proto_parser.Parse(filename, bytes.NewReader(data), errs.Handler())
	if err != nil {
		return nil, nil, err
	}
	result, err := proto_parser.ResultFromAST(fileNode, true, errs.Handler())
	if err != nil {
		return nil, nil, err
	}

	summary, err := buildSummaryFromDescriptor(result.FileDescriptorProto(), errs)
	if err != nil {
		return nil, nil, err
	}
	return result, summary, nil
}

func buildSummaryFromReflect(res protoreflect.FileDescriptor, errs *ErrCollector) (*j5convert.FileSummary, error) {
	return buildSummaryFromDescriptor(protodesc.ToFileDescriptorProto(res), errs)
}

func buildSummaryFromDescriptor(res *descriptorpb.FileDescriptorProto, errs *ErrCollector) (*j5convert.FileSummary, error) {
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
	for idx, en := range res.EnumType {
		built, err := buildEnumRef(res, int32(idx), en, errs)
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
		SourceFilename:   filename,
		Exports:          exports,
		FileDependencies: res.Dependency,
		ProducesFiles:    []string{filename},
		Package:          res.GetPackage(),

		// No type dependencies for proto files, all deps come from the files.
		TypeDependencies: nil,
	}, nil
}

func buildEnumRef(file *descriptorpb.FileDescriptorProto, idx int32, enumDescriptor *descriptorpb.EnumDescriptorProto, errs *ErrCollector) (*j5convert.EnumRef, error) {
	for idx, value := range enumDescriptor.Value {
		if value.Number == nil {
			return nil, fmt.Errorf("enum value[%d] does not have a number", idx)
		}
		if value.Name == nil {
			return nil, fmt.Errorf("enum value[%d] does not have a name", idx)
		}
	}

	suffix := "UNSPECIFIED"
	var trimPrefix string

	if len(enumDescriptor.Value) < 1 {
		return nil, fmt.Errorf("enum has no values")
	} else {
		if *enumDescriptor.Value[0].Number != 0 {
			return nil, fmt.Errorf("enum does not have a value 0")
		}
		unspecifiedVal := *enumDescriptor.Value[0].Name

		if strings.HasSuffix(unspecifiedVal, suffix) {
			trimPrefix = strings.TrimSuffix(unspecifiedVal, suffix)
		} else {
			errs.WarnProtoDesc(file, []int32{5, idx}, fmt.Errorf("enum value 0 should have suffix %s", suffix))
			// proceed without prefix.
		}
	}

	ref := &j5convert.EnumRef{
		Prefix: trimPrefix,
		ValMap: map[string]int32{},
	}

	for _, value := range enumDescriptor.Value {
		ref.ValMap[value.GetName()] = value.GetNumber()
	}
	return ref, nil
}
