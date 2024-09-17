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
		ss := mapNested(ent.Source, nil, ent.Schema.Schemas)
		if err := ss.RangeNestedSchemas(visitor); err != nil {
			return err
		}
	}

	return nil
}

func (ent *entityNode) acceptKeys(visitor FileVisitor) error {

	object, err := newVirtualObjectNode(
		ent.Source.child("keys"),
		nil,
		ent.componentName("Keys"),
		ent.Schema.Keys,
	)

	object.Entity = &schema_j5pb.EntityObject{
		Entity: ent.name,
		Part:   schema_j5pb.EntityPart_KEYS,
	}

	if err != nil {
		return wrapErr(ent.Source, err)
	}

	if err := visitor.VisitObject(object); err != nil {
		return wrapErr(ent.Source, err)
	}
	return nil
}

func (ent *entityNode) acceptData(visitor FileVisitor) error {

	node, err := newVirtualObjectNode(
		ent.Source.child("data"),
		nil,
		ent.componentName("Data"),
		ent.Schema.Data,
	)
	if err != nil {
		return wrapErr(ent.Source, err)
	}

	node.Entity = &schema_j5pb.EntityObject{
		Entity: ent.name,
		Part:   schema_j5pb.EntityPart_DATA,
	}

	return visitor.VisitObject(node)
}

func (ent *entityNode) acceptStatus(visitor FileVisitor) error {
	entity := ent.Schema
	status := &schema_j5pb.Enum{
		Name:    ent.componentName("Status"),
		Options: entity.Status,
		Prefix:  strcase.ToScreamingSnake(entity.Name) + "_STATUS_",
	}

	node, err := newEnumNode(ent.Source.child("status"), nil, status)
	if err != nil {
		return wrapErr(ent.Source, err)
	}
	return visitor.VisitEnum(node)
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

	node, err := newObjectSchemaNode(ent.Source.child("state"), nil, state)
	if err != nil {
		return wrapErr(ent.Source, err)
	}
	return visitor.VisitObject(node)
}

func (ent *entityNode) acceptEventOneof(visitor FileVisitor) error {

	entity := ent.Schema
	eventOneof := &schema_j5pb.Oneof{
		Name:       strcase.ToCamel(entity.Name + "EventType"),
		Properties: make([]*schema_j5pb.ObjectProperty, 0, len(entity.Events)),
	}

	eventObjects := make([]*sourcedef_j5pb.NestedSchema, 0, len(entity.Events))

	for idx, eventObjectSchema := range entity.Events {

		nestedName := eventObjectSchema.Def.Name

		nested := &sourcedef_j5pb.NestedSchema{
			Type: &sourcedef_j5pb.NestedSchema_Object{
				Object: eventObjectSchema,
			},
		}

		eventObjects = append(eventObjects, nested)

		propSchema := &schema_j5pb.ObjectProperty{
			Name:       strcase.ToLowerCamel(eventObjectSchema.Def.Name),
			ProtoField: []int32{int32(idx + 1)},
			Schema: &schema_j5pb.Field{
				Type: &schema_j5pb.Field_Object{
					Object: &schema_j5pb.ObjectField{
						Schema: &schema_j5pb.ObjectField_Ref{
							Ref: &schema_j5pb.Ref{
								Package: "",
								Schema: fmt.Sprintf("%s.%s",
									eventOneof.Name,
									nestedName,
								),
							},
						},
					},
				},
			},
		}

		eventOneof.Properties = append(eventOneof.Properties, propSchema)
	}

	node, err := newOneofNode(ent.Source.child(virtualPathNode, "event_type"), nil, &sourcedef_j5pb.Oneof{
		Def:     eventOneof,
		Schemas: eventObjects,
	})

	if err != nil {
		return wrapErr(ent.Source, err)
	}

	return visitor.VisitOneof(node)
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

	node, err := newObjectSchemaNode(ent.Source.child("event"), nil, eventObject)
	if err != nil {
		return wrapErr(ent.Source, err)
	}
	return visitor.VisitObject(node)
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
