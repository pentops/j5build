package gogen

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"path/filepath"
	"strings"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
)

type FileSet struct {
	files  map[string]*GeneratedFile
	prefix string
}

func NewFileSet(prefix string) *FileSet {
	return &FileSet{
		files:  make(map[string]*GeneratedFile),
		prefix: prefix,
	}
}

func (fileSet *FileSet) WriteAll(output FileWriter) error {
	for fullPackageName, file := range fileSet.files {
		file.addCombinedClient()
		log.Printf("Writing %s", fullPackageName)
		bb, err := file.ExportBytes()
		if err != nil {
			return err
		}
		fileName := strings.TrimPrefix(fullPackageName, fileSet.prefix)
		if err := output.WriteFile(filepath.Join(fileName, "generated.go"), bb); err != nil {
			return err
		}
	}
	return nil
}

func (fs *FileSet) File(packagePath string, packageName string) (*GeneratedFile, error) {
	file, ok := fs.files[packagePath]
	if !ok {
		file = NewFile(packagePath, packageName)
		fs.files[packagePath] = file
		return file, nil
	}

	if file.packageName != packageName {
		return nil, fmt.Errorf("file %s already exists with package name %s, adding %s", packagePath, file.packageName, packageName)
	}

	return file, nil
}

type FileElement interface {
	Print(gen *StringGen)
}

type GeneratedFile struct {
	path        string
	packageName string

	elements    []FileElement
	_interfaces map[string]*Interface
	_services   map[string]*Struct
	_enums      map[string]*Enum
	_types      map[string]*Struct
	_funcs      map[string]*Function

	*StringGen
}

func NewFile(importPath string, packageName string) *GeneratedFile {
	gen := &GeneratedFile{
		path:        importPath,
		packageName: packageName,
		_services:   make(map[string]*Struct),
		_enums:      make(map[string]*Enum),
		_types:      make(map[string]*Struct),
		_funcs:      make(map[string]*Function),
		_interfaces: make(map[string]*Interface),
		StringGen: &StringGen{
			buf:       &bytes.Buffer{},
			imports:   make(map[string]string),
			myPackage: importPath,
		},
	}

	return gen
}

type PackagedIdentity interface {
	PackageName() string
	Identity() string
}

func (gen *GeneratedFile) addCombinedClient() {
	if len(gen._services) == 0 {
		return
	}

	constructor := &Function{
		Name: "NewCombinedClient",
		Parameters: []*Parameter{{
			Name: "requester",
			DataType: DataType{
				Name: "Requester",
			},
		}},
		Returns: []*Parameter{{
			DataType: DataType{
				Name:    "CombinedClient",
				Pointer: true,
			},
		}},
		StringGen: gen.ChildGen(),
	}

	combined := &Struct{
		Name:         "CombinedClient",
		Constructors: []*Function{constructor},
	}

	constructor.P("  return &CombinedClient{")
	for _, service := range gen._services {
		combined.Fields = append(combined.Fields, &Field{
			//Name:     service.Name,
			DataType: DataType{Name: service.Name, Pointer: true},
		})
		constructor.P("  ", service.Name, ": New", service.Name, "(requester),")
	}
	constructor.P("  }")

	gen._types["CombinedClient"] = combined
	gen.elements = append(gen.elements, combined)
}

func (gen *GeneratedFile) Service(serviceName string) *Struct {
	existing, ok := gen._services[serviceName]
	if ok {
		return existing
	}

	constructor := &Function{
		Name: fmt.Sprintf("New%s", serviceName),
		Parameters: []*Parameter{{
			Name: "requester",
			DataType: DataType{
				Name: "Requester",
			},
		}},
		Returns: []*Parameter{{
			DataType: DataType{
				Name:    serviceName,
				Pointer: true,
			},
		}},
		StringGen: gen.ChildGen(),
	}

	constructor.P("  return &", serviceName, "{")
	constructor.P("    Requester: requester,")
	constructor.P("  }")

	service := &Struct{
		Name: serviceName,
		Fields: []*Field{{
			DataType: DataType{
				Name: "Requester",
			},
			// Anon
		}},
		Constructors: []*Function{constructor},
	}

	gen._services[serviceName] = service
	gen.elements = append(gen.elements, service)
	return service
}

