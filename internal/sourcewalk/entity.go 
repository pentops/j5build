package sourcewalk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5build/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type entityNode struct {
	name   string
	Source SourceNode
	Schema *sourcedef_j5pb.Entity
}

func (ent *entityNode) componentName(suffix string) string {
	return strcase.ToCamel(ent.Schema.Name) + strcase.ToCamel(suffix)
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

// run converts the entity into lower level schema elements and calls the
// visitors on those.
func (ent *entityNode) run(visitor FileVisitor) error {

	if err := ent.acceptKeys(visitor); err != nil {
		return err
	}
	if err := ent.acceptData(visitor); err != nil {
		return err
	}
	if err := ent.acceptStatus(visitor); err != nil {
		return err
	}

	if err := ent.acceptState(visitor); err != nil {
		return err
	}
	if err := ent.acceptEventOneof(visitor); err != nil {
		return err
	}
	if err := ent.acceptEvent(visitor); err != nil {
		return err
	}
	if err := ent.acceptQuery(visitor); err != nil {
		return err
	}
	if err := ent.acceptCommands(visitor); err != nil {
		return err
	}

	if len(ent.Schema.Schemas) > 0 {
		nested := mapNested(ent.Source, "schemas", []string{}, ent.Schema.Schemas)
		if err := rangeNestedSchemas(nested, visitor); err != nil {
			return wrapErr(ent.Source, err)
		}
	}

	return nil
}

func (ent *entityNode) acceptKeys(visitor FileVisitor) error {

	keys := &schema_j5pb.Object{
		Name: ent.componentName("Keys"),
		Entity: &schema_j5pb.EntityObject{
			Entity: ent.name,
			Part:   schema_j5pb.EntityPart_KEYS,
		},
		Properties: ent.Schema.Keys,
	}

	err := visitor.VisitObject(&ObjectNode{
		Schema: keys,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source, "keys", keys.Properties),
			Source:     ent.Source,
		},
	})
	if err != nil {
		return wrapErr(ent.Source, err)
	}
	return nil
}

func (ent *entityNode) acceptData(visitor FileVisitor) error {

	data := &schema_j5pb.Object{
		Name: ent.componentName("Data"),
		Entity: &schema_j5pb.EntityObject{
			Entity: ent.name,
			Part:   schema_j5pb.EntityPart_DATA,
		},
		Properties: ent.Schema.Data,
	}

	return visitor.VisitObject(&ObjectNode{
		Schema: data,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source, "data", data.Properties),
			Source:     ent.Source,
		},
	})

}

func (ent *entityNode) acceptStatus(visitor FileVisitor) error {
	entity := ent.Schema
	status := &schema_j5pb.Enum{
		Name:    ent.componentName("Status"),
		Options: entity.Status,
		Prefix:  strcase.ToScreamingSnake(entity.Name) + "_STATUS_",
	}
	return visitor.VisitEnum(&EnumNode{
		Schema: status,
		Source: ent.Source,
	})
}

func (ent *entityNode) innerRef(name string) *schema_j5pb.Field {
	return schemaRefField("", ent.componentName(name))
}

func (ent *entityNode) acceptState(visitor FileVisitor) error {
	entity := ent.Schema

	objKeys := schemaRefField("", ent.componentName("Keys"))
	objKeys.GetObject().Flatten = true

	state := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "State"),
		Entity: &schema_j5pb.EntityObject{
			Entity: ent.name,
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
			Schema:     ent.innerRef("Data"),
		}, {
			Name:       "status",
			ProtoField: []int32{4},
			Required:   true,
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Enum{
					Enum: &schema_j5pb.EnumField{
						Schema: &schema_j5pb.EnumField_Ref{
							Ref: &schema_j5pb.Ref{
								Schema: ent.componentName("Status"),
							},
						},
					},
				},
			},
		}},
	}

	return visitor.VisitObject(&ObjectNode{
		Schema: state,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.child(virtualPathNode), "state", state.Properties),
			Source:     ent.Source,
		},
	})
}

