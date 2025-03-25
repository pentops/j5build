package j5convert

import (
	"strings"

	"github.com/pentops/golib/gl"
)

type comment struct {
	path        []int32
	description *string
}

type commentSet []*comment

func (cs *commentSet) comment(path []int32, description string) {
	cc := &comment{
		path: path,
	}

	if description != "" {
		lines := strings.Split(description, "\n")
		joined := " " + strings.Join(lines, "\n ") + "\n"
		cc.description = gl.Ptr(joined)
	}
	*cs = append(*cs, cc)
}

// mergeAt adds the comments in the nested set to this set rooted at 'path'
func (cs *commentSet) mergeAt(path []int32, nested commentSet) {
	for _, input := range nested {

		thisPath := make([]int32, len(path)+len(input.path))
		copy(thisPath, path)
		copy(thisPath[len(path):], input.path)

		newComment := &comment{
			path:        thisPath,
			description: input.description,
		}
		*cs = append(*cs, newComment)
	}
}

/*
func pathString(path []int32) string {
	if len(path) == 0 {
		return "<root>"
	}
	if len(path) < 2 {
		return fmt.Sprintf("!%v", path)
	}
	switch path[0] {
	case 4:
		index := path[1]
		return fmt.Sprintf("message[%d] %s", index, messagePathString(path[2:]))
	case 5:
		return "enum " + pathString(path[1:])
	case 6:
		return "service " + pathString(path[1:])
	default:
		return fmt.Sprintf("%v", path)
	}
}

func messagePathString(path []int32) string {
	if len(path) == 0 {
		return "<root>"
	}
	if len(path) < 2 {
		return fmt.Sprintf("!%v", path)
	}
	switch path[0] {
	case 3:
		index := path[1]
		return fmt.Sprintf("nested[%d] %s", index, messagePathString(path[2:]))
	case 4:
		return "enum " + messagePathString(path[1:])
	default:
		return fmt.Sprintf("?%v", path)
	}
}*/
