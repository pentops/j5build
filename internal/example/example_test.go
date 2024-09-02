package example

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/internal/protobuild"
	"github.com/pentops/bcl.go/internal/protoprint"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/descriptorpb"
)

type testDeps struct {
	files map[string]*descriptorpb.FileDescriptorProto
}

func (td *testDeps) GetDependencyFile(filename string) (*descriptorpb.FileDescriptorProto, error) {
	if f, ok := td.files[filename]; ok {
		return f, nil
	}
	return nil, fmt.Errorf("file %q not found", filename)
}

func TestFull(t *testing.T) {

	deps := &testDeps{
		files: map[string]*descriptorpb.FileDescriptorProto{},
	}

	localFiles := &fsReader{
		fs: os.DirFS("."),
		packages: []string{
			"pack1.v1",
		},
	}

	ctx := context.Background()

	resolver, err := protobuild.NewResolver(deps, localFiles)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	filename := "pack1/v1/foo.j5gen.proto"

	built, err := resolver.Compile(ctx, filename)
	if err != nil {
		t.Fatal(err.Error())
	}
	fileOut := built[0]
	assert.Equal(t, filename, fileOut.Path())

	out, err := protoprint.PrintFile(ctx, fileOut)
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Logf("GOT FILE \n%s", out)

	want, err := localFiles.GetLocalFile(ctx, filename)
	if err != nil {
		t.Fatal(err.Error())
	}

	strWant := string(want)
	cmpLines(t, strWant, out)
}

type lineset struct {
	lines  []string
	offset int
}

func (ls lineset) more() bool {
	return ls.offset < len(ls.lines)
}

func (ls *lineset) next() (int, string) {
	if ls.offset >= len(ls.lines) {
		ls.offset++
		return ls.offset, "<none>"
	}

	line := ls.lines[ls.offset]
	ls.offset++
	return ls.offset, line
}

func cmpLines(t *testing.T, wantStr, gotStr string) {
	t.Helper()
	wants := lineset{lines: strings.Split(wantStr, "\n")}
	gots := lineset{lines: strings.Split(gotStr, "\n")}

	for wants.more() || gots.more() {
		if !wants.more() {
			line, got := gots.next()
			t.Errorf("     %03d: ++ %s", line, got)
		} else if !gots.more() {
			line, want := wants.next()
			t.Logf("%03d     : -- %s", line, want)
		}

		wantLine, wantTxt := wants.next()
		gotLine, gotTxt := gots.next()

		if gotTxt != wantTxt {
			t.Logf("%03d     : -- %s", wantLine, wantTxt)
			t.Errorf("     %03d: ++ %s", gotLine, gotTxt)
		} else {
			t.Logf("%03d %03d:     %s", wantLine, gotLine, wantTxt)
		}
	}
}

type fsReader struct {
	fs       fs.FS
	packages []string
}

func (rr *fsReader) GetLocalFile(ctx context.Context, filename string) ([]byte, error) {
	return fs.ReadFile(rr.fs, filename)
}

func (rr *fsReader) ListPackages() []string {
	return rr.packages
}

func (rr *fsReader) ListSourceFiles(ctx context.Context, pkgName string) ([]string, error) {

	pkgRoot := strings.ReplaceAll(pkgName, ".", "/")

	files := make([]string, 0)
	err := fs.WalkDir(rr.fs, pkgRoot, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".j5gen.proto") {
			return nil
		}
		if strings.HasSuffix(path, ".proto") {
			files = append(files, path)
		}
		if strings.HasSuffix(path, ".j5s") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil

}
