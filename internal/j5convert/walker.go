package j5convert

import (
	"fmt"
	"log"

	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
)

type walkNode struct {
	path          []string
	alias         map[string][]string
	root          *FileBuilder
	parentContext parentContext
}

func (ww *walkNode) at(path ...string) *walkNode {
	if len(path) > 0 {
		if alias, ok := ww.alias[path[0]]; ok {
			path = append(alias, path[1:]...)
		}
	}

	_ = ww.root.source.getPos(append(ww.path, path...))

	return &walkNode{
		path:          append(ww.path, path...),
		parentContext: ww.parentContext,
		root:          ww.root,
	}
}

func (ww *walkNode) inMessage(msg *MessageBuilder) *walkNode {
	return &walkNode{
		path:          ww.path,
		parentContext: msg,
		root:          ww.root,
	}
}

func (ww *walkNode) withAlias(key string, value []string) *walkNode {
	if ww.alias == nil {
		ww.alias = map[string][]string{}
	}
	ww.alias[key] = value
	return ww
}

func (ww *walkNode) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	ww.error(err)
}

func (ww *walkNode) error(err error) {
	loc := ww.root.source.getPos(ww.path)
	if loc != nil {
		err = errpos.AddPosition(err, *loc)
	}
	log.Printf("error at %s: %v", ww.path, err)
	ww.root.errors = append(ww.root.errors, err)
}

func (ww *walkNode) addRoot(schema *sourcedef_j5pb.RootElement) {
	switch st := schema.Type.(type) {
	case *sourcedef_j5pb.RootElement_Object:
		if st.Object.Def == nil {
			ww.at("object").errorf("missing object definition")
		} else {
			ww.at("object", "def").doObject(st.Object.Def, st.Object.Schemas)
		}

	case *sourcedef_j5pb.RootElement_Enum:
		ww.at("enum").doEnum(st.Enum)

	case *sourcedef_j5pb.RootElement_Oneof:
		if st.Oneof.Def == nil {
			ww.at("oneof").errorf("missing oneof definition")
		}
		ww.at("oneof", "def").doOneof(st.Oneof.Def, st.Oneof.Schemas)

	case *sourcedef_j5pb.RootElement_Entity:
		ww.at("entity").doEntity(st.Entity)

	case *sourcedef_j5pb.RootElement_Partial:
		// Ignore, these are only used when included.
		return

	default:
		ww.errorf("unknown root element type %T", st)
	}

}

func (ww *walkNode) doObject(schema *schema_j5pb.Object, nested []*sourcedef_j5pb.NestedSchema) {
	message := blankMessage(ww.root, schema.Name)

	if schema.Entity != nil {
		ww.root.ensureImport(j5ExtImport)
		proto.SetExtension(message.descriptor.Options, ext_j5pb.E_Psm, &ext_j5pb.PSMOptions{
			EntityName: schema.Entity.Entity,
		})

	}
	message.comment([]int32{}, schema.Description)

	for idx, prop := range schema.Properties {
		ww.at("properties", fmt.Sprint(idx)).doProperty(message, prop)
	}

	ww.parentContext.addMessage(message)

	for idx, nestedSchema := range nested {
		ww := ww.at("schemas", fmt.Sprint(idx))
		switch st := nestedSchema.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			ww.at("object", "def").inMessage(message).doObject(st.Object.Def, st.Object.Schemas)
		case *sourcedef_j5pb.NestedSchema_Oneof:
			ww.at("oneof", "def").inMessage(message).doOneof(st.Oneof.Def, st.Oneof.Schemas)
		case *sourcedef_j5pb.NestedSchema_Enum:
			ww.at("enum", "def").inMessage(message).doEnum(st.Enum)
		default:
			ww.errorf("unknown schema type %T", st)
		}
	}
}

func (ww *walkNode) doOneof(schema *schema_j5pb.Oneof, nested []*sourcedef_j5pb.NestedSchema) {
	message := blankOneof(ww.root, schema.Name)
	message.comment([]int32{}, schema.Description)

	for idx, prop := range schema.Properties {
		prop.ProtoField = []int32{int32(idx + 1)}
		ww.at("properties", fmt.Sprint(idx)).doProperty(message, prop)
	}

	ww.parentContext.addMessage(message)

	for idx, nestedSchema := range nested {
		ww := ww.at("schemas", fmt.Sprint(idx))
		switch st := nestedSchema.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			ww.at("object", "def").inMessage(message).doObject(st.Object.Def, st.Object.Schemas)
		case *sourcedef_j5pb.NestedSchema_Oneof:
			ww.at("oneof", "def").inMessage(message).doOneof(st.Oneof.Def, st.Oneof.Schemas)
		case *sourcedef_j5pb.NestedSchema_Enum:
			ww.at("enum", "def").inMessage(message).doEnum(st.Enum)
		default:
			ww.errorf("unknown schema type %T", st)
		}
	}
}

func (ww *walkNode) doEnum(schema *schema_j5pb.Enum) {
	eb := buildEnum(ww.parentContext, schema)
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

	protoFieldName := strcase.ToSnake(prop.Name)
	desc.Name = ptr(protoFieldName)
	desc.JsonName = ptr(prop.Name)
	desc.Number = ptr(prop.ProtoField[0])
	msg.comment([]int32{2, *desc.Number}, prop.Description)

	msg.descriptor.Field = append(msg.descriptor.Field, desc)
}
