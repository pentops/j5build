package sourcewalk

import (
	"strconv"

	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type FileVisitor interface {
	SchemaVisitor
	VisitTopicFile(*TopicFileNode)
	VisitServiceFile(*ServiceFileNode)
}

type FileCallbacks struct {
	SchemaCallbacks
	TopicFile   func(*TopicFileNode)
	ServiceFile func(*ServiceFileNode)
}

func (fc FileCallbacks) VisitTopicFile(tfn *TopicFileNode) {
	fc.TopicFile(tfn)
}

func (fc FileCallbacks) VisitServiceFile(sfn *ServiceFileNode) {
	fc.ServiceFile(sfn)
}

var _ FileVisitor = FileCallbacks{}

type FileNode struct {
	*sourcedef_j5pb.SourceFile
	Source SourceNode
}

func (fn *FileNode) RangeRootElements(visitor FileVisitor) {
	for idx, element := range fn.SourceFile.Elements {
		source := fn.Source.child("elements", strconv.Itoa(idx))
		switch element := element.Type.(type) {
		case *sourcedef_j5pb.RootElement_Object:
			source := source.child("object")
			object := element.Object.Def
			objectNode := &ObjectNode{
				Schema: object,
				objectLikeNode: objectLikeNode{
					Source:     source.child("def"),
					children:   mapNested(source.maybeChild("schemas"), element.Object.Schemas),
					properties: mapProperties(source.maybeChild("def", "properties"), element.Object.Def.Properties),
				},
			}
			visitor.VisitObject(objectNode)

		case *sourcedef_j5pb.RootElement_Oneof:
			source := source.child("oneof")
			oneof := element.Oneof.Def
			oneofNode := &OneofNode{
				Schema: oneof,
				objectLikeNode: objectLikeNode{
					Source:     source.child("def"),
					children:   mapNested(source.maybeChild("schemas"), element.Oneof.Schemas),
					properties: mapProperties(source.maybeChild("def", "properties"), element.Oneof.Def.Properties),
				},
			}
			visitor.VisitOneof(oneofNode)

		case *sourcedef_j5pb.RootElement_Enum:
			enum := element.Enum
			enumNode := &EnumNode{
				Schema: enum,
				Source: source.child("enum"),
			}
			visitor.VisitEnum(enumNode)

		case *sourcedef_j5pb.RootElement_Entity:
			entity := element.Entity
			entityNode := &entityNode{
				Schema: entity,
				Source: source.child("entity"),
			}
			entityNode.run(visitor)
			// Entity is converted on-the-fly to root schemas, and uses the file
			// callbacks for the elements it creates.

		case *sourcedef_j5pb.RootElement_Topic:
			topic := element.Topic
			topicFileNode := &TopicFileNode{
				topics: []*topicRef{{
					schema: topic,
					source: source.child("topic"),
				}},
			}
			visitor.VisitTopicFile(topicFileNode)

		case *sourcedef_j5pb.RootElement_Service:
			service := element.Service
			serviceFileNode := &ServiceFileNode{
				services: []*serviceRef{{
					schema: service,
					source: source.child("service"),
				}},
			}
			visitor.VisitServiceFile(serviceFileNode)
		}
	}
}
