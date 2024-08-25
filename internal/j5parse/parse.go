package j5parse

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/bufbuild/protovalidate-go"
	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5/lib/j5reflect"
)

type Parser struct {
	refl *j5reflect.Reflector
}

func NewParser() *Parser {
	return &Parser{
		refl: j5reflect.New(),
	}
}

func (p *Parser) ParseFile(filename string, data string) (*sourcedef_j5pb.SourceFile, error) {

	dirName, fileName := path.Split(filename)
	ext := path.Ext(fileName)
	dirName = strings.TrimSuffix(dirName, "/")

	fileName = strings.TrimSuffix(fileName, ext)
	genFilename := fileName + ".gen.proto"
	_ = genFilename

	pathPackage := strings.Join(strings.Split(dirName, "/"), ".")
	file := &sourcedef_j5pb.SourceFile{
		Path:            genFilename,
		Package:         pathPackage,
		SourceLocations: &sourcedef_j5pb.SourceLocation{},
	}
	obj, err := p.refl.NewObject(file.ProtoReflect())
	if err != nil {
		return nil, err
	}

	err = bcl.ParseIntoSchema(data, obj, file.SourceLocations, Spec)
	if err != nil {
		err = errpos.AddSourceFile(err, filename, data)
		return nil, err
	}

	err = walkFile(file)
	if err != nil {
		err = errpos.AddSourceFile(err, filename, data)
		return nil, err
	}

	return file, nil
}

type baseSet struct {
	errors    []*errpos.Err
	validator *protovalidate.Validator
}

type sourceSet struct {
	path []string
	loc  *sourcedef_j5pb.SourceLocation
	base *baseSet
}

func newSourceSet(locs *sourcedef_j5pb.SourceLocation) (sourceSet, error) {
	vv, err := protovalidate.New()
	if err != nil {
		return sourceSet{}, err
	}
	return sourceSet{
		loc: locs,
		base: &baseSet{
			validator: vv,
		},
	}, nil
}

func (s sourceSet) addViolation(violation *validate.Violation) {
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
	fmt.Printf("WALKING\n")
	for _, p := range fullPath {
		fmt.Printf("  Check %s\n", p)
		ss = ss.field(p)
		fmt.Printf("  Location %d\n", ss.loc.StartLine)
	}
	ss.err(fmt.Errorf(violation.Message))
}

func (s sourceSet) prop(name string) sourceSet {
	return s.field(name)
}

func (s sourceSet) idx(idx int) sourceSet {
	return s.field(fmt.Sprintf("%d", idx))
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
	if ss.loc != nil && base.Pos == nil {
		base.Pos = &errpos.Position{
			Line:   int(ss.loc.StartLine),
			Column: int(ss.loc.StartColumn),
		}
	}
	if len(base.Ctx) == 0 {
		base.Ctx = ss.path
	}
	ss.base.errors = append(ss.base.errors, base)
}

func printSource(loc *sourcedef_j5pb.SourceLocation, indent int) {
	for name, child := range loc.Children {
		fmt.Printf("%s%s  AT %d %d\n", strings.Repeat(" ", indent), name, child.StartLine, child.StartColumn)
		printSource(child, indent+2)
	}
}

func walkFile(file *sourcedef_j5pb.SourceFile) error {

	sources, err := newSourceSet(file.SourceLocations)
	if err != nil {
		return err
	}

	printSource(sources.loc, 0)

	elementsSource := sources.prop("elements")
	for idx, element := range file.Elements {
		elementSource := elementsSource.idx(idx)

		switch st := element.Type.(type) {
		case *sourcedef_j5pb.RootElement_Object:
			if st.Object.Def == nil {
				err := fmt.Errorf("missing object definition")
				sources.err(err)
				continue
			}
			walkObject(elementSource, st.Object.Def)
			/*
				case *sourcedef_j5pb.RootElement_Enum:
					err := walkEnum(st.Enum)
					if err != nil {
						return errpos.AddContext(err, "enum")
					}
					return nil*/
		case *sourcedef_j5pb.RootElement_Oneof:
			if st.Oneof.Def == nil {
				err := fmt.Errorf("missing oneof definition")
				elementsSource.err(err)
				continue
			}
			walkOneof(elementSource, st.Oneof.Def)

		case *sourcedef_j5pb.RootElement_Entity:
			walkEntity(elementSource, st.Entity)

		default:
			err := fmt.Errorf("unknown element type %T", st)
			elementsSource.err(err)
		}

	}

	validationErr := sources.base.validator.Validate(file)
	if validationErr != nil {
		valErr := &protovalidate.ValidationError{}
		if errors.As(validationErr, &valErr) {
			sources.err(valErr)
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

func walkObject(source sourceSet, obj *schema_j5pb.Object) {
	propSources := source.field("properties")
	for idx, prop := range obj.Properties {
		propSource := propSources.idx(idx)
		prop.ProtoField = []int32{int32(idx) + 1}
		walkProperty(propSource, prop)
	}

}

func walkOneof(source sourceSet, oneof *schema_j5pb.Oneof) {
	propSources := source.field("properties")
	for idx, prop := range oneof.Properties {
		propSource := propSources.idx(idx)
		prop.ProtoField = []int32{int32(idx) + 1}

		if obj := prop.Schema.GetObject(); obj != nil {
			if sch := obj.GetObject(); sch != nil {
				if sch.Name == "" {
					sch.Name = strcase.ToCamel(prop.Name)
				}
			}
		}

		walkProperty(propSource, prop)
	}

}

func walkEntity(source sourceSet, entity *sourcedef_j5pb.Entity) {

	datasSource := source.field("data")
	for idx, prop := range entity.Data {
		propSource := datasSource.idx(idx)
		prop.ProtoField = []int32{int32(idx) + 1}
		walkProperty(propSource, prop)
	}

	keysSource := source.field("keys")
	for idx, prop := range entity.Keys {
		propSource := keysSource.idx(idx)
		prop.ProtoField = []int32{int32(idx) + 1}
		walkProperty(propSource, prop)
	}

	eventsSource := source.field("events")
	for idx, evt := range entity.Events {
		eventSource := eventsSource.idx(idx)
		if evt.Def == nil {
			err := fmt.Errorf("missing event definition")
			eventSource.err(err)
			continue
		}
		walkObject(eventSource, evt.Def)
	}

}

func walkProperty(source sourceSet, prop *schema_j5pb.ObjectProperty) {
	if prop.Schema == nil {
		source.err(fmt.Errorf("missing schema"))
		return
	}
	switch st := prop.Schema.Type.(type) {
	case *schema_j5pb.Field_Object:
		obj := st.Object

		switch st := obj.Schema.(type) {
		case *schema_j5pb.ObjectField_Object:
			walkObject(source.field("schema").field("type").field("object").field("schema").field("object"), st.Object)

		case *schema_j5pb.ObjectField_Ref:
			//
		default:
			source.err(fmt.Errorf("unknown object schema %T, must set Ref or Object", st))
		}
	}
}
