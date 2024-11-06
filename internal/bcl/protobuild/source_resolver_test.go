package protobuild

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

type testFiles struct {
	localFiles    map[string][]byte
	localPackages []string
}

func newTestFiles() *testFiles {
	return &testFiles{
		localFiles:    map[string][]byte{},
		localPackages: []string{},
	}
}

func (tf *testFiles) tIncludePackage(pkg string) {
	for _, p := range tf.localPackages {
		if p == pkg {
			return
		}
	}
	tf.localPackages = append(tf.localPackages, pkg)
}

func (tf *testFiles) tAddProtoFile(filename string, body ...string) {
	pkg := tFileToPackage(filename)
	body = append([]string{
		`syntax = "proto3";`,
		fmt.Sprintf("package %s;", pkg),
	}, body...)
	tf.localFiles[filename] = []byte(strings.Join(body, "\n"))

	tf.tIncludePackage(pkg)
}

func (tf *testFiles) tAddJ5SFile(filename string, body ...string) {
	pkg := tFileToPackage(filename)
	body = append([]string{
		fmt.Sprintf("package %s", pkg),
	}, body...)
	tf.localFiles[filename] = []byte(strings.Join(body, "\n"))
	tf.tIncludePackage(pkg)
}

func (tf *testFiles) ListPackages() []string {
	return tf.localPackages
}

func (tf *testFiles) ListSourceFiles(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	for k := range tf.localFiles {
		if strings.HasPrefix(k, prefix) {
			files = append(files, k)
		}
	}
	sort.Strings(files) // makes testing easier
	return files, nil
}

func (tf *testFiles) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	if desc, ok := tf.localFiles[filename]; ok {
		return desc, nil
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func TestLocalResolver(t *testing.T) {
	tf := newTestFiles()
	tf.tAddProtoFile("local/v1/foo.proto", "local.v1")
	tf.tAddJ5SFile("local/v1/bar.j5s", "local.v1")

	sourceResolver, err := newSourceResolver(tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}
	pkgName, isLocal, err := sourceResolver.packageForFile("local/v1/foo.proto")
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}
	if !isLocal {
		t.Fatalf("Expected local package, got external")
	}
	if pkgName != "local.v1" {
		t.Fatalf("Expected package name to be local.v1, got %s", pkgName)
	}

	pkgIsLocal := sourceResolver.isLocalPackage("local.v1")
	if !pkgIsLocal {
		t.Fatalf("Expected local package, got external")
	}
}
