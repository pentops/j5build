package j5parse

import (
	"errors"
	"fmt"
	"log"
	"path"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/bufbuild/protovalidate-go"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type Parser struct {
	refl     *j5reflect.Reflector
	Verbose  bool
	FailFast bool
	validate *protovalidate.Validator
}

func NewParser() (*Parser, error) {
	pv, err := protovalidate.New(protovalidate.WithMessages(
		&schema_j5pb.IntegerField_Rules{},
		&schema_j5pb.IntegerField{
			Format: schema_j5pb.IntegerField_FORMAT_INT32,
		},
		&sourcedef_j5pb.SourceFile{},
	), protovalidate.WithDisableLazy(true))
	if err != nil {
		return nil, err
	}
	return &Parser{
		refl:     j5reflect.New(),
		FailFast: true,
		validate: pv,
	}, nil
}

func (p *Parser) fileStub(sourceFilename string) *sourcedef_j5pb.SourceFile {
	dirName, fileName := path.Split(sourceFilename)
	ext := path.Ext(fileName)
	dirName = strings.TrimSuffix(dirName, "/")

	fileName = strings.TrimSuffix(fileName, ext)
	genFilename := fileName + ".gen.proto"

	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	file := &sourcedef_j5pb.SourceFile{
		Path:            genFilename,
		Package:         pathPackage,
		SourceLocations: &sourcedef_j5pb.SourceLocation{},
	}
	return file
}

func (p *Parser) ParseFile(filename string, data string) (*sourcedef_j5pb.SourceFile, error) {

	file := p.fileStub(filename)
	obj, err := p.refl.NewObject(file.ProtoReflect())
	if err != nil {
		return nil, err
	}

	tree, err := ast.ParseFile(data, p.FailFast)
	if err != nil {
		if err == ast.HadErrors {
			return nil, errpos.AddSource(tree.Errors, data)
		}
		return nil, err
	}

	scope, err := schema.NewRootSchemaWalker(J5SchemaSpec, obj, file.SourceLocations)
	if err != nil {
		return nil, err
	}

	err = walker.WalkSchema(scope, tree.Body, p.Verbose)
	if err != nil {
		err = errpos.AddSourceFile(err, filename, data)
		return nil, err
	}

	err = validateFile(p.validate, file)
	if err != nil {
		err = errpos.AddSourceFile(err, filename, data)
		return nil, err
	}

	return file, nil
}

type baseSet struct {
	errors []*errpos.Err
}

type sourceSet struct {
	path []string
	loc  *sourcedef_j5pb.SourceLocation
	base *baseSet
}

func newSourceSet(locs *sourcedef_j5pb.SourceLocation) sourceSet {
	return sourceSet{
		loc:  locs,
		base: &baseSet{},
	}
}

func (s sourceSet) addViolation(violation *validate.Violation) {
	log.Printf("VIOLATION %s %s", violation.FieldPath, violation.Message)
	path := strings.Split(violation.FieldPath, ".")
	fullPath := make([]string, 0)
	for i, p := range path {
		parts := strings.Split(p, "[")
		if len(parts) > 1 {
			idx := parts[1]
			idx = strings.TrimSuffix(idx, "]")
			path[i] = parts[0]
			fullPath = append(fullPath, parts[0], idx)
		} else {
			fullPath = append(fullPath, p)
		}
	}

	ss := s
	for _, p := range fullPath {
		ss = ss.field(p)
		log.Printf("FIELD %s = %d,%d\n", p, ss.loc.StartLine, ss.loc.StartColumn)
	}
	ss.err(fmt.Errorf(violation.Message))
}

func (s sourceSet) field(name string) sourceSet {
	childLoc := s.loc.Children[name]
	if childLoc == nil {
		childLoc = &sourcedef_j5pb.SourceLocation{
			StartLine:   s.loc.StartLine,
			StartColumn: s.loc.StartColumn,
		}
		if s.loc.Children == nil {
			s.loc.Children = make(map[string]*sourcedef_j5pb.SourceLocation)
		}
		s.loc.Children[name] = childLoc
	}
	if childLoc.StartLine == 0 {
		childLoc.StartLine = s.loc.StartLine
		childLoc.StartColumn = s.loc.StartColumn
	}

	child := sourceSet{
		path: append(s.path, name),
		base: s.base,
		loc:  childLoc,
	}
	return child
}

func (ss sourceSet) err(err error) {
	base, ok := errpos.AsError(err)
	if !ok {
		base = &errpos.Err{
			Err: err,
		}
	}

	log.Printf("ERROR %s %s", ss.path, err.Error())
	if ss.loc != nil && base.Pos == nil {
		base.Pos = &errpos.Position{
			Start: errpos.Point{Line: int(ss.loc.StartLine), Column: int(ss.loc.StartColumn)},
			End:   errpos.Point{Line: int(ss.loc.EndLine), Column: int(ss.loc.EndColumn)},
		}
	}
	if len(base.Ctx) == 0 {
		base.Ctx = ss.path
	}
	ss.base.errors = append(ss.base.errors, base)
}

func printSource(loc *sourcedef_j5pb.SourceLocation, prefix string) {
	fmt.Printf("%03d,%03d - %03d,%03d %s\n", loc.StartLine, loc.StartColumn, loc.EndLine, loc.EndColumn, prefix)
	for name, child := range loc.Children {
		printSource(child, prefix+"."+name)
	}
}

func validateFile(pv *protovalidate.Validator, file *sourcedef_j5pb.SourceFile) error {

	sources := newSourceSet(file.SourceLocations)

	printSource(sources.loc, " ROOT")

	validationErr := pv.Validate(file)
	if validationErr != nil {
		valErr := &protovalidate.ValidationError{}
		if errors.As(validationErr, &valErr) {
			//sources.err(valErr)
			for _, violation := range valErr.Violations {
				sources.addViolation(violation)
			}

		} else {
			return validationErr
		}
	}

	if len(sources.base.errors) == 0 {
		return nil
	}

	return errpos.Errors(sources.base.errors)
}
