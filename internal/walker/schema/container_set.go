package schema

import "fmt"

type containerSet []containerField

type Container interface {
	Path() []string
	Spec() BlockSpec
	Name() string
	RunCloseHooks() error
}

func (bs containerSet) schemaNames() []string {
	names := make([]string, 0, len(bs))
	for _, block := range bs {
		names = append(names, block.schemaName)
	}
	return names
}

func (bs containerSet) hasBlock(name string) bool {
	for _, blockSchema := range bs {
		if _, ok := blockSchema.spec.Children[name]; ok {
			return true
		}
	}
	return false
}

func (bs containerSet) listChildren() []string {

	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName := range blockSchema.spec.Children {
			possibleNames = append(possibleNames, blockName)
		}
	}

	if len(possibleNames) == 0 {
		return []string{"<no children>"}
	}

	return possibleNames
}
func (bs containerSet) listAttributes() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName, spec := range blockSchema.spec.Children {
			if spec.IsScalar {
				possibleNames = append(possibleNames, blockName)
			} else if spec.IsContainer {
				possibleNames = append(possibleNames, fmt.Sprintf("%s.", blockName))
			}
		}
	}

	return possibleNames
}

func (bs containerSet) listBlocks() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName, spec := range blockSchema.spec.Children {
			if !spec.IsContainer {
				continue
			}
			possibleNames = append(possibleNames, blockName)
		}
	}
	return possibleNames
}
