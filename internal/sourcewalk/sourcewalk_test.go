package sourcewalk

import (
	"testing"

	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
)

func TestSourceNode(t *testing.T) {

	root := &bcl_j5pb.SourceLocation{
		StartLine: 1,
	}
	walkLoc(root, "foo", "bar").StartLine = 2
	walkLoc(root, "foo", "bar", "def", "properties", "0").StartLine = 3

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
