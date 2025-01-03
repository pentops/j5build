package protoprint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/pentops/j5build/internal/bcl/protoprint/optionreflect"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Options struct {
	PackagePrefixes []string
	OnlyFilenames   []string
}

type FileWriter interface {
	PutFile(ctx context.Context, path string, data []byte) error
}

type mapResolver struct {
	descriptors map[string]*descriptorpb.FileDescriptorProto
	built       map[string]protoreflect.FileDescriptor
}

func (r *mapResolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if file, ok := r.built[path]; ok {
		return file, nil
	}
	if file, ok := r.descriptors[path]; ok {
		fd, err := protodesc.NewFile(file, r)
		if err != nil {
			return nil, err
		}
		r.built[path] = fd
		return fd, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(path)
}

func (r *mapResolver) FindDescriptorByName(message protoreflect.FullName) (protoreflect.Descriptor, error) {
	return protoregistry.GlobalFiles.FindDescriptorByName(message)
}

func PrintFile(ctx context.Context, file protoreflect.FileDescriptor) (string, error) {
	fileData, err := printFile(file, nil)
	if err != nil {
		return "", fmt.Errorf("in file %s: %w", file.Path(), err)
	}
	return string(fileData), nil
}

func PrintReflect(ctx context.Context, out FileWriter, descriptors []protoreflect.FileDescriptor, opts Options) error {
	for _, file := range descriptors {
		fileData, err := printFile(file, nil)
		if err != nil {
			return fmt.Errorf("in file %s: %w", file.Path(), err)
		}

		if err := out.PutFile(ctx, file.Path(), fileData); err != nil {
			return err
		}
	}

	return nil
}

func PrintProtoFiles(ctx context.Context, out FileWriter, src *descriptorpb.FileDescriptorSet, opts Options) error {

	fileMap := make(map[string]struct{})
	if len(opts.OnlyFilenames) > 0 {
		for _, filename := range opts.OnlyFilenames {
			fileMap[filename] = struct{}{}
		}
	} else {
		for _, file := range src.File {
			fileMap[*file.Name] = struct{}{}
		}
	}

	sourceMap := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, file := range src.File {
		sourceMap[*file.Name] = file
	}

	resolver := &mapResolver{descriptors: sourceMap}
	descriptors := make([]protoreflect.FileDescriptor, 0)
	for _, file := range src.File {
		descriptor, err := protodesc.NewFile(file, resolver)
		if err != nil {
			return err
		}
		descriptors = append(descriptors, descriptor)
	}

	foundExtensions := make([]protoreflect.ExtensionDescriptor, 0)

	for _, file := range descriptors {
		for i := 0; i < file.Extensions().Len(); i++ {
			foundExtensions = append(foundExtensions, file.Extensions().Get(i))
		}
	}

	extBuilder := optionreflect.NewBuilder(foundExtensions)

	for _, file := range descriptors {
		if _, ok := fileMap[string(file.Path())]; !ok {
			continue
		}

		fileData, err := printFile(file, extBuilder)
		if err != nil {
			return fmt.Errorf("in file %s: %w", file.Path(), err)

		}

		if err := out.PutFile(ctx, file.Path(), fileData); err != nil {
			return err
		}

	}

	return nil
}

type fileBuffer struct {
	out        *bytes.Buffer
	addGap     bool
	extensions *optionreflect.Builder
}

func (fb *fileBuffer) p(indent int, args ...interface{}) {
	if fb.addGap {
		fb.addGap = false
		fb.out.WriteString("\n")
	}
	fmt.Fprint(fb.out, strings.Repeat(" ", indent*2))
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			fmt.Fprint(fb.out, arg)
		case []string:
			for _, subArg := range arg {
				fmt.Fprint(fb.out, subArg)
			}
		default:
			fmt.Fprintf(fb.out, "%v", arg)
		}
	}
	fb.out.WriteString("\n")
}

type fileBuilder struct {
	out *fileBuffer
	ind int
}

func printFile(ff protoreflect.FileDescriptor, exts *optionreflect.Builder) ([]byte, error) {
	p := &fileBuilder{
		out: &fileBuffer{
			extensions: exts,
			out:        &bytes.Buffer{},
		},
	}
	return p.printFile(ff)
}

func (fb *fileBuilder) p(args ...interface{}) {
	fb.out.p(fb.ind, args...)
}

func commentLines(comment string) []string {
	if comment == "" {
		return nil
	}
	lines := strings.Split(comment, "\n")
	lines = lines[:len(lines)-1] // comment strings end with a newline
	for i, line := range lines {
		lines[i] = fmt.Sprintf("//%s", line)
	}
	return lines
}

func inlineComment(loc protoreflect.SourceLocation) []string {
	lines := strings.Split(loc.TrailingComments, "\n")
	lines = lines[:len(lines)-1] // comment strings end with a newline
	if len(lines) > 1 {
		return nil // to be picked up by trailing comments.
	}
	for i, line := range lines {
		lines[i] = fmt.Sprintf(" //%s", line)
	}
	return lines
}

func (fb *fileBuilder) trailingComments(loc protoreflect.SourceLocation) {
	lines := strings.Split(loc.TrailingComments, "\n")
	lines = lines[:len(lines)-1] // comment strings end with a newline
	if len(lines) <= 1 {
		return // picked up by inlineComment
	}

	for _, line := range lines {
		fb.p("//", line)
	}
	fb.addGap()
}

