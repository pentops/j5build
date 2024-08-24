package j5parse

import (
	"fmt"
	"path"
	"strings"

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
		Package: pathPackage,
	}
	obj, err := p.refl.NewObject(file.ProtoReflect())
	if err != nil {
		return nil, err
	}

	err = bcl.ParseIntoSchema(data, obj, Spec)
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

func srcLoc(loc *sourcedef_j5pb.SourceLocation) errpos.Position {
	if loc == nil {
		return errpos.Position{}
	}
	return errpos.Position{
		Filename: loc.Path,
		Line:     int(loc.Line),
		Column:   int(loc.Column),
	}
}
func walkFile(file *sourcedef_j5pb.SourceFile) error {
	errors := errpos.Errors{}

	for _, element := range file.Elements {

		switch st := element.Type.(type) {
		case *sourcedef_j5pb.RootElement_Object:
			if st.Object.Def == nil {
				err := fmt.Errorf("missing object definition")
				err = errpos.AddPosition(err, srcLoc(st.Object.Location))
				err = errpos.AddContext(err, "object")
				errors = errors.Append(err)
			}
			err := walkObject(st.Object.Def)
			if err != nil {
				err = errpos.AddPosition(err, srcLoc(st.Object.Location))
				err = errpos.AddContext(err, "object")
				errors = errors.Append(err)
			}
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
				err = errpos.AddPosition(err, srcLoc(st.Oneof.Location))
				err = errpos.AddContext(err, "oneof")
				errors = errors.Append(err)
			}
			err := walkOneof(st.Oneof.Def)
			if err != nil {
				err = errpos.AddPosition(err, srcLoc(st.Oneof.Location))
				err = errpos.AddContext(err, "oneof")
				errors = errors.Append(err)
			}
		case *sourcedef_j5pb.RootElement_Entity:
			err := walkEntity(st.Entity)
			if err != nil {
				err = errpos.AddPosition(err, srcLoc(st.Entity.Location))
				err = errpos.AddContext(err, "entity")
				errors = errors.Append(err)
			}

		default:
			err := fmt.Errorf("unknown element type %T", st)
			errors = errors.Append(err)
		}

	}

	if len(errors) == 0 {
		return nil
	}

	return errors
}

func walkObject(obj *schema_j5pb.Object) error {
	for idx, prop := range obj.Properties {
		prop.ProtoField = []int32{int32(idx) + 1}
		if err := walkProperty(prop); err != nil {
			return errpos.AddContext(err, prop.Name)
		}
	}

	return nil
}

func walkOneof(oneof *schema_j5pb.Oneof) error {
	for idx, prop := range oneof.Properties {
		prop.ProtoField = []int32{int32(idx) + 1}

		if err := walkProperty(prop); err != nil {
			return errpos.AddContext(err, prop.Name)
		}
	}

	return nil
}

func walkEntity(entity *sourcedef_j5pb.Entity) error {
	if entity.Data == nil {
		entity.Data = &sourcedef_j5pb.Object{}
	}

	if entity.Data.Def == nil {
		entity.Data.Def = &schema_j5pb.Object{
			Name: entity.Name + "Data",
		}
	}
	if err := walkObject(entity.Data.Def); err != nil {
		err = errpos.AddPosition(err, srcLoc(entity.Data.Location))
		return errpos.AddContext(err, "data")
	}

	for _, evt := range entity.Events {
		if evt.Def == nil {
			return fmt.Errorf("missing event definition")
		}
		if err := walkObject(evt.Def); err != nil {
			err = errpos.AddPosition(err, srcLoc(evt.Location))
			return errpos.AddContext(err, "event")
		}
	}

	return nil
}

func walkProperty(prop *schema_j5pb.ObjectProperty) error {
	if prop.Schema == nil {
		return fmt.Errorf("missing schema")
	}
	switch st := prop.Schema.Type.(type) {
	case *schema_j5pb.Field_Object:
		obj := st.Object
		switch st := obj.Schema.(type) {
		case *schema_j5pb.ObjectField_Object:
			if err := walkObject(st.Object); err != nil {
				return err
			}

		case *schema_j5pb.ObjectField_Ref:
			//
		default:
			return fmt.Errorf("unknown object schema %T, must set Ref or Object", st)
		}
	}
	return nil
}
