package j5convert

import (
	"fmt"

	"github.com/iancoleman/strcase"
	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

func (fb *FileBuilder) AddEntity(entity *sourcedef_j5pb.Entity) error {
	if entity.Keys == nil {
		return fmt.Errorf("missing keys")
	}
	if entity.Status == nil {
		return fmt.Errorf("missing status")
	}

	stateObj := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "State"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_STATE,
		},
	}

	keysObj := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Keys"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_KEYS,
		},
	}

	dataObj := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Data"),
		//	Entity: &schema_j5pb.EntityObject{
		//		Entity: entity.Name,
		//		Part:   schema_j5pb.EntityPart_DATA,
		//	},
	}

	dataMessage, err := buildMessage(fb, dataObj)
	if err != nil {
		return errpos.AddContext(err, "data")
	}

	keysMessage, err := buildMessage(fb, keysObj)
	if err != nil {
		return errpos.AddContext(err, "keys")
	}

	stateMessage, err := buildMessage(fb, stateObj)
	if err != nil {
		return errpos.AddContext(err, "state")
	}

	eventOneof, err := buildOneof(fb, &schema_j5pb.Oneof{
		Name: strcase.ToCamel(entity.Name + "EventType"),
	})
	if err != nil {
		return err
	}

	for _, event := range entity.Events {
		event.Def.Name = strcase.ToCamel(event.Def.Name)
		eventMsg, err := buildMessage(fb, event.Def)
		if err != nil {
			return errpos.AddContext(err, "event", event.Def.Name)
		}

		if err := eventOneof.addProperty(&schema_j5pb.ObjectProperty{
			Name:       strcase.ToSnake(event.Def.Name),
			ProtoField: []int32{int32(len(eventOneof.descriptor.OneofDecl) + 1)},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Object{
					Object: &schema_j5pb.ObjectField{
						Schema: &schema_j5pb.ObjectField_Ref{
							Ref: &schema_j5pb.Ref{
								Package: "",
								Schema:  event.Def.Name,
							},
						},
					},
				},
			},
		}); err != nil {
			return err
		}

		eventOneof.addMessage(eventMsg)

	}

	eventObject, err := buildMessage(fb, &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Event"),
		Entity: &schema_j5pb.EntityObject{
			Entity: entity.Name,
			Part:   schema_j5pb.EntityPart_EVENT,
		},
	})
	if err != nil {
		return err
	}

	err = eventObject.addProperty(&schema_j5pb.ObjectProperty{
		Name:       "type",
		ProtoField: []int32{1},
		Schema: &schema_j5pb.Field{
			Type: &schema_j5pb.Field_Oneof{
				Oneof: &schema_j5pb.OneofField{
					Schema: &schema_j5pb.OneofField_Ref{
						Ref: &schema_j5pb.Ref{
							Package: "",
							Schema:  eventOneof.descriptor.GetName(),
						},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	statusEnum := buildEnum(fb, strcase.ToCamel(entity.Name+"Status"), strcase.ToScreamingSnake(entity.Name)+"_STATUS_")
	for _, value := range entity.Status {
		statusEnum.addValue(value.Name, value.Number, value.Description)
	}

	fb.addMessage(keysMessage)
	fb.addMessage(stateMessage)
	fb.addEnum(statusEnum)
	fb.addMessage(dataMessage)
	fb.addMessage(eventObject)
	fb.addMessage(eventOneof)

	for _, msg := range entity.Schemas {
		switch st := msg.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Enum:
			if err := doEnum(fb, st.Enum); err != nil {
				return errpos.AddContext(err, "schema", st.Enum.Name)
			}
		case *sourcedef_j5pb.NestedSchema_Object:
			if err := doMessage(fb, st.Object.Def); err != nil {
				return errpos.AddContext(err, "schema", st.Object.Def.Name)
			}
		case *sourcedef_j5pb.NestedSchema_Oneof:
			if err := doOneof(fb, st.Oneof.Def); err != nil {
				return errpos.AddContext(err, "schema", st.Oneof.Def.Name)
			}
		default:
			return fmt.Errorf("AddEntity: Unknown %T", msg.Type)
		}
	}

	return nil
}