func (fb *fileBuilder) leadingComments(loc protoreflect.SourceLocation) {
	for _, comment := range loc.LeadingDetachedComments {
		parts := commentLines(comment)
		for _, part := range parts {
			fb.p(part)
		}
		fb.addGap()
	}

	if loc.LeadingComments != "" {
		fb.addGap()
		parts := commentLines(loc.LeadingComments)
		for _, part := range parts {
			fb.p(part)
		}
	}
}

func (fb *fileBuilder) addGap() {
	fb.out.addGap = true
}

func (fb *fileBuilder) endElem(end ...interface{}) {
	// gaps should only occur between elements, not after the last one
	fb.out.addGap = false
	fb.p(end...)
}

func (fb fileBuilder) indent() fileBuilder {
	return fileBuilder{out: fb.out, ind: fb.ind + 1}
}

func (fb *fileBuilder) printFile(ff protoreflect.FileDescriptor) ([]byte, error) {

	if ff.Syntax() != protoreflect.Proto3 {
		return nil, errors.New("only proto3 syntax is supported")
	}

	opening := ff.SourceLocations().ByPath(nil)
	fb.leadingComments(opening)
	fb.p("syntax = \"proto3\";")
	fb.p()
	fb.p("package ", ff.Package(), ";")
	fb.addGap()

	imports := ff.Imports()

	importStrings := make([]string, 0, imports.Len())
	for idx := 0; idx < imports.Len(); idx++ {
		dep := imports.Get(idx)
		importStrings = append(importStrings, dep.Path())
	}

	if len(importStrings) > 0 {
		sort.Strings(importStrings)
		for _, dep := range importStrings {
			fb.p("import \"", dep, "\";")
		}
		fb.addGap()
	}
	// This could be manual iteration, but seemed more future-proof and
	// quicker to write.
	refl := ff.Options().ProtoReflect()
	fields := refl.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		field := fields.Get(i)
		if !refl.Has(field) {
			continue
		}
		switch field.Kind() {
		case protoreflect.BoolKind:
			fb.p("option ", field.Name(), " = ", refl.Get(field).Interface(), ";")
		case protoreflect.StringKind:
			fb.p("option ", field.Name(), " = \"", refl.Get(field).Interface(), "\";")
		}
	}
	fb.addGap()

	extBlocks := make([]extBlock, 0)

	exts := ff.Extensions()
	for idx := 0; idx < exts.Len(); idx++ {
		ext := exts.Get(idx)
		fullName := ext.ContainingMessage().FullName()
		found := false
		for i := range extBlocks {
			if extBlocks[i].extends == fullName {
				extBlocks[i].fields = append(extBlocks[i].fields, ext)
				found = true
				break
			}
		}
		if !found {
			extBlocks = append(extBlocks, extBlock{
				extends: fullName,
				fields:  []protoreflect.FieldDescriptor{ext},
			})
		}
	}

	for _, block := range extBlocks {
		if err := fb.printExtension(block); err != nil {
			return nil, err
		}
	}

	var elements = make(sourceElements, 0)

	messages := ff.Messages()
	for idx := 0; idx < messages.Len(); idx++ {
		elements.add(messages.Get(idx))
	}

	services := ff.Services()
	for idx := 0; idx < services.Len(); idx++ {
		elements.add(services.Get(idx))
	}

	enums := ff.Enums()
	for idx := 0; idx < enums.Len(); idx++ {
		elements.add(enums.Get(idx))
	}

	if err := fb.printElements(elements); err != nil {
		return nil, err
	}

	return fb.out.out.Bytes(), nil
}

func fieldTypeName(field protoreflect.FieldDescriptor) (string, error) {
	fieldType := field.Kind()

	var refElement protoreflect.Descriptor

	switch fieldType {
	case protoreflect.EnumKind:
		refElement = field.Enum()
	case protoreflect.MessageKind:
		refElement = field.Message()
	default:
		return fieldType.String(), nil
	}

	if refElement == nil {
		return "", fmt.Errorf("field type is nil for %s", field.FullName())
	}
	fieldMsg := field.Parent()

	return contextRefName(fieldMsg, refElement)
}

func contextRefName(contextOfCall protoreflect.Descriptor, refElement protoreflect.Descriptor) (string, error) {

	if contextOfCall.ParentFile().Package() != refElement.ParentFile().Package() {
		// if the thing the field references is in a different package, then the
		// full reference is used
		return string(refElement.FullName()), nil
	}

	refPath := pathToPackage(refElement)
	contextPath := pathToPackage(contextOfCall)

	for i := 0; i < len(contextPath); i++ {
		if len(refPath) == 0 || refPath[0] != contextPath[i] {
			break
		}
		refPath = refPath[1:]
	}

	return strings.Join(refPath, "."), nil
}

func pathToPackage(refElement protoreflect.Descriptor) []string {

	refPath := make([]string, 0)
	parentFileName := refElement.ParentFile().FullName()
	parent := refElement
	for parent.FullName() != parentFileName {
		refPath = append(refPath, string(parent.Name()))
		parent = parent.Parent()
	}

	stringsOut := make([]string, len(refPath))
	for i, part := range refPath {
		stringsOut[len(refPath)-i-1] = part
	}

	return stringsOut
}
