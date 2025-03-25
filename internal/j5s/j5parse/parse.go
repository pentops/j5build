package j5parse

import (
	"path"
	"strings"

	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/bcl"
	"github.com/pentops/j5build/internal/bcl/gen/j5/bcl/v1/bcl_j5pb"
)

type Parser struct {
	bcl *bcl.Parser
}

func NewParser() (*Parser, error) {
	p, err := bcl.NewParser(J5SchemaSpec)
	if err != nil {
		return nil, err
	}
	return &Parser{bcl: p}, nil
}

func (p *Parser) fileStub(sourceFilename string) *sourcedef_j5pb.SourceFile {
	dirName, _ := path.Split(sourceFilename)
	dirName = strings.TrimSuffix(dirName, "/")

	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	file := &sourcedef_j5pb.SourceFile{
		Path: sourceFilename,
		Package: &sourcedef_j5pb.Package{
			Name: pathPackage,
		},
		SourceLocations: &bcl_j5pb.SourceLocation{},
	}
	return file

}

func (p *Parser) ParseFile(filename string, data string) (*sourcedef_j5pb.SourceFile, error) {
	file := p.fileStub(filename)
	refl := file.ProtoReflect()
	sourceLocs, err := p.bcl.ParseFile(filename, data, refl)
	if err != nil {
		return nil, err
	}
	file.SourceLocations = sourceLocs

	return file, nil

}
