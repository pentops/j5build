package reader

import (
	"context"
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type testResolver struct {
}

func TestCompile(t *testing.T) {

	mr := protocompile.SourceAccessorFromMap(map[string]string{
		"file1.proto": `
		syntax = "proto3";
		import "buf/validate/validate.proto";
		package foo; 
		message Bar {
			string name = 1 [(buf.validate.field).string.min_len = 1];
		}
		`,
		"file2.proto": `
		syntax = "proto3";
		import "file1.proto";
		import "buf/validate/validate.proto";
		package foo; 
		message Baz {
			string name = 1 [(buf.validate.field).string.min_len = 1];
			Bar bar = 2;
		}
		`,
	})
	resolver := &protocompile.SourceResolver{
		Accessor: mr,
	}
	cc := NewCompiler(resolver)
	ctx := context.Background()
	files, err := cc.Compile(ctx, []string{"file1.proto", "file2.proto"})
	if err != nil {
		t.Fatal(err)
	}

	df2, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: files,
	})
	if err != nil {
		t.Fatal(err)
	}

	bar, err := df2.FindDescriptorByName("foo.Bar")
	if err != nil {
		t.Fatal(err)
	}
	field := bar.(protoreflect.MessageDescriptor).Fields().ByName("name")
	if field == nil {
		t.Fatal("field not found")
	}

	proto.GetExtension(field.Options(), validate.E_Field)

}
