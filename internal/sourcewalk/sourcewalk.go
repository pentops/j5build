package sourcewalk

import (
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"golang.org/x/exp/maps"
)

type SourceNode struct {
	Path          []string
	Source        *bcl_j5pb.SourceLocation
	DerivedSource bool
}

func (sn SourceNode) maybeChild(path ...string) SourceNode {
	return sn._child(false, path...)
}

func (sn SourceNode) child(path ...string) SourceNode {
	return sn._child(true, path...)
}

func (sn SourceNode) _child(must bool, path ...string) SourceNode {
	walk := sn
	for _, part := range path {
		nextLoc := walk.Source.Children[part]
		if nextLoc != nil {
			walk = SourceNode{
				Path:          append(walk.Path, part),
				Source:        nextLoc,
				DerivedSource: false,
			}
			continue
		}
		if !walk.DerivedSource {
			if must {
				options := maps.Keys(walk.Source.Children)
				log.Printf("No source child %q in %s, have %q", part, strings.Join(walk.Path, "."), options)
				panic("Missing Source Node")
			}
		}

		loc := walk.Source
		walk = SourceNode{
			Path: append(walk.Path, part),
			Source: &bcl_j5pb.SourceLocation{
				StartLine:   loc.StartLine,
				StartColumn: loc.StartColumn,
				EndLine:     loc.EndLine,
				EndColumn:   loc.EndColumn,
			},
			DerivedSource: true,
		}
	}

	return walk
}

func (sn SourceNode) GetPos() *errpos.Position {
	return &errpos.Position{
		Start: errpos.Point{
			Line:   int(sn.Source.StartLine),
			Column: int(sn.Source.StartColumn),
		},
		End: errpos.Point{
			Line:   int(sn.Source.EndLine),
			Column: int(sn.Source.EndColumn),
		},
	}
}

func NewRoot(file *sourcedef_j5pb.SourceFile) *FileNode {

	root := SourceNode{
		Path:   []string{},
		Source: file.SourceLocations,
	}

	if root.Source == nil {
		root.Source = &bcl_j5pb.SourceLocation{}
		root.DerivedSource = true
	}

	fn := &FileNode{
		SourceFile: file,
		Source:     root,
	}

	return fn
}