func (gen *GeneratedFile) EnsureInterface(ii *Interface) {
	_, ok := gen._interfaces[ii.Name]
	if ok {
		return
	}
	// TODO: Compare Methods

	gen._interfaces[ii.Name] = ii
	gen.elements = append(gen.elements, ii)
}

func (gen *GeneratedFile) AddStruct(ss *Struct) error {
	_, ok := gen._types[ss.Name]
	if ok {
		return fmt.Errorf("struct %s already exists", ss.Name)
	}
	gen._types[ss.Name] = ss
	gen.elements = append(gen.elements, ss)
	return nil
}

func (gen *GeneratedFile) AddEnum(ee *Enum) error {
	_, ok := gen._enums[ee.Name]
	if ok {
		return fmt.Errorf("enum %s already exists", ee.Name)
	}
	gen._enums[ee.Name] = ee
	gen.elements = append(gen.elements, ee)
	return nil
}

type Field struct {
	Name     string
	DataType DataType
	Tags     map[string]string
	Property *schema_j5pb.ObjectProperty
}

type DataType struct {
	Name      string // string or GoIdent
	GoPackage string // Leave empty for no package
	J5Package string
	Pointer   bool
	TakeAddr  bool
	Slice     bool // Is []X
	Map       bool // Is map<string, X>
}

func (dt DataType) Addr() DataType {
	return DataType{
		Name:      dt.Name,
		GoPackage: dt.GoPackage,
		J5Package: dt.J5Package,
		Pointer:   false,
		TakeAddr:  true,
		Slice:     dt.Slice,
	}

}
func (dt DataType) AsSlice() DataType {
	return DataType{
		Name:      dt.Name,
		GoPackage: dt.GoPackage,
		J5Package: dt.J5Package,
		Pointer:   dt.Pointer,
		Slice:     true,
	}
}

func (dt DataType) Prefix() string {
	if dt.Slice {
		if dt.Pointer {
			return "[]*"
		} else {
			return "[]"
		}
	}

	if dt.Map {
		if dt.Pointer {
			return "map[string]*"
		} else {
			return "map[string]"
		}
	}

	if dt.Pointer {
		return "*"
	} else if dt.TakeAddr {
		return "&"
	}

	return ""
}

type Parameter struct {
	Name     string
	DataType DataType
}

type Function struct {
	Name       string
	TakesPtr   bool
	Parameters []*Parameter
	Returns    []*Parameter
	*StringGen
}

func (f *Function) PrintSignature(gen *StringGen) {
	parts := f.signature()
	gen.P(parts...)
}

func (f *Function) signature() []interface{} {
	parts := []interface{}{
		f.Name, "(",
	}
	for idx, parameter := range f.Parameters {
		if idx > 0 {
			parts = append(parts, ", ")
		}

		parts = append(parts, parameter.Name, " ", parameter.DataType)
	}
	parts = append(parts, ") (")
	for idx, parameter := range f.Returns {
		if idx > 0 {
			parts = append(parts, ", ")
		}
		parts = append(parts, parameter.DataType)
	}
	parts = append(parts, ")")
	return parts
}

func (f *Function) Print(gen *StringGen) {
	parts := f.signature()
	parts = append(parts, " {")
	gen.P(append([]interface{}{
		"func ",
	}, parts...)...)

	f.StringGen.buf.WriteTo(gen.buf) // nolint: errcheck //  as all writes to a bytes.Buffer succeed

	gen.P("}")
	gen.P()
}