func (ent *entityNode) acceptEventOneof(visitor FileVisitor) error {

	entity := ent.Schema
	eventOneof := &schema_j5pb.Oneof{
		Name:       strcase.ToCamel(entity.Name + "EventType"),
		Properties: make([]*schema_j5pb.ObjectProperty, 0, len(entity.Events)),
	}

	eventObjectProperties := make([]*propertyNode, 0, len(entity.Events))
	listOfEventsSource := ent.Source.child("events")
	eventObjects := make([]*nestedNode, 0, len(entity.Events))

	for idx, eventObjectSchema := range entity.Events {

		// points to sourcedef.Object, which has def and schemas
		eventSource := listOfEventsSource.child(strconv.Itoa(idx))

		nested := &nestedNode{
			schema: &sourcedef_j5pb.NestedSchema_Object{
				Object: eventObjectSchema,
			},
			source:   eventSource, // Source of the object
			nestPath: []string{eventOneof.Name},
		}

		eventObjects = append(eventObjects, nested)

		propSchema := &schema_j5pb.ObjectProperty{
			Name:       strcase.ToCamel(eventObjectSchema.Def.Name),
			ProtoField: []int32{int32(idx + 1)},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Object{
					Object: &schema_j5pb.ObjectField{
						Schema: &schema_j5pb.ObjectField_Ref{
							Ref: &schema_j5pb.Ref{
								Package: "",
								Schema:  eventObjectSchema.Def.Name,
							},
						},
					},
				},
			},
		}
		property := &propertyNode{
			schema: propSchema,
			source: eventSource.child(virtualPathNode, "wrapper"),
			number: int32(idx + 1),
		}
		eventObjectProperties = append(eventObjectProperties, property)

	}

	return visitor.VisitOneof(&OneofNode{
		Schema: eventOneof,
		objectLikeNode: objectLikeNode{
			properties: eventObjectProperties,
			Source:     ent.Source.child(virtualPathNode, "event_type"),
			children:   eventObjects,
		},
	})
}

func (ent *entityNode) acceptEvent(visitor FileVisitor) error {
	entity := ent.Schema

	eventKeys := ent.innerRef("Keys")
	eventKeys.GetObject().Flatten = true

	eventObject := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Event"),
		Entity: &schema_j5pb.EntityObject{
			Entity: ent.name,
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
								Schema: ent.componentName("EventType"),
							},
						},
					},
				},
			},
		}},
	}

	return visitor.VisitObject(&ObjectNode{
		Schema: eventObject,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.child(virtualPathNode), "event", eventObject.Properties),
			Source:     ent.Source,
		},
	})

}

func (ent *entityNode) acceptCommands(visitor FileVisitor) error {
	entity := ent.Schema
	commands := entity.Commands
	for _, service := range commands {
		var serviceName string
		var servicePath string

		if service.Name != nil {
			serviceName = *service.Name
			if !strings.HasSuffix(serviceName, "Command") {
				serviceName += "Command"
			}
		} else {
			serviceName = fmt.Sprintf("%sCommand", strcase.ToCamel(ent.Schema.Name))
		}

		if service.BasePath != nil {
			servicePath = fmt.Sprintf("%s%s", entity.BaseUrlPath, *service.BasePath)
		} else {
			servicePath = fmt.Sprintf("/%s/c", entity.BaseUrlPath)
		}

		service.BasePath = &servicePath
		service.Name = &serviceName

		service.Options = &ext_j5pb.ServiceOptions{
			Type: &ext_j5pb.ServiceOptions_StateCommand_{
				StateCommand: &ext_j5pb.ServiceOptions_StateCommand{
					Entity: ent.name,
				},
			},
		}
	}

	services := make([]*serviceRef, 0)

	for idx, service := range commands {
		source := ent.Source.child("commands", strconv.Itoa(idx))
		services = append(services, &serviceRef{
			schema: service,
			source: source,
		})
	}

	return visitor.VisitServiceFile(&ServiceFileNode{
		services: services,
	})
}

