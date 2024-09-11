package sourcewalk

import "github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"

type PropertyVisitor interface {
	SchemaVisitor
	VisitProperty(*PropertyNode)
}

type PropertyCallbacks struct {
	SchemaVisitor
	Property func(*PropertyNode)
}

func (pc PropertyCallbacks) VisitProperty(pn *PropertyNode) {
	pc.Property(pn)
}

type PropertyNode struct {
	Schema *schema_j5pb.ObjectProperty
	Source SourceNode
	Number int32
	Field  FieldNode
}

type FieldNode struct {
	Source SourceNode
	Schema schema_j5pb.IsField_Type
}
