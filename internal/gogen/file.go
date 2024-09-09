package gogen

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
)

func quoteString(s string) string {
	return fmt.Sprintf("%q", s)
}

type StringGen struct {
	imports   map[string]string
	buf       *bytes.Buffer
	myPackage string
}

func (g *StringGen) ImportPath(importSrc string) string {
	existing, ok := g.imports[importSrc]
	if ok {
		return existing
	}

	importName := g.findUnusedPackage(path.Base(importSrc))
	g.imports[importSrc] = importName
	return g.imports[importSrc]
}

func (g *StringGen) findUnusedPackage(want string) string {
	for _, used := range g.imports {
		if used == want {
			// TODO if the name is already a number, increment it
			return g.findUnusedPackage(want + "_1")
		}
	}

	return want
}

func (g *StringGen) ChildGen() *StringGen {
	return &StringGen{
		buf:       &bytes.Buffer{},
		imports:   g.imports,
		myPackage: g.myPackage,
	}
}

// P prints a line to the generated output. It converts each parameter to a
// string following the same rules as fmt.Print. It never inserts spaces
// between parameters.
func (g *StringGen) P(v ...interface{}) {
	for _, x := range v {
		if packaged, ok := x.(DataType); ok {
			if packaged.GoPackage == "" {
				fmt.Fprint(g.buf, packaged.Prefix(), packaged.Name)
				continue
			}

			specified := packaged.GoPackage
			if specified == g.myPackage {
				fmt.Fprint(g.buf, packaged.Prefix(), packaged.Name)
			} else {
				importedName := g.ImportPath(packaged.GoPackage)
				fmt.Fprint(g.buf, packaged.Prefix(), importedName, ".", packaged.Name)
			}
		} else {
			fmt.Fprint(g.buf, x)
		}
	}
	fmt.Fprintln(g.buf)
}

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

type GoIdent struct {
	Package string
	Name    string
}

func (gi GoIdent) PackageName() string {
	return gi.Package
}

func (gi GoIdent) Identity() string {
	return gi.Name
}

type GeneratedFile struct {
	path        string
	packageName string

	interfaces map[string]*Interface
	services   map[string]*Struct
	types      map[string]*Struct
	funcs      map[string]*Function

	*StringGen
}

func NewFile(importPath string, packageName string) *GeneratedFile {
	gen := &GeneratedFile{
		path:        importPath,
		packageName: packageName,
		services:    make(map[string]*Struct),
		types:       make(map[string]*Struct),
		funcs:       make(map[string]*Function),
		interfaces:  make(map[string]*Interface),
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
	if len(gen.services) == 0 {
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
	for _, service := range gen.services {
		combined.Fields = append(combined.Fields, &Field{
			//Name:     service.Name,
			DataType: DataType{Name: service.Name, Pointer: true},
		})
		constructor.P("  ", service.Name, ": New", service.Name, "(requester),")
	}
	constructor.P("  }")

	gen.types["CombinedClient"] = combined
}

func (gen *GeneratedFile) Service(serviceName string) *Struct {
	existing, ok := gen.services[serviceName]
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
	gen.services[serviceName] = service

	return service
}

func (gen *GeneratedFile) EnsureInterface(ii *Interface) {
	_, ok := gen.interfaces[ii.Name]
	if ok {
		return
	}
	// TODO: Compare Methods

	gen.interfaces[ii.Name] = ii
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
	Slice     bool
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

	ptr := ""
	if dt.Slice && dt.Pointer {
		ptr = "[]*"
	} else if dt.Slice {
		ptr = "[]"
	} else if dt.Pointer {
		ptr = "*"
	} else if dt.TakeAddr {
		ptr = "&"
	}
	return ptr
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

	header := g.ChildGen()

	for _, ii := range g.interfaces {
		ii.Print(header)
	}

	for _, ss := range g.services {
		ss.Print(header)
	}

	for _, ss := range g.types {
		ss.Print(header)
	}

	for _, ff := range g.funcs {
		ff.Print(header)
	}

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

	headerBytes.Write(header.buf.Bytes())
	headerBytes.Write(bb)

	p, err := format.Source(headerBytes.Bytes())
	if err != nil {

		return headerBytes.Bytes(), nil

		//	return nil, err
	}

	return p, nil
}
