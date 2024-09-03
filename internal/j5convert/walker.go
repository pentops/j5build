package j5convert

import (
	"fmt"
	"log"
	"path"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type rootContext interface {
	resolveTypeNoImport(pkg string, name string) (*TypeRef, error)
	addError(error)
	sourceFor(path []string) *sourcedef_j5pb.SourceLocation
	sourcePosition(path []string) *errpos.Position
	subPackageFile(string) fileContext
}

type Root struct {
	packageName string
	deps        Package
	source      sourceLink
	errors      []error

	importAliases map[string]string

	mainFile *FileBuilder
	files    []*FileBuilder
}

func newRoot(deps Package, file *FileBuilder, source *sourcedef_j5pb.SourceLocation) *Root {
	return &Root{
		packageName:   file.fdp.GetPackage(),
		deps:          deps,
		source:        sourceLink{root: source},
		importAliases: map[string]string{},
		mainFile:      file,
		files:         []*FileBuilder{file},
	}
}

var _ rootContext = &Root{}

func (r *Root) ensureImport(alias string) {
	r.mainFile.ensureImport(alias)
}

func (rr *Root) subPackageFile(subPackage string) fileContext {
	fullPackage := path.Join(rr.packageName, subPackage)

	for _, search := range rr.files {
		if search.fdp.GetPackage() == fullPackage {
			return search
		}
	}

	rootName := *rr.mainFile.fdp.Name
	baseName := path.Base(rootName)
	dirName := path.Dir(rootName)
	subName := path.Join(dirName, subPackage, baseName)
	found := newFileBuilder(subName)
	rr.files = append(rr.files, found)
	return found
}

func (rr *Root) addError(err error) {
	rr.errors = append(rr.errors, err)
}

func (rr *Root) sourceFor(path []string) *sourcedef_j5pb.SourceLocation {
	return rr.source.getSource(path)
}

func (rr *Root) sourcePosition(path []string) *errpos.Position {
	return rr.source.getPos(path)
}

type fileContext interface {
	parentContext
	ensureImport(string)
	addService(*ServiceBuilder)
}

type parentContext interface {
	addMessage(*MessageBuilder)
	addEnum(*EnumBuilder)
}

type walkNode struct {
	path          []string
	root          rootContext
	file          fileContext
	parentContext parentContext
}

func (ww *walkNode) at(path ...string) *walkNode {
	return &walkNode{
		path:          append(ww.path, path...),
		parentContext: ww.parentContext,
		root:          ww.root,
		file:          ww.file,
	}
}

func (ww *walkNode) inMessage(msg *MessageBuilder) *walkNode {
	return &walkNode{
		path:          ww.path,
		parentContext: msg,
		root:          ww.root,
		file:          ww.file,
	}
}

func (ww *walkNode) subPackageFile(subPackage string) *walkNode {
	file := ww.root.subPackageFile(subPackage)
	return &walkNode{
		path:          ww.path,
		parentContext: file,
		root:          ww.root,
		file:          file,
	}
}

func (ww *walkNode) resolveType(pkg string, name string) (*TypeRef, error) {
	typeRef, err := ww.root.resolveTypeNoImport(pkg, name)
	if err != nil {
		return nil, err
	}

	ww.file.ensureImport(typeRef.File)
	return typeRef, nil
}

func (ww *walkNode) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	ww.error(err)
}

func (ww *walkNode) error(err error) {
	loc := ww.root.sourcePosition(ww.path)
	if loc != nil {
		err = errpos.AddPosition(err, *loc)
	}
	log.Printf("error at %s: %v", ww.path, err)
	ww.root.addError(err)
}

func (ww *walkNode) rootFile(file *sourcedef_j5pb.SourceFile) {
	for idx, element := range file.Elements {
		ww.at("elements", fmt.Sprint(idx)).addRoot(element)
	}
}

