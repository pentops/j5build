package protobuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type testFiles struct {
	localFiles    map[string][]byte
	localPackages []string
	externalDeps  map[string]*descriptorpb.FileDescriptorProto
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

func (tf *testFiles) GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error) {
	if desc, ok := tf.externalDeps[filename]; ok {
		return desc, nil
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func (rf *testFiles) ListDependencyFiles(root string) []string {
	var files []string
	for k := range rf.externalDeps {
		if strings.HasPrefix(k, root) {
			files = append(files, k)
		}
	}
	sort.Strings(files) // makes testing easier
	return files
}

func (tf *testFiles) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	if desc, ok := tf.localFiles[filename]; ok {
		return desc, nil
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func TestFileLoad(t *testing.T) {
	tf := &testFiles{
		localFiles: map[string][]byte{
			"local/v1/foo.proto": []byte("syntax = \"proto3\";"),
		},
		localPackages: []string{"local.v1"},
		externalDeps: map[string]*descriptorpb.FileDescriptorProto{
			"external/v1/foo.proto": {
				Name:    proto.String("external/v1/foo.proto"),
				Syntax:  proto.String("proto3"),
				Package: proto.String("external.v1"),
			},
		},
	}

	rr, err := NewResolver(tf, tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	t.Run("Inbuilt", func(t *testing.T) {
		path := "j5/list/v1/query.proto"
		result, err := rr.findFileByPath(path)
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Result == nil || result.Result.Desc == nil {
			t.Fatal("FATAL: Desc is nil")
		}
		assert.Equal(t, path, result.Result.Desc.Path())
	})

	t.Run("External", func(t *testing.T) {
		result, err := rr.findFileByPath("external/v1/foo.proto")
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Result == nil || result.Result.Proto == nil {
			t.Fatal("FATAL: Proto is nil")
		}
		assert.Equal(t, "external/v1/foo.proto", *result.Result.Proto.Name)
	})

	t.Run("Local Proto", func(t *testing.T) {
		result, err := rr.findFileByPath("local/v1/foo.proto")
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Result == nil {
			t.Fatal("FATAL: Source is nil")
		}
	})

	t.Run("Local j5 file", func(t *testing.T) {
		t.Skip("Not implemented")
		result, err := rr.findFileByPath("local/v1/foo/foo.j5s")
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Result != nil {
			t.Fatal("Somehow find file got a result...")
		}
		if result.J5Source == nil {
			t.Fatal("FATAL: J5Source is nil")
		}

	})

	t.Run("Local j5 service file", func(t *testing.T) {
		t.Skip("Not implemented")
		result, err := rr.findFileByPath("local/v1/foo/service/foo.j5s")
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.J5Source == nil {
			t.Fatal("FATAL: J5 Source is nil")
		}
	})

}

func TestResolveType(t *testing.T) {
	ctx := context.Background()

	tf := &testFiles{
		localFiles: map[string][]byte{
			"local/v1/foo.proto": []byte(`
			syntax = "proto3";
			package local.v1;
			message Foo {
			}
			`),
			"local/v1/bar.j5s": []byte(`
			package local.v1
			object Bar {
			}
			`),
		},
		localPackages: []string{"local.v1"},
		externalDeps: map[string]*descriptorpb.FileDescriptorProto{
			"external/v1/foo.proto": {
				Name:    proto.String("external/v1/foo.proto"),
				Syntax:  proto.String("proto3"),
				Package: proto.String("external.v1"),
			},
		},
	}

	rr, err := NewResolver(tf, tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	cc := NewCompiler(rr)

	out, err := cc.CompilePackage(ctx, "local.v1")
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	for _, file := range out {
		t.Logf("GOT FILE %s", file.Path())
	}
}

func TestCircularDependency(t *testing.T) {

	ctx := context.Background()

	tf := &testFiles{
		localFiles: map[string][]byte{
			"foo/v1/foo.proto": []byte(`
				syntax = "proto3";
				package foo.v1;
				import "bar/v1/bar.proto";
			`),
			"bar/v1/bar.proto": []byte(`
				syntax = "proto3";
				package bar.v1;
				import "baz/v1/baz.proto";
			`),
			"baz/v1/baz.proto": []byte(`
				syntax = "proto3";
				package baz.v1;
				import "foo/v1/foo.proto";
			`),
		},
		localPackages: []string{"foo.v1", "bar.v1", "baz.v1"},
	}

	rr, err := NewResolver(tf, tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}

	cc := NewCompiler(rr)

	_, err = cc.loadPackage(ctx, "foo.v1", nil)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
	cde := &CircularDependencyError{}

	if !errors.As(err, &cde) {
		t.Fatalf("Expected CircularDependencyError, got %T (%s)", err, err.Error())
	}
}
