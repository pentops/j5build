package sourcewalk

type DefaultVisitor struct {
	File            func(*FileNode) error
	FileExit        func(*FileNode) error
	Property        func(*PropertyNode) error
	Object          func(*ObjectNode) error
	ObjectExit      func(*ObjectNode) error
	Oneof           func(*OneofNode) error
	OneofExit       func(*OneofNode) error
	Enum            func(*EnumNode) error
	TopicFile       func(*TopicFileNode) error
	Topic           func(*TopicNode) error
	TopicFileExit   func(*TopicFileNode) error
	ServiceFile     func(*ServiceFileNode) error
	Service         func(*ServiceNode) error
	ServiceFileExit func(*ServiceFileNode) error
}

func (df *DefaultVisitor) VisitProperty(node *PropertyNode) error {
	if df.Property != nil {
		return df.Property(node)
	}
	return nil
}

func (df *DefaultVisitor) VisitObject(node *ObjectNode) error {
	if err := node.RangeNestedSchemas(df); err != nil {
		return err
	}
	if err := node.RangeProperties(df); err != nil {
		return err
	}
	if df.Object != nil {
		return df.Object(node)
	}
	return nil
}

func (df *DefaultVisitor) VisitOneof(node *OneofNode) error {
	if df.Oneof != nil {
		if err := df.Oneof(node); err != nil {
			return err
		}
	}
	if err := node.RangeNestedSchemas(df); err != nil {
		return err
	}
	if err := node.RangeProperties(df); err != nil {
		return err
	}

	if df.OneofExit != nil {
		return df.OneofExit(node)
	}
	return nil
}

func (df *DefaultVisitor) VisitEnum(node *EnumNode) error {
	if df.Enum != nil {
		return df.Enum(node)
	}
	return nil
}

func (df *DefaultVisitor) VisitFile(file *FileNode) error {
	if df.File != nil {
		if err := df.File(file); err != nil {
			return err
		}
	}
	if err := file.RangeRootElements(df); err != nil {
		return err
	}
	if df.FileExit != nil {
		if err := df.FileExit(file); err != nil {
			return err
		}
	}
	return nil
}

func (df *DefaultVisitor) VisitTopicFile(node *TopicFileNode) error {
	if df.TopicFile != nil {
		if err := df.TopicFile(node); err != nil {
			return err
		}

	}
	if err := node.Accept(df); err != nil {
		return err
	}
	if df.TopicFileExit != nil {
		if err := df.TopicFileExit(node); err != nil {
			return err
		}
	}
	return nil
}

func (df *DefaultVisitor) VisitTopic(node *TopicNode) error {
	if df.Topic != nil {
		return df.Topic(node)
	}
	return nil
}

func (df *DefaultVisitor) VisitServiceFile(node *ServiceFileNode) error {
	if df.ServiceFile != nil {
		if err := df.ServiceFile(node); err != nil {
			return err
		}
	}
	if err := node.Accept(df); err != nil {
		return err
	}
	if df.ServiceFileExit != nil {
		if err := df.ServiceFileExit(node); err != nil {
			return err
		}
	}
	return nil
}

func (df *DefaultVisitor) VisitService(node *ServiceNode) error {
	if df.Service != nil {
		return df.Service(node)
	}
	return nil
}
