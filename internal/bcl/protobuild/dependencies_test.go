package protobuild

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type testDeps struct {
	externalDeps map[string]*descriptorpb.FileDescriptorProto
}

func newTestDeps() *testDeps {
	return &testDeps{
		externalDeps: map[string]*descriptorpb.FileDescriptorProto{},
	}
}

type fileBuilder struct {
	fd *descriptorpb.FileDescriptorProto
}

func (fb *fileBuilder) msg(name string, fields ...*descriptorpb.FieldDescriptorProto) {
	for idx, f := range fields {
		f.Number = proto.Int32(int32(idx + 1))
	}
	fb.fd.MessageType = append(fb.fd.MessageType, &descriptorpb.DescriptorProto{
		Name:  proto.String(name),
		Field: fields,
	})
}

func tFileToPackage(filename string) string {
	dir := path.Dir(filename)
	return strings.ReplaceAll(dir, "/", ".")
}
func (tf *testDeps) tAddSimple(filename string) *fileBuilder {
	pkg := tFileToPackage(filename)
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(filename),
		Syntax:  proto.String("proto3"),
		Package: proto.String(pkg),
	}
	tf.externalDeps[filename] = fd
	return &fileBuilder{
		fd: fd,
	}
}

func (tf *testDeps) GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error) {
	if desc, ok := tf.externalDeps[filename]; ok {
		return desc, nil
	}
	return nil, fmt.Errorf("file not found: %s", filename)
}

func (rf *testDeps) ListDependencyFiles(root string) []string {
	var files []string
	for k := range rf.externalDeps {
		if strings.HasPrefix(k, root) {
			files = append(files, k)
		}
	}
	sort.Strings(files) // makes testing easier
	return files
}

func TestFileLoad(t *testing.T) {
	tf := &testDeps{
		externalDeps: map[string]*descriptorpb.FileDescriptorProto{
			"external/v1/foo.proto": {
				Name:    proto.String("external/v1/foo.proto"),
				Syntax:  proto.String("proto3"),
				Package: proto.String("external.v1"),
			},
		},
	}

	rr, err := newDependencyResolver(tf)
	if err != nil {
		t.Fatalf("FATAL: Unexpected error: %s", err.Error())
	}
	ctx := context.Background()

	t.Run("Inbuilt", func(t *testing.T) {
		path := "j5/list/v1/query.proto"
		result, err := rr.getFile(ctx, path)
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Refl == nil {
			t.Fatal("FATAL: result.Refl is nil")
		}
		assert.Equal(t, path, result.Refl.Path())
	})

	t.Run("External", func(t *testing.T) {
		result, err := rr.getFile(ctx, "external/v1/foo.proto")
		if err != nil {
			t.Fatalf("FATAL: Unexpected error: %s", err.Error())
		}
		if result.Desc == nil {
			t.Fatal("FATAL: result.Desc is nil")
		}
		assert.Equal(t, "external/v1/foo.proto", *result.Desc.Name)
	})
}
