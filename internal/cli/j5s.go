package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/protobuild"
	"github.com/pentops/j5build/internal/bcl/protoprint"
	"github.com/pentops/j5build/internal/source"
	"github.com/pentops/log.go/log"
	"github.com/pentops/runner/commander"
)

func j5sSet() *commander.CommandSet {
	genGroup := commander.NewCommandSet()
	genGroup.Add("fmt", commander.NewCommand(runJ5sFmt))
	genGroup.Add("lint", commander.NewCommand(runJ5sLint))
	genGroup.Add("genproto", commander.NewCommand(runJ5sGenProto))
	return genGroup
}

func runJ5sLint(ctx context.Context, cfg struct {
	Dir  string `flag:"dir" required:"false" description:"Source / working directory containing j5.yaml"`
	File string `flag:"file" required:"false" description:"Single file to format"`
}) error {

	resolver, err := source.NewEnvResolver()
	if err != nil {
		return err
	}

	if cfg.Dir == "" {
		cfg.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	fsRoot := os.DirFS(cfg.Dir)
	srcRoot, err := source.NewFSRepoRoot(ctx, fsRoot, resolver)
	if err != nil {
		return err
	}

	bundles := srcRoot.AllBundles()
	if cfg.File != "" {
		fullDir, err := filepath.Abs(cfg.Dir)
		if err != nil {
			return err
		}
		var bundle source.Bundle
		var relToBundle string
		for _, search := range bundles {
			bundleDir := filepath.Join(fullDir, search.DirInRepo())
			rel, err := filepath.Rel(bundleDir, cfg.File)
			if err != nil {
				continue
			}
			if strings.HasPrefix(rel, "..") {
				continue
			}
			bundle = search
			relToBundle = rel
		}

		if bundle == nil {
			return fmt.Errorf("File %s not found in any bundle", cfg.File)
		}

		deps, err := bundle.GetDependencies(ctx, srcRoot)
		if err != nil {
			return err
		}

		localFiles, err := protobuild.NewBundleResolver(ctx, bundle)
		if err != nil {
			return err
		}

		compiler, err := protobuild.NewPackageSet(deps, localFiles)
		if err != nil {
			return err
		}

		data, err := fs.ReadFile(bundle.FS(), relToBundle)
		if err != nil {
			return err
		}

		lintErr, err := protobuild.LintFile(ctx, compiler, relToBundle, string(data))
		if err != nil {
			return err
		}
		if lintErr == nil {
			fmt.Fprintln(os.Stderr, "No linting errors")
			return nil
		}
		fmt.Fprintln(os.Stderr, lintErr.HumanString(2))
		return fmt.Errorf("Linting failed")
	}

	hadErrors := false

	for _, bundle := range bundles {

		deps, err := bundle.GetDependencies(ctx, srcRoot)
		if err != nil {
			return err
		}

		localFiles, err := protobuild.NewBundleResolver(ctx, bundle)
		if err != nil {
			return err
		}

		compiler, err := protobuild.NewPackageSet(deps, localFiles)
		if err != nil {
			return err
		}

		lintErr, err := protobuild.LintAll(ctx, compiler)
		if err != nil {
			return err
		}
		if lintErr == nil {
			continue
		}
		hadErrors = true
		fmt.Fprintln(os.Stderr, lintErr.HumanString(2))
	}

	if hadErrors {
		return fmt.Errorf("Linting failed")
	}

	fmt.Fprintln(os.Stderr, "No linting errors")
	return nil
}

func runJ5sFmt(ctx context.Context, cfg struct {
	Dir   string `flag:"dir" required:"false" description:"Source / working directory containing j5.yaml and buf.lock.yaml"`
	File  string `flag:"file" required:"false" description:"Single file to format"`
	Write bool   `flag:"write" default:"false" desc:"Write fixes to files"`
}) error {

	var outWriter *fileWriter

	doFile := func(ctx context.Context, pathname string, data []byte) error {
		fixed, err := bcl.Fmt(string(data))
		if err != nil {
			return err
		}
		if !cfg.Write {
			fmt.Printf("Fixed: %s\n", pathname)
			fmt.Println(fixed)
			return nil
		} else {
			return outWriter.PutFile(ctx, pathname, []byte(fixed))
		}
	}

	if cfg.File != "" {
		if cfg.Dir != "" {
			return fmt.Errorf("Cannot specify both dir and file")
		}
		dir, pathname := path.Split(cfg.File)
		outWriter = &fileWriter{dir: dir}

		data, err := os.ReadFile(cfg.File)
		if err != nil {
			return err
		}
		err = doFile(ctx, pathname, data)
		if err != nil {
			return err
		}
		return nil
	}

	return runForJ5Files(ctx, os.DirFS(cfg.Dir), doFile)
}

func runForJ5Files(ctx context.Context, root fs.FS, doFile func(ctx context.Context, pathname string, data []byte) error) error {
	err := fs.WalkDir(root, ".", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if path.Ext(pathname) != ".j5s" {
			return nil
		}

		data, err := fs.ReadFile(root, pathname)
		if err != nil {
			return err
		}

		return doFile(ctx, pathname, data)
	})
	if err != nil {
		return err
	}
	return nil
}

