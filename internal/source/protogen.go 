package source

import (
	"fmt"
	"strings"

	"github.com/pentops/j5/gen/j5/source/v1/source_j5pb"
	"github.com/pentops/j5build/internal/structure"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func CodeGeneratorRequestFromImage(img *source_j5pb.SourceImage) (*pluginpb.CodeGeneratorRequest, error) {

	out := &pluginpb.CodeGeneratorRequest{
		CompilerVersion: nil,
		FileToGenerate:  img.SourceFilenames,
	}

	includeFiles := map[string]bool{}
	for _, file := range img.File {
		includeFiles[*file.Name] = true
	}

	if img.Options.Go != nil {
		for _, file := range img.File {
			if file.Options != nil && file.Options.GoPackage != nil {
				continue
			}
			if file.Options == nil {
				file.Options = &descriptorpb.FileOptions{}
			}

			pkg := *file.Package
			for _, prefix := range img.Options.Go.TrimPrefixes {
				if strings.HasPrefix(pkg, prefix) {
					pkg = pkg[len(prefix):]
					pkg = strings.TrimPrefix(pkg, ".")
					break
				}
			}
			basePkg, subPkg, err := structure.SplitPackageParts(pkg)
			if err != nil {
				return nil, fmt.Errorf("splitting package %q: %w", pkg, err)
			}

			suffix := "_pb"
			if subPkg != nil {
				sub := *subPkg
				suffix = fmt.Sprintf("_%spb", sub[0:1])
			}
			if img.Options.Go.Suffix != nil {
				suffix = *img.Options.Go.Suffix
			}
			pkgParts := strings.Split(basePkg, ".")
			baseName := strings.Join(pkgParts, "/")
			namePart := pkgParts[len(pkgParts)-2]
			lastPart := fmt.Sprintf("%s%s", namePart, suffix)
			fullName := strings.Join([]string{img.Options.Go.Prefix, baseName, lastPart}, "/")
			file.Options.GoPackage = &fullName

		}
	}
	// Prepare the files for the generator.
	// From the docs on out.ProtoFile:
	// FileDescriptorProtos for all files in files_to_generate and everything
	// they import.  The files will appear in topological order, so each file
	// appears before any file that imports it.

	// TODO: For now we are only including files that are in the FileToGenerate list, we should include the dependencies as well

	workingOn := make(map[string]bool)
	hasFile := make(map[string]bool)

	var addFile func(file *descriptorpb.FileDescriptorProto) error

	requireFile := func(name string) error {
		for _, f := range img.File {
			if *f.Name == name {
				return addFile(f)
			}
		}
		return fmt.Errorf("could not find file %q", name)
	}

	addFile = func(file *descriptorpb.FileDescriptorProto) error {
		if hasFile[*file.Name] {
			return nil
		}

		if workingOn[*file.Name] {
			return fmt.Errorf("circular dependency detected: %s", *file.Name)
		}
		workingOn[*file.Name] = true

		for _, dep := range file.Dependency {
			if err := requireFile(dep); err != nil {
				return fmt.Errorf("resolving dep %s for %s: %w", dep, *file.Name, err)
			}
		}

		out.ProtoFile = append(out.ProtoFile, file)
		if includeFiles[*file.Name] {
			out.SourceFileDescriptors = append(out.SourceFileDescriptors, file)
		}

		delete(workingOn, *file.Name)
		hasFile[*file.Name] = true

		return nil
	}

	for _, file := range img.File {
		if err := addFile(file); err != nil {
			return nil, err
		}
	}

	return out, nil
}
