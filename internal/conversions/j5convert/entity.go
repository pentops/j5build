package j5convert

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pentops/j5/gen/j5/bcl/v1/bcl_j5pb"
	"github.com/pentops/j5/gen/j5/client/v1/client_j5pb"
	"github.com/pentops/j5/gen/j5/ext/v1/ext_j5pb"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
	"google.golang.org/protobuf/proto"
)

func (ww *walkNode) doEntity(src *sourcedef_j5pb.Entity) {
	ww.file.ensureImport(psmStateImport)

	entitySource := ww.root.sourceFor(ww.path)
	if entitySource != nil {
		mapEntitySource(entitySource)
	}
	converted := convertEntity(src)

	ww.at("_keys").doObject(converted.keys)

	ww.at("_data").doObject(converted.data)

	ww.at("_status").doEnum(converted.status)

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
	ww.at("query").doService(converted.query)
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
	query    *sourcedef_j5pb.Service
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

func mapEntitySource(root *bcl_j5pb.SourceLocation) {
	if _, ok := root.Children["_converted"]; ok {
		return
	}

	keys := root.Children["keys"]
	root.Children["_keys"] = &bcl_j5pb.SourceLocation{
		Children: map[string]*bcl_j5pb.SourceLocation{
			"properties": keys,
		},
	}
	root.Children["_data"] = &bcl_j5pb.SourceLocation{
		Children: map[string]*bcl_j5pb.SourceLocation{
			"properties": root.Children["data"],
		},
	}
	root.Children["_status"] = &bcl_j5pb.SourceLocation{
		Children: map[string]*bcl_j5pb.SourceLocation{
			"options": root.Children["status"],
		},
	}

	root.Children["_eventType"] = &bcl_j5pb.SourceLocation{
		Children: map[string]*bcl_j5pb.SourceLocation{
			"nested": root.Children["events"],
		},
	}

	root.Children["nested"] = root.Children["schemas"]

	root.Children["_converted"] = &bcl_j5pb.SourceLocation{}
}

func convertEntity(entity *sourcedef_j5pb.Entity) *entitySchemas {
	entity = proto.Clone(entity).(*sourcedef_j5pb.Entity)

	name := strcase.ToSnake(entity.Name)

	keys := &schema_j5pb.Object{
		Name: strcase.ToCamel(entity.Name + "Keys"),
		Entity: &schema_j5pb.EntityObject{
			Entity: name,
			Part:   schema_j5pb.EntityPart_KEYS,
		},
		Properties: entity.Keys,
	}

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

	/*
		message ListAccountEventsRequest {
		  string account_id = 1 [(buf.validate.field).string.uuid = true];

		  j5.list.v1.PageRequest page = 100;
		  j5.list.v1.QueryRequest query = 101;
		}

		message ListAccountEventsResponse {
		  repeated interxfi.registration.v1.AccountEvent events = 2 [(buf.validate.field).repeated = {
		    min_items: 1
		    max_items: 20
		  }];

		  j5.list.v1.PageResponse page = 100;
		}

		message ListAccountsRequest {
		  j5.list.v1.PageRequest page = 100;
		  j5.list.v1.QueryRequest query = 101;
		}

		message ListAccountsResponse {
		  repeated interxfi.registration.v1.AccountState accounts = 1 [(buf.validate.field).repeated = {
		    min_items: 1
		    max_items: 20
		  }];
		  j5.list.v1.PageResponse page = 100;
		}*/
	return &entitySchemas{
		name:      name,
		keys:      keys,
		data:      data,
		status:    status,
		state:     state,
		event:     eventObject,
		eventType: eventParent,
		commands:  commands,
		query:     query,
	}
}
