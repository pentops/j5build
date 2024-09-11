package j5convert

import (
	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"github.com/pentops/j5build/internal/sourcewalk"
	"google.golang.org/protobuf/proto"
)

func convertFile(ww *walkContext, src *sourcedef_j5pb.SourceFile) {
	file := sourcewalk.NewRoot(src)
	file.RangeRootElements(sourcewalk.FileCallbacks{
		SchemaCallbacks: sourcewalk.SchemaCallbacks{
			Object: func(on *sourcewalk.ObjectNode) {
				convertObject(ww, on)
			},
			Oneof: func(on *sourcewalk.OneofNode) {
				convertOneof(ww, on)
			},
			Enum: func(en *sourcewalk.EnumNode) {
				convertEnum(ww, en)
			},
		},
		TopicFile: func(tn *sourcewalk.TopicFileNode) {
			subWalk := ww.subPackageFile("topic")
			convertTopic(subWalk, tn)
		},
		ServiceFile: func(sn *sourcewalk.ServiceFileNode) {
			subWalk := ww.subPackageFile("service")
			convertService(subWalk, sn)
		},
	})
}

func convertObject(ww *walkContext, node *sourcewalk.ObjectNode) {
	schema := node.Schema
	if schema.Name == "" {
		if ww.field == nil {
			ww.errorf(node.Source, "missing object name")
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

	node.RangeProperties(&sourcewalk.PropertyCallbacks{
		SchemaVisitor: walkerSchemaVisitor(ww),
		Property: func(node *sourcewalk.PropertyNode) {

			propertyDesc, err := buildProperty(ww, node)
			if err != nil {
				ww.error(node.Source, err)
			}

			// Take the index (prior to append len == index), not the field number
			locPath := []int32{2, int32(len(message.descriptor.Field))}
			message.comment(locPath, schema.Description)
			message.descriptor.Field = append(message.descriptor.Field, propertyDesc)

		},
	})

	if node.HasNestedSchemas() {
		subContext := ww.inMessage(message)
		node.RangeNestedSchemas(walkerSchemaVisitor(subContext))
	}

	ww.parentContext.addMessage(message)
}

func convertOneof(ww *walkContext, node *sourcewalk.OneofNode) {
	schema := node.Schema
	if schema.Name == "" {
		if ww.field == nil {
			ww.errorf(node.Source, "missing object name")
			return
		}
		schema.Name = strcase.ToCamel(ww.field.name)
	}

	message := blankMessage(ww.file, schema.Name)
	message.comment([]int32{}, schema.Description)

	node.RangeProperties(&sourcewalk.PropertyCallbacks{
		SchemaVisitor: walkerSchemaVisitor(ww),
		Property: func(node *sourcewalk.PropertyNode) {
			schema := node.Schema
			schema.ProtoField = []int32{node.Number}

			propertyDesc, err := buildProperty(ww, node)
			if err != nil {
				ww.error(node.Source, err)
				return
			}
			propertyDesc.OneofIndex = ptr(int32(0))

			// Take the index (prior to append len == index), not the field number
			locPath := []int32{2, int32(len(message.descriptor.Field))}
			message.comment(locPath, schema.Description)
			message.descriptor.Field = append(message.descriptor.Field, propertyDesc)
		},
	})

	if node.HasNestedSchemas() {
		subContext := ww.inMessage(message)
		node.RangeNestedSchemas(walkerSchemaVisitor(subContext))
	}

	ww.parentContext.addMessage(message)
}

func convertEnum(ww *walkContext, node *sourcewalk.EnumNode) {
	eb := buildEnum(node.Schema)
	ww.parentContext.addEnum(eb)
}

func walkerSchemaVisitor(ww *walkContext) sourcewalk.SchemaVisitor {
	return &sourcewalk.SchemaCallbacks{
		Object: func(on *sourcewalk.ObjectNode) {
			convertObject(ww, on)
		},
		Oneof: func(on *sourcewalk.OneofNode) {
			convertOneof(ww, on)
		},
		Enum: func(en *sourcewalk.EnumNode) {
			convertEnum(ww, en)
		},
	}
}
