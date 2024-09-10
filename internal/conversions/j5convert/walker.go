package j5convert

import (
	"fmt"
	"log"
	"path"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/messaging/v1/messaging_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type rootContext interface {
	resolveTypeNoImport(pkg string, name string) (*TypeRef, error)
	addError(error)
	sourceFor(path []string) *bcl_j5pb.SourceLocation
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

func newRoot(deps Package, file *FileBuilder, source *bcl_j5pb.SourceLocation) *Root {
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
	fullPackage := fmt.Sprintf("%s.%s", rr.packageName, subPackage)

	for _, search := range rr.files {
		if search.fdp.GetPackage() == fullPackage {
			return search
		}
	}

	rootName := *rr.mainFile.fdp.Name
	dirName, baseName := path.Split(rootName)

	baseRoot := strings.TrimSuffix(baseName, ".j5s.proto")
	newBase := fmt.Sprintf("%s.p.j5s.proto", baseRoot)

	subName := path.Join(dirName, subPackage, newBase)
	found := newFileBuilder(subName)

	found.fdp.Package = &fullPackage
	rr.files = append(rr.files, found)
	return found
}

func (rr *Root) addError(err error) {
	rr.errors = append(rr.errors, err)
}

func (rr *Root) sourceFor(path []string) *bcl_j5pb.SourceLocation {
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

type fieldContext struct {
	name string
}

type walkNode struct {
	path          []string
	root          rootContext
	file          fileContext
	field         *fieldContext
	parentContext parentContext
}

func (ww *walkNode) _clone() *walkNode {
	return &walkNode{
		path:          ww.path[:],
		root:          ww.root,
		file:          ww.file,
		field:         ww.field,
		parentContext: ww.parentContext,
	}
}

func (ww *walkNode) at(path ...string) *walkNode {
	walk := ww._clone()
	walk.path = append(ww.path, path...)
	return walk
}

func (ww *walkNode) inField(name string) *walkNode {
	walk := ww._clone()
	walk.field = &fieldContext{name: name}
	return walk
}

func (ww *walkNode) inMessage(msg *MessageBuilder) *walkNode {
	walk := ww._clone()
	walk.parentContext = msg
	walk.field = nil
	return walk
}

func (ww *walkNode) subPackageFile(subPackage string) *walkNode {
	file := ww.root.subPackageFile(subPackage)
	walk := ww._clone()
	walk.file = file
	walk.parentContext = file
	return walk
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
	log.Printf("walker error at %s: %v", strings.Join(ww.path, "."), err)
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

	case *sourcedef_j5pb.RootElement_Topic:
		ww.at("topic").doTopic(st.Topic)

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
	if schema.Name == "" {
		if ww.field == nil {
			ww.errorf("missing object name")
			return
		}
		schema.Name = strcase.ToCamel(ww.field.name)
	}

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
		prop.ProtoField = []int32{int32(idx + 1)}
		ww.
			inMessage(message).
			at("properties", fmt.Sprint(idx)).
			inField(prop.Name).
			doProperty(message, prop)
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
		ww.
			inMessage(message).
			at("properties", fmt.Sprint(idx)).
			inField(prop.Name).
			doProperty(message, prop)
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

	if ext := proto.GetExtension(desc.Options, ext_j5pb.E_Key).(*ext_j5pb.PSMKeyFieldOptions); ext != nil {
		if ext.PrimaryKey {
			// even if not explicitly set, a primary key is required, we son't support partial primary keys.
			prop.Required = true
		}
	}

	if prop.Required {
		ext := proto.GetExtension(desc.Options, validate.E_Field).(*validate.FieldConstraints)
		if ext == nil {
			ext = &validate.FieldConstraints{}
		}
		ww.file.ensureImport(bufValidateImport)
		ext.Required = true
		proto.SetExtension(desc.Options, validate.E_Field, ext)
		ww.file.ensureImport(j5ExtImport)
	}

	if prop.ExplicitlyOptional {
		if prop.Required {
			ww.errorf("cannot be both required and optional")
		}
		desc.Proto3Optional = ptr(true)
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

func (ww *walkNode) doTopic(schema *sourcedef_j5pb.Topic) {
	subWalk := ww.subPackageFile("topic")

	switch tt := schema.Type.Type.(type) {
	case *sourcedef_j5pb.TopicType_Publish_:
		subWalk.at("type", "publish").doPublishTopic(schema.Name, tt.Publish)
	case *sourcedef_j5pb.TopicType_Reqres:
		subWalk.at("type", "reqres").doReqResTopic(schema.Name, tt.Reqres)
	case *sourcedef_j5pb.TopicType_Upsert_:
		subWalk.at("type", "upsert").doUpsertTopic(schema.Name, tt.Upsert)
	default:
		ww.errorf("unknown topic type %T", tt)
	}
}

func (ww *walkNode) doPublishTopic(name string, schema *sourcedef_j5pb.TopicType_Publish) {
	name = strcase.ToCamel(name)
	desc := &descriptorpb.ServiceDescriptorProto{
		Name:    ptr(name + "Topic"),
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(desc.Options, messaging_j5pb.E_Config, &messaging_j5pb.Config{
		Type: &messaging_j5pb.Config_Broadcast{
			Broadcast: &messaging_j5pb.BroadcastConfig{
				Name: strcase.ToSnake(name),
			},
		},
	})

	for idx, msg := range schema.Messages {
		objSchema := &schema_j5pb.Object{
			Name:       fmt.Sprintf("%sMessage", msg.Name),
			Properties: msg.Fields,
		}
		ww.at("message", fmt.Sprint(idx)).doObject(objSchema)
		rpcDesc := &descriptorpb.MethodDescriptorProto{
			Name:       ptr(msg.Name),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(objSchema.Name),
		}
		desc.Method = append(desc.Method, rpcDesc)
	}

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: desc,
	})
}

func (ww *walkNode) doReqResTopic(name string, schema *sourcedef_j5pb.TopicType_ReqRes) {
	addRequest := func(ww *walkNode, msg *MessageBuilder) {
		msg.descriptor.Field = append(msg.descriptor.Field, &descriptorpb.FieldDescriptorProto{
			Name:     ptr("request"),
			JsonName: ptr("request"),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: ptr(".j5.messaging.v1.RequestMetadata"),
			Number:   ptr(int32(len(msg.descriptor.Field) + 1)),
		})
	}

	name = strcase.ToCamel(name)
	reqObj := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sRequestMessage", name),
		Properties: schema.Request.Fields,
	}
	ww.at("request").doObject(reqObj, addRequest)

	resObj := &schema_j5pb.Object{
		Name:       fmt.Sprintf("%sReplyMessage", name),
		Properties: schema.Reply.Fields,
	}
	ww.at("reply").doObject(resObj, addRequest)

	reqDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "RequestTopic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name + "Request"),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(reqObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(reqDesc.Options, messaging_j5pb.E_Config, &messaging_j5pb.Config{
		Type: &messaging_j5pb.Config_Request{
			Request: &messaging_j5pb.RequestConfig{
				Name: strcase.ToSnake(name),
			},
		},
	})

	resDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "ReplyTopic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name + "Reply"),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(resObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(resDesc.Options, messaging_j5pb.E_Config, &messaging_j5pb.Config{
		Type: &messaging_j5pb.Config_Reply{
			Reply: &messaging_j5pb.ReplyConfig{
				Name: strcase.ToSnake(name),
			},
		},
	})

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(messagingReqResImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: reqDesc,
	})
	ww.file.addService(&ServiceBuilder{
		desc: resDesc,
	})

}
func (ww *walkNode) doUpsertTopic(name string, schema *sourcedef_j5pb.TopicType_Upsert) {

	name = strcase.ToCamel(name)
	reqObj := &schema_j5pb.Object{
		Name: fmt.Sprintf("%sMessage", name),
		Properties: append([]*schema_j5pb.ObjectProperty{{
			Name:   "upsert",
			Schema: schemaRefField("j5.messaging.v1", "UpsertMetadata"),
		}}, schema.Message.Fields...),
	}
	ww.at("message").doObject(reqObj)

	reqDesc := &descriptorpb.ServiceDescriptorProto{
		Name: ptr(name + "Topic"),
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:       ptr(name),
			OutputType: ptr(googleProtoEmptyType),
			InputType:  ptr(reqObj.Name),
		}},
		Options: &descriptorpb.ServiceOptions{},
	}

	proto.SetExtension(reqDesc.Options, messaging_j5pb.E_Config, &messaging_j5pb.Config{
		Type: &messaging_j5pb.Config_Broadcast{
			Broadcast: &messaging_j5pb.BroadcastConfig{
				Name: strcase.ToSnake(name),
			},
		},
	})

	ww.file.ensureImport(messagingAnnotationsImport)
	ww.file.ensureImport(messagingUpsertImport)
	ww.file.ensureImport(googleProtoEmptyImport)
	ww.file.addService(&ServiceBuilder{
		desc: reqDesc,
	})

}
