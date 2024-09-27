package reader

import (
	"context"
	"errors"
	"fmt"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Compiler struct {
	Resolver protocompile.Resolver
}

func NewCompiler(resolver protocompile.Resolver) *Compiler {
	return &Compiler{
		Resolver: protocompile.CompositeResolver{
			BuiltinResolver,
			resolver,
		},
	}
}

func (cc *Compiler) Compile(ctx context.Context, filenames []string) ([]*descriptorpb.FileDescriptorProto, error) {
	errs := func(err reporter.ErrorWithPos) error {
		log.WithError(ctx, err).Error("Compiler Error")
		pos := err.GetPosition()
		return errpos.AddPosition(err.Unwrap(), errpos.Position{
			Filename: &pos.Filename,
			Start: errpos.Point{
				Line:   pos.Line - 1,
				Column: pos.Col - 1,
			},
		})
	}

	warnings := func(err reporter.ErrorWithPos) {
		log.WithError(ctx, err).Warn("Compiler Warning")
	}

	pcCompiler := protocompile.Compiler{
		Resolver:       cc.Resolver,
		SourceInfoMode: protocompile.SourceInfoStandard,
		Reporter:       reporter.NewReporter(errs, warnings),
	}

	customDesc, err := pcCompiler.Compile(ctx, filenames...)
	if err != nil {
		panicErr := protocompile.PanicError{}
		if errors.As(err, &panicErr) {
			fmt.Printf("PANIC: %s\n", panicErr.Stack)
		}

		return nil, err
	}

	var files []*descriptorpb.FileDescriptorProto
	seen := map[string]struct{}{}
	for _, file := range customDesc {
		if err := addFile(file, &files, seen); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func addFile(fd protoreflect.FileDescriptor, results *[]*descriptorpb.FileDescriptorProto, seen map[string]struct{}) error {
	name := fd.Path()
	if _, ok := seen[name]; ok {
		return nil
	}
	seen[name] = struct{}{}
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		err := addFile(imports.Get(i).FileDescriptor, results, seen)
		if err != nil {
			return err
		}
	}
	fd1 := protodesc.ToFileDescriptorProto(fd)

	// TODO: Only run this when required.

	// This hack matches an accidental work-around in earlier code.
	// The act of marshalling and unmarshalling changes the underlying type
	// of the extensions.
	// In the output of protocompile, the options are dynamicpb, but we need
	// them to be the implemented Go type for proto.GetExtension to not
	// panic.
	// Another workaround is here:
	// https://github.com/jhump/protoreflect/blob/v1.17.0/desc/protoparse/parser.go#L724
	// stored images were always fine, the issue only comes up when we use
	// the descriptors without storing them,.
	fdB, err := proto.Marshal(fd1)
	if err != nil {
		return err
	}
	fd2 := &descriptorpb.FileDescriptorProto{}
	if err := proto.Unmarshal(fdB, fd2); err != nil {
		return err
	}
	*results = append(*results, fd2)
	return nil
}
