package sourcewalk

import (
	"fmt"
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"golang.org/x/exp/maps"
)

// WalkerError is returned when the source structure can't be walked, usually
// indicating a bug or unhandled edge case in the schema.
type WalkerError string

func (we WalkerError) Error() string {
	return fmt.Sprintf("walker: %s", string(we))
}
func walkerErrorf(f string, p ...any) WalkerError {
	return WalkerError(fmt.Sprintf(f, p...))
}

type SourceNode struct {
	Path    []string
	Source  *bcl_j5pb.SourceLocation
	virtual bool
}

func (sn SourceNode) PathString() string {
	return strings.Join(sn.Path, ".")
}

const virtualPathNode = "<virtual>"

func (sn SourceNode) child(path ...string) SourceNode {
	walk := sn
	virtual := sn.virtual
	for _, part := range path {
		nodePath := make([]string, len(walk.Path)+1)
		copy(nodePath, walk.Path)
		nodePath[len(walk.Path)] = part

		if part == virtualPathNode {
			virtual = true
		} else {
			nextLoc, ok := walk.Source.Children[part]
			if ok {
				walk = SourceNode{
					Path:    nodePath,
					Source:  nextLoc,
					virtual: virtual,
				}
				continue
			}
		}

		if !virtual {
			options := maps.Keys(walk.Source.Children)
			log.Printf("No source child %q in %s (%v), have %q", part, walk.PathString(), walk.virtual, options)
		}

		newNode := SourceNode{
			Path: nodePath,
			Source: &bcl_j5pb.SourceLocation{
				StartLine:   walk.Source.StartLine,
				StartColumn: walk.Source.StartColumn,
				EndLine:     walk.Source.EndLine,
				EndColumn:   walk.Source.EndColumn,
			},
			virtual: true,
		}

		walk = newNode
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
		root.Path = []string{virtualPathNode}
		root.virtual = true
	}

	fn := &FileNode{
		SourceFile: file,
		Source:     root,
	}

	return fn
}
