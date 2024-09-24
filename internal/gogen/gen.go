package gogen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/lib/patherr"
)

type Options struct {
	TrimPackagePrefix   string
	FilterPackagePrefix string
	GoPackagePrefix     string
}

// ReferenceGoPackage returns the go package for the given proto package. It may
// be within the generated code, or a reference to an external package.
func (o Options) ReferenceGoPackage(pkg string) (string, error) {
	if pkg == "" {
		return "", fmt.Errorf("empty package")
	}

	if !strings.HasPrefix(pkg, o.FilterPackagePrefix) {
		return "", fmt.Errorf("package %s not in prefix %s", pkg, o.FilterPackagePrefix)
	}

	if o.TrimPackagePrefix != "" {
		pkg = strings.TrimPrefix(pkg, o.TrimPackagePrefix)
	}

	pkg = strings.TrimSuffix(pkg, ".service")
	pkg = strings.TrimSuffix(pkg, ".topic")
	pkg = strings.TrimSuffix(pkg, ".sandbox")

	parts := strings.Split(pkg, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid package: %s", pkg)
	}
	nextName := parts[len(parts)-2]
	parts = append(parts, nextName)

	pkg = strings.Join(parts, "/")

	pkg = path.Join(o.GoPackagePrefix, pkg)
	return pkg, nil

}
func WriteGoCode(api *client_j5pb.API, output FileWriter, options Options) error {

	/*
		reflect, err := j5schema.APIFromDesc(api)
		if err != nil {
			return err
		}*/
	fileSet := NewFileSet(options.GoPackagePrefix)

	bb := &builder{
		fileSet: fileSet,
		options: options,
	}

	for _, j5Package := range api.Packages {
		if err := bb.addPackage(j5Package); err != nil {
			return patherr.Wrap(err, j5Package.Name)
		}
	}

	return fileSet.WriteAll(output)
}

type FileWriter interface {
	WriteFile(name string, data []byte) error
}

type DirFileWriter string

func (fw DirFileWriter) WriteFile(relPath string, data []byte) error {
	fullPath := filepath.Join(string(fw), relPath)
	dirName := filepath.Dir(fullPath)
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return fmt.Errorf("mkdirall for %s: %w", fullPath, err)
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("writefile for %s: %w", fullPath, err)
	}
	return nil
}
