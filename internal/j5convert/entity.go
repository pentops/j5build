package j5convert

import (
	"fmt"
	"strconv"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func (ww *walkNode) doEntity(src *sourcedef_j5pb.Entity) {
	ww.file.ensureImport(psmStateImport)

	converted := convertEntity(src)

	entitySource := ww.root.sourceFor(ww.path)
	if entitySource != nil {
		keys := entitySource.Children["keys"]
		entitySource.Children["keys"] = &sourcedef_j5pb.SourceLocation{
			Children: map[string]*sourcedef_j5pb.SourceLocation{
				"properties": keys,
			},
		}
		// TODO: Test this and do for data etc
	}

	ww.at("keys").doObject(converted.keys)

	ww.at("data").doObject(converted.data)

	ww.at("status").doEnum(converted.status)

	ww.doObject(converted.state)
	ww.doOneof(converted.eventType.Def,
		nestMessages(converted.eventType.Schemas),
	)

	ww.doObject(converted.event)

	for idx, msg := range src.Schemas {
		ww := ww.at("schemas", strconv.Itoa(idx))
		switch st := msg.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Enum:
			ww.at("enum").doEnum(st.Enum)
		case *sourcedef_j5pb.NestedSchema_Object:
			ww.at("object", "def").doObject(st.Object.Def, nestMessages(st.Object.Schemas))
		case *sourcedef_j5pb.NestedSchema_Oneof:
			ww.at("oneof", "def").doOneof(st.Oneof.Def, nestMessages(st.Oneof.Schemas))
		default:
			ww.errorf("unknown schema type %T", st)
		}
	}

	for _, service := range converted.commands {
		ww.at("commands").doService(service)
	}
}

type entitySchemas struct {
	name      string
	keys      *schema_j5pb.Object
	data      *schema_j5pb.Object
	status    *schema_j5pb.Enum
	state     *schema_j5pb.Object
	event     *schema_j5pb.Object
	eventType *sourcedef_j5pb.Oneof

	commands []*sourcedef_j5pb.Service
}

func schemaRefField(pkg, desc string) *schema_j5pb.Field {
	return &schema_j5pb.Field{
		Type: &schema_j5pb.Field_Object{
			Object: &schema_j5pb.ObjectField{
				Schema: &schema_j5pb.ObjectField_Ref{
					Ref: &schema_j5pb.Ref{
						Package: pkg,
						Schema:  desc,
					},
				},
			},
		},
	}
}

func convertEntity(entity *sourcedef_j5pb.Entity) *entitySchemas {
	name := strcase.ToLowerCamel(entity.Name)
	keys := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Keys"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_KEYS,
		},
		Properties: entity.Keys,
	}

	data := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Data"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_DATA,
		},
		Properties: entity.Data,
	}

	status := &schema_j5pb.Enum{
		Name:    strcase.ToCamel(entity.Name + "Status"),
		Options: entity.Status,
		Prefix:  strcase.ToScreamingSnake(entity.Name) + "_STATUS_",
	}

	objKeys := schemaRefField("", keys.Name)
	objKeys.GetObject().Flatten = true
	state := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "State"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_STATE,
		},
		Properties: []*schema_j5pb.ObjectProperty{{
			Name:       "metadata",
			ProtoField: []int32{1},
			Required:   true,
			Schema:     schemaRefField("j5.state.v1", "StateMetadata"),
		}, {
			Name:       "keys",
			ProtoField: []int32{2},
			Required:   true,
			Schema:     objKeys,
		}, {
			Name:       "data",
			ProtoField: []int32{3},
			Required:   true,
			Schema:     schemaRefField("", data.Name),
		}, {
			Name:       "status",
			ProtoField: []int32{4},
			Required:   true,
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Enum{
					Enum: &schema_j5pb.EnumField{
						Schema: &schema_j5pb.EnumField_Ref{
							Ref: &schema_j5pb.Ref{
								Schema: status.Name,
							},
						},
					},
				},
			},
		}},
	}

	eventOneof := &schema_j5pb.Oneof{
		Name:       strcase.ToCamel(entity.Name + "EventType"),
		Properties: make([]*schema_j5pb.ObjectProperty, 0, len(entity.Events)),
	}

	eventParent := &sourcedef_j5pb.Oneof{
		Def: eventOneof,
	}

	for idx, event := range entity.Events {
		eventParent.Schemas = append(eventParent.Schemas, &sourcedef_j5pb.NestedSchema{
			Type: &sourcedef_j5pb.NestedSchema_Object{
				Object: event,
			},
		})

		eventOneof.Properties = append(eventOneof.Properties, &schema_j5pb.ObjectProperty{
			Name:       strcase.ToCamel(event.Def.Name),
			ProtoField: []int32{int32(idx + 1)},
			Schema:     schemaRefField("", eventOneof.Name+"."+event.Def.Name),
		})
	}

	eventKeys := schemaRefField("", keys.Name)
	eventKeys.GetObject().Flatten = true
	eventObject := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Event"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_EVENT,
		},
		Properties: []*schema_j5pb.ObjectProperty{{
			Name:       "metadata",
			ProtoField: []int32{1},
			Required:   true,
			Schema:     schemaRefField("j5.state.v1", "EventMetadata"),
		}, {
			Name:       "keys",
			ProtoField: []int32{2},
			Required:   true,
			Schema:     eventKeys,
		}, {
			Name:       "event",
			ProtoField: []int32{3},
			Required:   true,
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Oneof{
					Oneof: &schema_j5pb.OneofField{
						Schema: &schema_j5pb.OneofField_Ref{
							Ref: &schema_j5pb.Ref{
								Schema: eventOneof.Name,
							},
						},
					},
				},
			},
		}},
	}

	commands := entity.Commands
	for _, service := range commands {
		if service.Name == "" {
			service.Name = fmt.Sprintf("%sCommand", name)
		}
	}

	return &entitySchemas{
		name:      name,
		keys:      keys,
		data:      data,
		status:    status,
		state:     state,
		event:     eventObject,
		eventType: eventParent,
	}
}