func (ent *entityNode) acceptQuery(visitor FileVisitor) error {

	entity := ent.Schema
	name := ent.name

	primaryKeys := make([]*schema_j5pb.ObjectProperty, 0, len(ent.Schema.Keys))
	httpPath := []string{}
	for _, key := range ent.Schema.Keys {
		kk := key.Schema.GetKey()
		if kk == nil {
			continue
		}
		if kk.Entity != nil && kk.Entity.GetPrimaryKey() {
			primaryKeys = append(primaryKeys, key)
			httpPath = append(httpPath, fmt.Sprintf(":%s", key.Name))
		}
	}

	query := &sourcedef_j5pb.Service{
		BasePath: ptr(fmt.Sprintf("/%s/q", entity.BaseUrlPath)),
		Name:     ptr(fmt.Sprintf("%sQuery", strcase.ToCamel(name))),
		Methods: []*sourcedef_j5pb.Method{{
			Name: fmt.Sprintf("%sGet", strcase.ToCamel(name)),
			Request: &sourcedef_j5pb.AnonymousObject{
				Properties: primaryKeys,
			},

			Response: &sourcedef_j5pb.AnonymousObject{
				Properties: []*schema_j5pb.ObjectProperty{{
					Name:       strcase.ToLowerCamel(name),
					ProtoField: []int32{1},
					Schema:     ent.innerRef("State"),
					Required:   true,
				}},
			},
			HttpPath:   strings.Join(httpPath, "/"),
			HttpMethod: client_j5pb.HTTPMethod_GET,
			Options: &ext_j5pb.MethodOptions{
				StateQuery: &ext_j5pb.StateQueryMethodOptions{
					Get: true,
				},
			},
		}, {
			Name: fmt.Sprintf("%sList", strcase.ToCamel(name)),
			Request: &sourcedef_j5pb.AnonymousObject{
				Properties: []*schema_j5pb.ObjectProperty{{
					Name:       "page",
					ProtoField: []int32{100},
					Schema:     schemaRefField("j5.list.v1", "PageRequest"),
				}, {
					Name:       "query",
					ProtoField: []int32{101},
					Schema:     schemaRefField("j5.list.v1", "QueryRequest"),
				}},
			},
			Response: &sourcedef_j5pb.AnonymousObject{
				Properties: []*schema_j5pb.ObjectProperty{{
					Name:       strcase.ToLowerCamel(name),
					ProtoField: []int32{1},
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_Array{
							Array: &schema_j5pb.ArrayField{
								Items: ent.innerRef("State"),
							},
						},
					},
					Required: true,
				}, {
					Name:       "page",
					ProtoField: []int32{100},
					Schema:     schemaRefField("j5.list.v1", "PageResponse"),
				}},
			},
			HttpPath:   "",
			HttpMethod: client_j5pb.HTTPMethod_GET,
			Options: &ext_j5pb.MethodOptions{
				StateQuery: &ext_j5pb.StateQueryMethodOptions{
					List: true,
				},
			},
		}, {
			Name: fmt.Sprintf("%sEvents", strcase.ToCamel(name)),
			Request: &sourcedef_j5pb.AnonymousObject{
				Properties: append(primaryKeys, &schema_j5pb.ObjectProperty{
					Name:       "page",
					ProtoField: []int32{100},
					Schema:     schemaRefField("j5.list.v1", "PageRequest"),
				}, &schema_j5pb.ObjectProperty{
					Name:       "query",
					ProtoField: []int32{101},
					Schema:     schemaRefField("j5.list.v1", "QueryRequest"),
				}),
			},
			Response: &sourcedef_j5pb.AnonymousObject{
				Properties: []*schema_j5pb.ObjectProperty{{
					Name:       "events",
					ProtoField: []int32{1},
					Schema: &schema_j5pb.Field{
						Type: &schema_j5pb.Field_Array{
							Array: &schema_j5pb.ArrayField{
								Items: ent.innerRef("Event"),
							},
						},
					},
				}, {
					Name:       "page",
					ProtoField: []int32{100},
					Schema:     schemaRefField("j5.list.v1", "PageResponse"),
				}},
			},
			HttpPath:   strings.Join(append(httpPath, "events"), "/"),
			HttpMethod: client_j5pb.HTTPMethod_GET,
			Options: &ext_j5pb.MethodOptions{
				StateQuery: &ext_j5pb.StateQueryMethodOptions{
					ListEvents: true,
				},
			},
		}},
		Options: &ext_j5pb.ServiceOptions{
			Type: &ext_j5pb.ServiceOptions_StateQuery_{
				StateQuery: &ext_j5pb.ServiceOptions_StateQuery{
					Entity: name,
				},
			},
		},
	}

	return visitor.VisitServiceFile(&ServiceFileNode{
		services: []*serviceRef{{
			schema: query,
			source: ent.Source.child(virtualPathNode, "query"),
		}},
	})

}

func ptr[T any](v T) *T {
	return &v
}
