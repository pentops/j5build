package sourcewalk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
)

func walkLoc(walk *bcl_j5pb.SourceLocation, path ...string) *bcl_j5pb.SourceLocation {
	for _, part := range path {
		if walk.Children == nil {
			walk.Children = map[string]*bcl_j5pb.SourceLocation{}
		}
		nextLoc := walk.Children[part]
		if nextLoc == nil {
			nextLoc = &bcl_j5pb.SourceLocation{}
			walk.Children[part] = nextLoc
		}
		walk = nextLoc
	}
	return walk
}
func TestSourceNode(t *testing.T) {

	root := &bcl_j5pb.SourceLocation{
		StartLine: 1,
	}
	walkLoc(root, "foo", "bar").StartLine = 2
	walkLoc(root, "foo", "bar", "def", "properties", "0").StartLine = 3

	printSource(t, root, []string{})
	ww := SourceNode{
		Path:   []string{},
		Source: root,
	}

	assert := func(sn SourceNode, line int32) {
		t.Helper()
		if sn.Source.StartLine != line {
			t.Errorf("StartLine = %d, want %d", sn.Source.StartLine, line)
		}
	}

	wrap := ww.child("foo", "bar")
	assert(wrap.child("def"), 0)
	assert(wrap.child(virtualPathNode, "wrapper"), 2)
}

func printSource(t testing.TB, loc *bcl_j5pb.SourceLocation, path []string) {
	if loc == nil {
		fmt.Printf("NIL LOC\n")
		return
	}
	t.Logf("%03d:%03d %s\n",
		loc.StartLine, loc.StartColumn,
		strings.Join(path, "."))
	for k, v := range loc.Children {
		printSource(t, v, append(path, k))
	}
}
