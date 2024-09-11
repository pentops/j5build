package sourcewalk

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
)

type entityNode struct {
	Source SourceNode
	Schema *sourcedef_j5pb.Entity
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
func (ent *entityNode) run(visitor FileVisitor) {

	entity := proto.Clone(ent.Schema).(*sourcedef_j5pb.Entity)

	name := strcase.ToSnake(entity.Name)

	keys := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Keys"),
		Entity: &schema_j5pb.EntityObject{
			Entity: name,
			Part:   schema_j5pb.EntityPart_KEYS,
		},
		Properties: entity.Keys,
	}

	visitor.VisitObject(&ObjectNode{
		Schema: keys,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.child("keys"), keys.Properties),
			Source:     ent.Source,
		},
	})

	data := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Data"),
		Entity: &schema_j5pb.EntityObject{
			Entity: name,
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
			Entity: name,
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

	visitor.VisitObject(&ObjectNode{
		Schema: state,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.maybeChild("_state"), state.Properties),
			Source:     ent.Source,
		},
	})

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
			Entity: name,
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
		var serviceName string
		var servicePath string

		if service.Name != nil {
			serviceName = *service.Name
			if !strings.HasSuffix(serviceName, "Command") {
				serviceName += "Command"
			}
		} else {
			serviceName = fmt.Sprintf("%sCommand", strcase.ToCamel(name))
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
					Entity: name,
				},
			},
		}
	}

	primaryKeys := make([]*schema_j5pb.ObjectProperty, 0, len(keys.Properties))
	httpPath := []string{}
	for _, key := range keys.Properties {
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
					Schema:     schemaRefField("", state.Name),
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
								Items: schemaRefField("", state.Name),
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
								Items: schemaRefField("", eventObject.Name),
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

	visitor.VisitObject(&ObjectNode{
		Schema: data,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.child("data"), data.Properties),
			Source:     ent.Source,
		},
	})

	visitor.VisitEnum(&EnumNode{
		Schema: status,
		Source: ent.Source,
	})

	visitor.VisitOneof(&OneofNode{
		Schema: eventOneof,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.maybeChild("_event_type"), eventOneof.Properties),
			Source:     ent.Source.maybeChild("_even_type"),
		},
	})

	visitor.VisitObject(&ObjectNode{
		Schema: eventObject,
		objectLikeNode: objectLikeNode{
			properties: mapProperties(ent.Source.maybeChild("_event"), eventObject.Properties),
			Source:     ent.Source,
		},
	})

	services := make([]*serviceRef, 0)

	for idx, service := range commands {
		source := ent.Source.child("commands", strconv.Itoa(idx))
		services = append(services, &serviceRef{
			schema: service,
			source: source,
		})
	}

	services = append(services, &serviceRef{
		schema: query,
		source: ent.Source.maybeChild("_query"),
	})

	visitor.VisitServiceFile(&ServiceFileNode{
		services: services,
	})

}

func ptr[T any](v T) *T {
	return &v
}
