package protosrc

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testFS(files map[string]string) fstest.MapFS {
	out := make(fstest.MapFS)
	for name, content := range files {
		out[name] = &fstest.MapFile{
			Data: []byte(content),
		}
	}

	return fstest.MapFS(out)
}
func TestGenerateRequest(t *testing.T) {

	src := testFS(map[string]string{
		"file1.proto": `
		syntax = "proto3";
		import "buf/validate/validate.proto";
		package foo; 

		// Comment on Bar
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

	ctx := context.Background()
	img, err := ReadFSImage(ctx, src, []string{"file1.proto", "file2.proto"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	codegenReq, err := CodeGeneratorRequestFromImage(img)
	if err != nil {
		t.Fatal(err)
	}

	file1 := codegenReq.SourceFileDescriptors[0]
	assert.Equal(t, "file1.proto", file1.GetName())

	var barSrc *descriptorpb.SourceCodeInfo_Location

	assert.NotNil(t, file1.SourceCodeInfo)
	for _, loc := range file1.SourceCodeInfo.Location {
		if slices.Equal(loc.Path, []int32{4, 0}) {
			barSrc = loc
		}
	}

	if barSrc == nil {
		t.Fatal("no source code info for Bar")
	}

	assert.Equal(t, " Comment on Bar\n", *barSrc.LeadingComments)

}
