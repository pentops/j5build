package reader

import (
	"io"
	"io/fs"

	"github.com/bufbuild/protocompile"
)

func NewFSResolver(fs fs.FS) protocompile.Resolver {
	return &protocompile.SourceResolver{
		Accessor: func(filename string) (io.ReadCloser, error) {
			return fs.Open(filename)
		},
	}
}