func (ww *walkNode) addRoot(schema *sourcedef_j5pb.RootElement) {
	switch st := schema.Type.(type) {
	case *sourcedef_j5pb.RootElement_Object:
		if st.Object.Def == nil {
			ww.at("object").errorf("missing object definition")
		} else {
			ww.at("object", "def").doObject(st.Object.Def, nestMessages(st.Object.Schemas))
		}

	case *sourcedef_j5pb.RootElement_Enum:
		ww.at("enum").doEnum(st.Enum)

	case *sourcedef_j5pb.RootElement_Oneof:
		if st.Oneof.Def == nil {
			ww.at("oneof").errorf("missing oneof definition")
		}
		ww.at("oneof", "def").doOneof(st.Oneof.Def, nestMessages(st.Oneof.Schemas))

	case *sourcedef_j5pb.RootElement_Entity:
		ww.at("entity").doEntity(st.Entity)

	case *sourcedef_j5pb.RootElement_Partial:
		// Ignore, these are only used when included.
		return

	default:
		ww.errorf("unknown root element type %T", st)
	}

}

func nestMessages(nested []*sourcedef_j5pb.NestedSchema) func(*walkNode, *MessageBuilder) {
	return func(ww *walkNode, message *MessageBuilder) {
		ww = ww.inMessage(message)
		for idx, nestedSchema := range nested {
			ww := ww.at("schemas", fmt.Sprint(idx))
			switch st := nestedSchema.Type.(type) {
			case *sourcedef_j5pb.NestedSchema_Object:
				ww.at("object", "def").inMessage(message).doObject(st.Object.Def, nestMessages(st.Object.Schemas))
			case *sourcedef_j5pb.NestedSchema_Oneof:
				ww.at("oneof", "def").inMessage(message).doOneof(st.Oneof.Def, nestMessages(st.Oneof.Schemas))
			case *sourcedef_j5pb.NestedSchema_Enum:
				ww.at("enum", "def").inMessage(message).doEnum(st.Enum)
			default:
				ww.errorf("unknown schema type %T", st)
			}
		}
	}
}

func (ww *walkNode) doObject(schema *schema_j5pb.Object, mod ...func(*walkNode, *MessageBuilder)) {
	message := blankMessage(ww.file, schema.Name)

	if schema.Entity != nil {
		ww.file.ensureImport(j5ExtImport)
		proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Psm, &ext_j5pb.PSMOptions{
			EntityName: schema.Entity.Entity,
			EntityPart: schema.Entity.Part.Enum(),
		})

	}
	message.comment([]int32{}, schema.Description)

	for idx, prop := range schema.Properties {
		ww.inMessage(message).at("properties", fmt.Sprint(idx)).doProperty(message, prop)
	}

	for _, m := range mod {
		if m == nil {
			panic("nil mod")
		}
		m(ww, message)
	}

	ww.parentContext.addMessage(message)

}

func (ww *walkNode) doOneof(schema *schema_j5pb.Oneof, mod ...func(*walkNode, *MessageBuilder)) {
	message := blankOneof(ww.file, schema.Name)
	message.comment([]int32{}, schema.Description)
	message.descriptor.OneofDecl = []*descriptorpb.OneofDescriptorProto{{
		Name: ptr("type"),
	}}

	for idx, prop := range schema.Properties {
		prop.ProtoField = []int32{int32(idx + 1)}
		ww.inMessage(message).at("properties", fmt.Sprint(idx)).doProperty(message, prop)
	}

	for _, m := range mod {
		m(ww, message)
	}

	ww.parentContext.addMessage(message)
}

func (ww *walkNode) doEnum(schema *schema_j5pb.Enum) {
	eb := buildEnum(schema)
	ww.parentContext.addEnum(eb)
}

