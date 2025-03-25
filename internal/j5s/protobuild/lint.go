package protobuild

import (
	"context"
	"fmt"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/j5s/j5convert"
	"github.com/pentops/log.go/log"
)

func LintFile(ctx context.Context, ps PackageSrc, filename string, fileData string) (*errpos.ErrorsWithSource, error) {
	pkgName, isLocal, err := ps.PackageForLocalFile(filename)
	if err != nil {
		return nil, fmt.Errorf("packageForFile %s: %w", filename, err)
	}
	if !isLocal {
		return nil, fmt.Errorf("file %s is not a local bundle file", filename)
	}

	// LoadLocalPackage parses both BCL and Proto files, but does not fully link.
	pkg, errs, err := ps.LoadLocalPackage(ctx, pkgName)
	if err != nil {
		if ep, ok := errpos.AsErrorsWithSource(err); ok {
			return ep, nil
		}
		return nil, fmt.Errorf("loadLocalPackage %s: %w", pkgName, err)
	}

	var sourceFile *SourceFile

	for _, search := range pkg.SourceFiles {
		if search.Filename == filename {
			sourceFile = search
			break
		}
	}
	if sourceFile == nil {
		return nil, fmt.Errorf("source file %s not found in package %s", filename, pkgName)
	}

	linker := newLinker(ps, errs)

	for _, srcFile := range pkg.SourceFiles {

		if srcFile.Filename != filename {
			continue
		}

		if srcFile.Result != nil {
			pkg.Files[srcFile.Filename] = srcFile.Result

			_, err = linker.linkResult(ctx, srcFile.Result)
			if err != nil {
				return nil, fmt.Errorf("linking proto file %s: %w", filename, err)
			}

		} else if srcFile.J5Source != nil {
			descs, err := j5convert.ConvertJ5File(pkg, srcFile.J5Source)
			if err != nil {
				return nil, fmt.Errorf("convertJ5File %s: %w", srcFile.Filename, err)
			}

			for _, desc := range descs {
				sr := &SearchResult{
					Summary: srcFile.Summary,
					Desc:    desc,
				}
				outputName := desc.GetName()

				pkg.Files[outputName] = sr

				ctx := log.WithField(ctx, "linking", outputName)
				log.Info(ctx, "linking for lint")
				_, err := linker.linkResult(ctx, sr)
				if err != nil {
					return nil, fmt.Errorf("linking j5 file %s: %w", filename, err)
				}
				if errs.HasAny() {
					return convertLintErrors(outputName, "", errs)
				}
			}
		} else {
			return nil, fmt.Errorf("source file %s has no result and is not j5s", srcFile.Filename)
		}
	}

	return convertLintErrors(filename, fileData, errs)
}

func LintAll(ctx context.Context, ps PackageSrc) (*errpos.ErrorsWithSource, error) {
	allPackages := ps.ListLocalPackages()
	errs := &ErrCollector{}
	linker := newLinker(ps, errs)

	for _, pkgName := range allPackages {
		// LoadLocalPackage parses both BCL and Proto files, but does not fully link.
		pkg, errs, err := ps.LoadLocalPackage(ctx, pkgName)
		if err != nil {
			if ep, ok := errpos.AsErrorsWithSource(err); ok {
				return ep, nil
			}
			return nil, fmt.Errorf("loadLocalPackage %s: %w", pkgName, err)
		}
		if errs.HasAny() {
			return convertLintErrors("", "", errs)
		}

		for _, file := range pkg.Files {
			linked, err := linker.linkResult(ctx, file)
			if err != nil {
				return nil, fmt.Errorf("linking file %s: %w", file.Summary.SourceFilename, err)
			}
			if errs.HasAny() {
				if file.Summary.SourceFilename == linked.Path() {
					data, err := ps.GetLocalFileContent(ctx, file.Summary.SourceFilename)
					if err != nil {
						return nil, fmt.Errorf("getRawFile %s: %w", file.Summary.SourceFilename, err)
					}
					return convertLintErrors(file.Summary.SourceFilename, data, errs)
				} else {
					return convertLintErrors(file.Summary.SourceFilename+"!Virtual", "", errs)
				}
			}
		}

	}
	return convertLintErrors("", "", errs)

}

func convertLintErrors(filename string, fileData string, errs *ErrCollector) (*errpos.ErrorsWithSource, error) {

	errors := errpos.Errors{}
	for _, err := range errs.Errors {
		errors = append(errors, err)
	}
	for _, err := range errs.Warnings {
		errors = append(errors, err)
	}

	if len(errors) == 0 {
		return nil, nil
	}

	ws := errpos.AddSourceFile(errors, filename, fileData)
	as, ok := errpos.AsErrorsWithSource(ws)
	if !ok {
		return nil, fmt.Errorf("error not valid for source: (%T) %w", ws, ws)
	}
	return as, nil
}
