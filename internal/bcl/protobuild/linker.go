package protobuild

import (
	"context"
	"fmt"

	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/options"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type searchLinker struct {
	symbols  *linker.Symbols
	Reporter reporter.Reporter
	resolver fileSource
}

type SearchResult struct {
	Filename string
	Summary  *j5convert.FileSummary

	// Results are checked in lexical order
	Linked      linker.File
	Refl        protoreflect.FileDescriptor
	Desc        *descriptorpb.FileDescriptorProto
	ParseResult *parser.Result
}

type fileSource interface {
	findFileByPath(ctx context.Context, filename string) (*SearchResult, error)
}

func (cc *searchLinker) resolve(ctx context.Context, filename string) (linker.File, error) {
	result, err := cc.resolver.findFileByPath(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("findFileByPath: %w", err)
	}

	if result.Linked != nil {
		return result.Linked, nil
	}

	linked, err := cc.link(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}
	result.Linked = linked
	return linked, nil
}

func (cc *searchLinker) link(ctx context.Context, result *SearchResult) (linker.File, error) {

	if result.Refl != nil {
		return linker.NewFileRecursive(result.Refl)
	}

	if result.Desc != nil {
		return cc.descriptorToFile(ctx, result.Desc)
	}

	if result.ParseResult != nil {
		return cc.resultToFile(ctx, *result.ParseResult)
	}

	return nil, fmt.Errorf("Had no result")
}

func (cc *searchLinker) resultToFile(ctx context.Context, result parser.Result) (linker.File, error) {

	desc := result.FileDescriptorProto()
	deps, err := cc.loadDependencies(ctx, desc)
	if err != nil {
		return nil, err
	}

	handler := reporter.NewHandler(cc.Reporter)
	linked, err := linker.Link(result, deps, cc.symbols, handler)
	if err != nil {
		return nil, err
	}

	_, err = options.InterpretOptions(linked, handler)
	if err != nil {
		return nil, err
	}

	linked.CheckForUnusedImports(handler)
	return linked, nil
}

func (cc *searchLinker) loadDependencies(ctx context.Context, desc *descriptorpb.FileDescriptorProto) (linker.Files, error) {
	deps := make(linker.Files, 0, len(desc.Dependency))
	for _, dep := range desc.Dependency {
		depFile, err := cc.resolve(ctx, dep)
		if err != nil {
			return nil, err
		}
		deps = append(deps, depFile)
	}
	return deps, nil
}

func (cc *searchLinker) descriptorToFile(ctx context.Context, desc *descriptorpb.FileDescriptorProto) (linker.File, error) {
	deps, err := cc.loadDependencies(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("loadDependencies: %w", err)
	}

	handler := reporter.NewHandler(cc.Reporter)
	result := parser.ResultWithoutAST(desc)
	linked, err := linker.Link(result, deps, cc.symbols, handler)
	if err != nil {
		return nil, fmt.Errorf("link: %w", err)
	}

	_, err = options.InterpretOptions(linked, handler)
	if err != nil {
		return nil, err
	}

	//linked.CheckForUnusedImports(handler)
	return linked, nil
}

// hacks the underlying linker to mark the imports which are used in extensions
// as 'used' to prevent a compiler warning.
func markExtensionImportsUsed(file linker.File) {
	resolver := linker.ResolverFromFile(file)
	messages := file.Messages()
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)
		markMessageExtensionImportsUsed(resolver, message)
	}
}

func markOptionImportsUsed(resolver linker.Resolver, opts proto.Message) {
	proto.RangeExtensions(opts, func(ext protoreflect.ExtensionType, value interface{}) bool {
		td := ext.TypeDescriptor()
		name := td.FullName()
		resolver.FindExtensionByName(name)
		return true
	})
}

func markMessageExtensionImportsUsed(resolver linker.Resolver, message protoreflect.MessageDescriptor) {
	markOptionImportsUsed(resolver, message.Options())
	fields := message.Fields()
	for j := 0; j < fields.Len(); j++ {
		field := fields.Get(j)
		markOptionImportsUsed(resolver, field.Options())
	}

	oneofs := message.Oneofs()
	for j := 0; j < oneofs.Len(); j++ {
		oneof := oneofs.Get(j)
		markOptionImportsUsed(resolver, oneof.Options())
	}

	nested := message.Messages()
	for j := 0; j < nested.Len(); j++ {
		nestedMessage := nested.Get(j)
		markMessageExtensionImportsUsed(resolver, nestedMessage)
	}
}
