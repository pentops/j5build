package j5convert

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func PackageFromFilename(filename string) string {
	dirName, _ := path.Split(filename)
	dirName = strings.TrimSuffix(dirName, "/")
	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	return pathPackage
}

var reVersion = regexp.MustCompile(`^v\d+$`)

func SplitPackageFromFilename(filename string) (string, string, error) {
	pkg := PackageFromFilename(filename)
	parts := strings.Split(pkg, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid package %q for file %q", pkg, filename)
	}

	// foo.v1 -> foo, v1
	// foo.v1.service -> foo.v1, service
	// foo.bar.v1.service -> foo.bar.v1, service

	if reVersion.MatchString(parts[len(parts)-1]) {
		return pkg, "", nil
	}
	if reVersion.MatchString(parts[len(parts)-2]) {
		upToVersion := parts[:len(parts)-1]
		return strings.Join(upToVersion, "."), parts[len(parts)-1], nil
	}
	return pkg, "", fmt.Errorf("no version in package %q", pkg)
}

type J5Result struct {
	SourceFile *sourcedef_j5pb.SourceFile
	Summary    *FileSummary
}
