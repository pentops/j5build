package j5convert

import (
	"log"
	"strconv"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func (ww *walkNode) doEntity(entity *sourcedef_j5pb.Entity) {
	ww.root.ensureImport(psmStateImport)

	converted := convertEntity(entity)
	ww.at("keys").withAlias("properties", []string{}).doObject(converted.keys, nil)
	ww.at("data").withAlias("properties", []string{}).doObject(converted.data, nil)
	ww.at("status").doEnum(converted.status)
	log.Printf("DoState")
	ww.doObject(converted.state, nil)
	log.Printf("DoEvent")
	ww.doOneof(converted.eventType.Def, converted.eventType.Schemas)
	log.Printf("DoEventObj")
	ww.doObject(converted.event, nil)

	for idx, msg := range entity.Schemas {
		ww := ww.at("schemas", strconv.Itoa(idx))
		switch st := msg.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Enum:
			ww.at("enum").doEnum(st.Enum)
		case *sourcedef_j5pb.NestedSchema_Object:
			ww.at("object", "def").doObject(st.Object.Def, st.Object.Schemas)
		case *sourcedef_j5pb.NestedSchema_Oneof:
			ww.at("oneof", "def").doOneof(st.Oneof.Def, st.Oneof.Schemas)
		default:
			ww.errorf("unknown schema type %T", st)
		}
	}
}

type entitySchemas struct {
	keys      *schema_j5pb.Object
	data      *schema_j5pb.Object
	status    *schema_j5pb.Enum
	state     *schema_j5pb.Object
	event     *schema_j5pb.Object
	eventType *sourcedef_j5pb.Oneof
}

func convertEntity(entity *sourcedef_j5pb.Entity) *entitySchemas {
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

	state := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "State"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_STATE,
		},
		Properties: []*schema_j5pb.ObjectProperty{{
			Name:       "metadata",
			ProtoField: []int32{1},
			Schema:     schemaRefField("j5.state.v1", "StateMetadata"),
		}, {
			Name:       "keys",
			ProtoField: []int32{2},
			Schema:     schemaRefField("", keys.Name),
		}, {
			Name:       "data",
			ProtoField: []int32{3},
			Schema:     schemaRefField("", data.Name),
		}, {
			Name:       "status",
			ProtoField: []int32{4},
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
			Schema:     schemaRefField("", event.Def.Name),
		})
	}

	eventObject := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Event"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_EVENT,
		},
		Properties: []*schema_j5pb.ObjectProperty{{
			Name:       "metadata",
			ProtoField: []int32{1},
			Schema:     schemaRefField("j5.state.v1", "EventMetadata"),
		}, {
			Name:       "keys",
			ProtoField: []int32{2},
			Schema:     schemaRefField("", keys.Name),
		}, {
			Name:       "type",
			ProtoField: []int32{1},
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

	return &entitySchemas{
		keys:      keys,
		data:      data,
		status:    status,
		state:     state,
		event:     eventObject,
		eventType: eventParent,
	}
}