func (ww *walkNode) doProperty(msg *MessageBuilder, prop *schema_j5pb.ObjectProperty) {

	if len(prop.ProtoField) != 1 {
		ww.errorf("only supporting single proto field")
		return
	}

	if prop.Schema == nil {
		ww.errorf("missing schema/type")
		return
	}
	desc := ww.at("schema").buildField(prop.Schema)
	if desc == nil {
		return
	}
	if msg.isOneof {
		desc.OneofIndex = ptr(int32(0))
	}

	if prop.Required {
		ext := proto.GetExtension(desc.Options, validate.E_Field).(*validate.FieldConstraints)
		if ext == nil {
			ext = &validate.FieldConstraints{}
		}
		ext.Required = true
		proto.SetExtension(desc.Options, validate.E_Field, ext)
	}

	protoFieldName := strcase.ToSnake(prop.Name)
	desc.Name = ptr(protoFieldName)
	desc.JsonName = ptr(prop.Name)
	desc.Number = ptr(prop.ProtoField[0])

	// Take the index (prior to append len == index), not the field number
	locPath := []int32{2, int32(len(msg.descriptor.Field))}
	msg.comment(locPath, prop.Description)

	msg.descriptor.Field = append(msg.descriptor.Field, desc)
}

func (ww *walkNode) doService(spec *sourcedef_j5pb.Service) {
	serviceWalker := ww.subPackageFile("service")

	service := blankService(serviceWalker.file, spec.Name+"Service")
	service.basePath = spec.BasePath

	for idx, method := range spec.Methods {
		serviceWalker.at("methods", fmt.Sprint(idx)).doMethod(service, method)
	}

	serviceWalker.file.addService(service)
}

func (ww *walkNode) doMethod(service *ServiceBuilder, method *sourcedef_j5pb.Method) {
	methodBuilder := blankMethod(method.Name)
	methodBuilder.comment([]int32{}, method.Description)
	ww.file.ensureImport(googleApiAnnotationsImport)

	if method.Request == nil {
		ww.errorf("missing input")
		return
	}
	methodBuilder.desc.InputType = ptr(fmt.Sprintf("%sRequest", method.Name))
	request := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sRequest", method.Name),
		Properties: method.Request.Properties,
	}
	ww.at("request").doObject(request)

	if method.Response == nil {
		ww.file.ensureImport(googleApiHttpBodyImport)
		methodBuilder.desc.OutputType = ptr("google.api.HttpBody")
	} else {
		methodBuilder.desc.OutputType = ptr(fmt.Sprintf("%sResponse", method.Name))
		response := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sResponse", method.Name),
			Properties: method.Response.Properties,
		}
		ww.at("response").doObject(response)
	}

	annotation := &annotations.HttpRule{}
	reqPathParts := strings.Split(path.Join(service.basePath, method.HttpPath), "/")
	for idx, part := range reqPathParts {
		if strings.HasPrefix(part, ":") {
			var field *schema_j5pb.ObjectProperty
			for _, search := range request.Properties {
				if search.Name == part[1:] {
					field = search
					break
				}
			}
			if field == nil {
				ww.errorf("missing field %s in request", part[1:])
			}

			fieldName := strcase.ToSnake(part[1:])
			reqPathParts[idx] = "{" + fieldName + "}"

		}
	}

	reqPath := strings.Join(reqPathParts, "/")

	switch method.HttpMethod {
	case client_j5pb.HTTPMethod_GET:
		annotation.Pattern = &annotations.HttpRule_Get{
			Get: reqPath,
		}
	case client_j5pb.HTTPMethod_POST:
		annotation.Pattern = &annotations.HttpRule_Post{
			Post: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_DELETE:
		annotation.Pattern = &annotations.HttpRule_Delete{
			Delete: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_PATCH:
		annotation.Pattern = &annotations.HttpRule_Patch{
			Patch: reqPath,
		}
		annotation.Body = "*"

	case client_j5pb.HTTPMethod_PUT:
		annotation.Pattern = &annotations.HttpRule_Put{
			Put: reqPath,
		}
		annotation.Body = "*"

	default:
		ww.errorf("unsupported http method %s", method.HttpMethod)
		return
	}

	proto.SetExtension(methodBuilder.desc.Options, annotations.E_Http, annotation)

	service.desc.Method = append(service.desc.Method, methodBuilder.desc)
}