func (f *Function) PrintAsMethod(gen *StringGen, methodOf string) {
	parts := f.signature()
	parts = append(parts, " {")

	if f.TakesPtr {
		methodOf = "*" + methodOf
	}
	gen.P(append([]interface{}{
		"func (s ", methodOf, ") ",
	}, parts...)...)

	f.StringGen.buf.WriteTo(gen.buf) // nolint: errcheck //  as all writes to a bytes.Buffer succeed

	gen.P("}")
	gen.P()
}

type Enum struct {
	Name    string
	Comment string
	Values  []*EnumValue
}

type EnumValue struct {
	Name    string
	Comment string
}

func (ee *Enum) Print(gen *StringGen) {
	gen.P("// ", ee.Name, " ", ee.Comment)
	gen.P("type ", ee.Name, " string")
	gen.P("const (")
	prefix := ee.Name + "_"
	for _, value := range ee.Values {
		gen.P("  ", prefix, value.Name, " ", ee.Name, " = ", quoteString(value.Name))
	}
	gen.P(")")
	gen.P()
}

type Struct struct {
	Name         string
	Comment      string
	Constructors []*Function
	Methods      []*Function
	Fields       []*Field
}

func tagString(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	tagStrings := make([]string, 0, len(tags))
	for tagName, tagValue := range tags {
		tagStrings = append(tagStrings, fmt.Sprintf("%s:\"%s\"", tagName, tagValue))
	}
	tagString := strings.Join(tagStrings, " ")
	return "`" + tagString + "`"
}

func (ss *Struct) Print(gen *StringGen) {

	gen.P("// ", ss.Name, " ", ss.Comment)
	gen.P("type ", ss.Name, " struct {")
	for _, field := range ss.Fields {
		tags := tagString(field.Tags)
		if field.Name == "" {
			gen.P("  ", field.DataType.Prefix(), field.DataType.Name, " ", tags)
		} else {
			gen.P("  ", field.Name, " ", field.DataType, " ", tags)
		}
	}
	gen.P("}")
	gen.P()

	for _, constructor := range ss.Constructors {
		constructor.Print(gen)
	}

	for _, method := range ss.Methods {
		method.PrintAsMethod(gen, ss.Name)
	}
}

type Interface struct {
	Name    string
	Methods []*Function
}

func (ii *Interface) Print(gen *StringGen) {
	gen.P("type ", ii.Name, " interface {")
	for _, method := range ii.Methods {
		method.PrintSignature(gen)
	}
	gen.P("}")
	gen.P()
}

func (g *GeneratedFile) ExportBytes() ([]byte, error) {

	fileGen := g.ChildGen()

	for _, ii := range g.elements {
		ii.Print(fileGen)
	}

	/*
		for _, ii := range g.interfaces {
			ii.Print(fileGen)
		}

		for _, ss := range g.services {
			ss.Print(fileGen)
		}

		for _, ee := range g.enums {
			ee.Print(fileGen)
		}

		for _, ss := range g.types {
			ss.Print(fileGen)
		}

		for _, ff := range g.funcs {
			ff.Print(fileGen)
		}*/

	bb := g.buf.Bytes()

	headerBytes := &bytes.Buffer{}
	headerBytes.WriteString("package " + g.packageName + "\n")
	headerBytes.WriteString("// Code generated by jsonapi. DO NOT EDIT.\n")
	headerBytes.WriteString("// Source: " + g.path + "\n")
	headerBytes.WriteString("\n")

	headerBytes.WriteString("import (\n")
	for importSrc, importName := range g.imports {
		fmt.Fprintf(headerBytes, "  %s \"%s\"\n", importName, importSrc)
	}
	headerBytes.WriteString(")\n")
	headerBytes.WriteString("\n")

	headerBytes.Write(fileGen.buf.Bytes())
	headerBytes.Write(bb)

	p, err := format.Source(headerBytes.Bytes())
	if err != nil {

		return headerBytes.Bytes(), nil

		//	return nil, err
	}

	return p, nil
}
