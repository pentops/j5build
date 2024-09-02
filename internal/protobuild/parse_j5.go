package protobuild

import (
	"github.com/pentops/bcl.go/internal/j5parse"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type J5Parser struct {
	fileParser *j5parse.Parser
}

func NewJ5Parser() (*J5Parser, error) {
	j5Parser, err := j5parse.NewParser()
	if err != nil {
		return nil, err
	}

	return &J5Parser{
		fileParser: j5Parser,
	}, nil
}

func (jp *J5Parser) parseToSourceDescriptor(sourceFilename string, data []byte) (*sourcedef_j5pb.SourceFile, error) {
	return jp.fileParser.ParseFile(sourceFilename, string(data))
}
