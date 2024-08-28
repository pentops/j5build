package schema

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

func (bs containerSet) listAttributes() []string {
	possibleNames := make([]string, 0)
	for _, blockSchema := range bs {
		for blockName, spec := range blockSchema.spec.Children {
			if !spec.IsScalar {
				continue
			}
			possibleNames = append(possibleNames, blockName)
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