type j5sGenProtoConfig struct {
	SourceConfig
	Verbose bool `flag:"verbose" env:"BCL_VERBOSE" default:"false" desc:"Verbose output"`
}

func runJ5sGenProto(ctx context.Context, cfg j5sGenProtoConfig) error {
	src, err := cfg.GetSource(ctx)
	if err != nil {
		return err
	}

	err = cfg.EachBundle(ctx, func(bundle source.Bundle) error {

		ctx = log.WithField(ctx, "bundle", bundle.DebugName())
		log.Debug(ctx, "GenProto for Bundle")

		deps, err := bundle.GetDependencies(ctx, src)
		if err != nil {
			return err
		}

		localFiles, err := protobuild.NewBundleResolver(ctx, bundle)
		if err != nil {
			return err
		}

		compiler, err := protobuild.NewPackageSet(deps, localFiles)
		if err != nil {
			return err
		}

		err = deleteJ5sProto(ctx, bundle.DirInRepo())
		if err != nil {
			return err
		}

		outWriter, err := cfg.FileWriterAt(ctx, bundle.DirInRepo())
		if err != nil {
			return err
		}

		for _, pkg := range localFiles.ListPackages() {

			out, err := compiler.CompilePackage(ctx, pkg)
			if err != nil {
				return fmt.Errorf("compile package %q: %w", pkg, err)
			}

			for _, file := range out {
				filename := file.Path()
				if !strings.HasSuffix(filename, ".j5s.proto") {
					continue
				}

				out, err := protoprint.PrintFile(ctx, file)
				if err != nil {
					log.WithFields(ctx, map[string]interface{}{
						"error":    err.Error(),
						"filename": file.Path(),
					}).Error("Error printing file")
					return err
				}

				err = outWriter.PutFile(ctx, filename, []byte(out))
				if err != nil {
					return err
				}

			}

		}

		return nil
	})

	if err == nil {
		return nil
	}

	e, ok := errpos.AsErrorsWithSource(err)
	if !ok {
		return err
	}
	fmt.Fprintln(os.Stderr, e.HumanString(3))

	return err
}

func deleteJ5sProto(ctx context.Context, dir string) error {
	err := fs.WalkDir(os.DirFS(dir), ".", func(pathname string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(pathname, ".j5s.proto") {
			// not using path.Ext because it returns .proto
			return nil
		}

		log.WithField(ctx, "file", pathname).Debug("Deleting file")
		return os.Remove(filepath.Join(dir, pathname))
	})
	if err != nil {
		return err
	}
	return nil
}
