package protobuild

import (
	"context"
	"fmt"

	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/options"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/pentops/j5build/internal/bcl/j5convert"
	"github.com/pentops/log.go/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type searchLinker struct {
	symbols  *linker.Symbols
	Reporter reporter.Reporter
	resolver fileSource
}

func newLinker(src fileSource, errs reporter.Reporter) *searchLinker {
	return &searchLinker{
		symbols:  &linker.Symbols{},
		Reporter: errs,
		resolver: src,
	}
}

type SearchResult struct {
	Summary *j5convert.FileSummary

	// Results are checked in lexical order
	Linked      linker.File
	Refl        protoreflect.FileDescriptor
	Desc        *descriptorpb.FileDescriptorProto
	ParseResult *parser.Result
}

type fileSource interface {
	findFileByPath(ctx context.Context, filename string) (*SearchResult, error)
}

func (ll *searchLinker) resolveAll(ctx context.Context, filenames []string) (linker.Files, error) {
	files := make(linker.Files, 0, len(filenames))
	for _, filename := range filenames {
		file, err := ll.resolve(ctx, filename)
		if err != nil {
			return nil, fmt.Errorf("resolveAll, resolve: %w", err)
		}
		files = append(files, file)
	}
	return files, nil
}

func (ll *searchLinker) resolve(ctx context.Context, filename string) (linker.File, error) {
	ctx = log.WithField(ctx, "askFilename", filename)
	result, err := ll.resolver.findFileByPath(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("findFileByPath: %w", err)
	}
	return ll.linkResult(ctx, result)
}

func (ll *searchLinker) linkResult(ctx context.Context, result *SearchResult) (linker.File, error) {
	if result.Linked != nil {
		log.WithField(ctx, "sourceFilename", result.Summary.SourceFilename).Debug("pre-linked")
		return result.Linked, nil
	}
	log.WithField(ctx, "sourceFilename", result.Summary.SourceFilename).Debug("link-new")

	linked, err := ll._linkNewResult(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("resolve, link: %w", err)
	}
	result.Linked = linked
	return linked, nil

}

func (ll *searchLinker) _linkNewResult(ctx context.Context, result *SearchResult) (linker.File, error) {

	if result.Refl != nil {
		return linker.NewFileRecursive(result.Refl)
	}

	if result.Desc != nil {
		return ll._descriptorToFile(ctx, result.Desc)
	}

	if result.ParseResult != nil {
		return ll.resultToFile(ctx, *result.ParseResult)
	}

	return nil, fmt.Errorf("Had no result")
}

func (ll *searchLinker) resultToFile(ctx context.Context, result parser.Result) (linker.File, error) {

	desc := result.FileDescriptorProto()
	deps, err := ll.loadDependencies(ctx, desc)
	if err != nil {
		return nil, err
	}

	handler := reporter.NewHandler(ll.Reporter)
	linked, err := linker.Link(result, deps, ll.symbols, handler)
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

func (ll *searchLinker) loadDependencies(ctx context.Context, desc *descriptorpb.FileDescriptorProto) (linker.Files, error) {
	deps := make(linker.Files, 0, len(desc.Dependency))
	for _, dep := range desc.Dependency {
		ll, err := ll.resolve(ctx, dep)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", dep, err)
		}
		deps = append(deps, ll)
	}
	return deps, nil
}

func (ll *searchLinker) _descriptorToFile(ctx context.Context, desc *descriptorpb.FileDescriptorProto) (linker.File, error) {
	deps, err := ll.loadDependencies(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("loadDependencies: %w", err)
	}

	handler := reporter.NewHandler(ll.Reporter)
	result := parser.ResultWithoutAST(desc)
	log.WithField(ctx, "descName", desc.GetName()).Debug("descriptorToFile")
	linked, err := linker.Link(result, deps, ll.symbols, handler)
	if err != nil {
		return nil, fmt.Errorf("descriptorToFile, link: %w", err)
	}

	_, err = options.InterpretOptions(linked, handler)
	if err != nil {
		return nil, err
	}

	err = markExtensionImportsUsed(linked)
	if err != nil {
		return nil, err
	}

	linked.PopulateSourceCodeInfo()

	linked.CheckForUnusedImports(handler)
	return linked, nil
}

// hacks the underlying linker to mark the imports which are used in extensions
// as 'used' to prevent a compiler warning.
func markExtensionImportsUsed(file linker.File) error {
	resolver := linker.ResolverFromFile(file)
	messages := file.Messages()
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)
		err := markMessageExtensionImportsUsed(resolver, message)
		if err != nil {
			return err
		}
	}

	services := file.Services()
	for i := 0; i < services.Len(); i++ {
		service := services.Get(i)
		err := markOptionImportsUsed(resolver, service.Options())
		if err != nil {
			return err
		}

		methods := service.Methods()
		for j := 0; j < methods.Len(); j++ {
			method := methods.Get(j)
			err = markOptionImportsUsed(resolver, method.Options())
			if err != nil {
				return err
			}
		}
	}

	enums := file.Enums()
	for i := 0; i < enums.Len(); i++ {
		enum := enums.Get(i)
		err := markOptionImportsUsed(resolver, enum.Options())
		if err != nil {
			return err
		}

		values := enum.Values()
		for j := 0; j < values.Len(); j++ {
			value := values.Get(j)
			err = markOptionImportsUsed(resolver, value.Options())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func markMessageExtensionImportsUsed(resolver linker.Resolver, message protoreflect.MessageDescriptor) error {
	err := markOptionImportsUsed(resolver, message.Options())
	if err != nil {
		return err
	}

	fields := message.Fields()
	for j := 0; j < fields.Len(); j++ {
		field := fields.Get(j)
		err = markOptionImportsUsed(resolver, field.Options())
		if err != nil {
			return err
		}

	}

	oneofs := message.Oneofs()
	for j := 0; j < oneofs.Len(); j++ {
		oneof := oneofs.Get(j)
		err = markOptionImportsUsed(resolver, oneof.Options())
		if err != nil {
			return err
		}
	}

	nested := message.Messages()
	for j := 0; j < nested.Len(); j++ {
		nestedMessage := nested.Get(j)
		err = markMessageExtensionImportsUsed(resolver, nestedMessage)
		if err != nil {
			return err
		}

	}
	return nil
}

func markOptionImportsUsed(resolver linker.Resolver, opts proto.Message) error {
	var outerErr error

	proto.RangeExtensions(opts, func(ext protoreflect.ExtensionType, value interface{}) bool {
		td := ext.TypeDescriptor()
		name := td.FullName()
		_, err := resolver.FindExtensionByName(name)
		if err != nil {
			outerErr = err
			return false
		}
		return true
	})

	return outerErr
}
