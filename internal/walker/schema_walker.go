package walker

type schemaWalker struct {
	currentScope blockScope
	leafBlock    *containerField
	schemaSet    *SchemaSet
}
