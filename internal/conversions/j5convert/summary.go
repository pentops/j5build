package j5convert

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/j5/gen/j5/schema/v1/schema_j5pb"
	"github.com/pentops/j5/gen/j5/sourcedef/v1/sourcedef_j5pb"
)

type FileSummary struct {
	Package          string
	Exports          map[string]*TypeRef
	FileDependencies []string
	TypeDependencies []*schema_j5pb.Ref
}

type PackageSummary struct {
	Exports map[string]*TypeRef
	Files   []*FileSummary
}

type TypeRef struct {
	Package  string
	Name     string
	File     string
	Position *errpos.Position

	// Oneof
	*EnumRef
	*MessageRef
}

// SourceSummary collects the exports and imports for a file
func SourceSummary(sourceFile *sourcedef_j5pb.SourceFile) (*FileSummary, error) {

	collector := &collector{
		source: sourceLink{
			root: sourceFile.SourceLocations,
		},
	}
	collector.file(sourceFile)
	if len(collector.errs) > 0 {
		joined := errors.Join(collector.errs...)
		return nil, joined
	}

	fs := &FileSummary{
		Package: sourceFile.Package.Name,
		Exports: make(map[string]*TypeRef),
	}

	importMap, err := j5Imports(sourceFile.Imports)
	if err != nil {
		return nil, err
	}

	for _, ref := range collector.refs {
		pkg := ref.ref.Package
		if pkg == "" {
			pkg = sourceFile.Package.Name
		}

		if pkg == sourceFile.Package.Name {
			fs.TypeDependencies = append(fs.TypeDependencies, &schema_j5pb.Ref{
				Package: pkg,
				Schema:  ref.ref.Schema,
			})
			continue
		}

		if implicit, ok := implicitImports[pkg]; ok {
			typeRef, ok := implicit.Exports[ref.ref.Schema]
			if ok {
				fs.TypeDependencies = append(fs.TypeDependencies, &schema_j5pb.Ref{
					Package: typeRef.Package,
					Schema:  typeRef.Name,
				})
				continue
			}
		}

		impPkg, ok := importMap[pkg]
		if ok {
			fs.TypeDependencies = append(fs.TypeDependencies, &schema_j5pb.Ref{
				Package: impPkg,
				Schema:  ref.ref.Schema,
			})
			continue
		}

		err := fmt.Errorf("package %s not imported", pkg)
		err = errpos.AddContext(err, strings.Join(ref.path, "."))
		loc := collector.source.getPos(ref.path)
		if loc != nil {
			err = errpos.AddPosition(err, *loc)
		}
		return nil, err

	}

	importPath := sourceFile.Path + ".proto"
	for _, export := range collector.exports {
		export.Package = sourceFile.Package.Name
		export.File = importPath
		fs.Exports[export.Name] = export
	}

	return fs, nil

}

func j5Imports(imps []*sourcedef_j5pb.Import) (map[string]string, error) {
	out := map[string]string{}
	errs := []error{}
	for _, imp := range imps {
		if strings.Contains(imp.Path, "/") {
			pkg := PackageFromFilename(imp.Path)
			out[pkg] = pkg
			continue
		}

		pkg := imp.Path
		if imp.Alias != "" {
			out[imp.Alias] = pkg
			continue
		}
		parts := strings.Split(pkg, ".")
		if len(parts) > 2 {
			errs = append(errs, fmt.Errorf("invalid package name in import %q", pkg))
			continue
		}
		withoutVersion := parts[len(parts)-2]
		out[withoutVersion] = pkg
		out[pkg] = pkg
	}
	return out, errors.Join(errs...)
}

type refWithSource struct {
	ref  *schema_j5pb.Ref
	path []string
}

type collector struct {
	refs    []*refWithSource
	exports []*TypeRef
	errs    []error
	source  sourceLink
}

func (c *collector) ref(path []string, node *schema_j5pb.Ref) {
	c.refs = append(c.refs, &refWithSource{
		ref:  node,
		path: path,
	})
}

func (c *collector) addErr(path []string, err error) {
	loc := c.source.getPos(path)
	if loc != nil {
		err = errpos.AddPosition(err, *loc)
	}
	c.errs = append(c.errs, err)
}

func (cc *collector) file(node *sourcedef_j5pb.SourceFile) {
	for idx, schema := range node.Elements {
		path := []string{"elements", strconv.Itoa(idx)}
		switch st := schema.Type.(type) {

		case *sourcedef_j5pb.RootElement_Object:
			path := append(path, "object")
			if st.Object.Def == nil {
				cc.addErr(path, fmt.Errorf("missing object definition"))
				continue
			}
			path = append(path, "def")
			cc.object(path, st.Object.Def, &objectCollect{
				nested: st.Object.Schemas,
			})

		case *sourcedef_j5pb.RootElement_Enum:
			path := append(path, "enum")
			cc.enum(path, st.Enum)

		case *sourcedef_j5pb.RootElement_Oneof:
			path := append(path, "oneof")
			if st.Oneof.Def == nil {
				cc.addErr(path, fmt.Errorf("missing oneof definition"))
			}
			path = append(path, "def")
			cc.oneof(path, st.Oneof.Def, &objectCollect{
				nested: st.Oneof.Schemas,
			})

		case *sourcedef_j5pb.RootElement_Entity:
			path := append(path, "entity")
			cc.entity(path, st.Entity)

		case *sourcedef_j5pb.RootElement_Topic:
			path := append(path, "topic")
			cc.topic(path, st.Topic)

		default:
			cc.addErr(path, fmt.Errorf("Collector, file: Unknown %T", schema.Type))
		}
	}
}

func (c *collector) prop(path []string, prop *schema_j5pb.ObjectProperty) {
	switch st := prop.Schema.Type.(type) {
	case *schema_j5pb.Field_Object:
		switch rt := st.Object.Schema.(type) {
		case *schema_j5pb.ObjectField_Ref:
			c.ref(append(path, "schema", "object", "ref"), rt.Ref)
		case *schema_j5pb.ObjectField_Object:
			c.object(append(path, "schema", "object", "object"), rt.Object, nil)
		}

	case *schema_j5pb.Field_Oneof:
		switch rt := st.Oneof.Schema.(type) {
		case *schema_j5pb.OneofField_Ref:
			c.ref(append(path, "schema", "oneof", "ref"), rt.Ref)
		case *schema_j5pb.OneofField_Oneof:
			c.oneof(append(path, "schema", "oneof", "oneof"), rt.Oneof, nil)
		}

	case *schema_j5pb.Field_Enum:
		switch rt := st.Enum.Schema.(type) {
		case *schema_j5pb.EnumField_Ref:
			c.ref(append(path, "schema", "enum", "ref"), rt.Ref)

		}
	}
}

type objectCollect struct {
	nameParent []string
	nested     []*sourcedef_j5pb.NestedSchema
}

func (c *collector) object(path []string, msg *schema_j5pb.Object, opts *objectCollect) {
	name := msg.Name
	if opts != nil && opts.nameParent != nil {
		name = strings.Join(append(opts.nameParent, msg.Name), ".")
	}

	c.exports = append(c.exports, &TypeRef{
		Name:       name,
		MessageRef: &MessageRef{},
		Position:   c.source.getPos(path),
	})

	for idx, prop := range msg.Properties {
		path := append(path, "properties", strconv.Itoa(idx))
		c.prop(path, prop)
	}
	if opts != nil && len(opts.nested) > 0 {
		c.nested(path, opts.nested, append(opts.nameParent, msg.Name))
	}
}

func (c *collector) oneof(path []string, msg *schema_j5pb.Oneof, opts *objectCollect) {
	name := msg.Name
	if opts != nil && opts.nameParent != nil {
		name = strings.Join(append(opts.nameParent, msg.Name), ".")
	}
	c.exports = append(c.exports, &TypeRef{
		Name:       name,
		MessageRef: &MessageRef{},
		Position:   c.source.getPos(path),
	})
	for idx, prop := range msg.Properties {
		path := append(path, "properties", strconv.Itoa(idx))
		c.prop(path, prop)
	}
	if opts != nil && len(opts.nested) > 0 {
		c.nested(path, opts.nested, append(opts.nameParent, msg.Name))
	}
}

func (c *collector) nested(path []string, nested []*sourcedef_j5pb.NestedSchema, nameParent []string) {
	for idx, nested := range nested {
		path := append(path, "nested", strconv.Itoa(idx))

		switch st := nested.Type.(type) {
		case *sourcedef_j5pb.NestedSchema_Object:
			if st.Object.Def == nil {
				c.addErr(path, fmt.Errorf("missing object definition"))
			}
			c.object(path, st.Object.Def, &objectCollect{
				nested:     st.Object.Schemas,
				nameParent: nameParent,
			})

		case *sourcedef_j5pb.NestedSchema_Oneof:
			if st.Oneof.Def == nil {
				c.addErr(path, fmt.Errorf("missing oneof definition"))
			}
			c.oneof(path, st.Oneof.Def, &objectCollect{
				nested:     st.Oneof.Schemas,
				nameParent: nameParent,
			})

		case *sourcedef_j5pb.NestedSchema_Enum:
			if st.Enum == nil {
				c.addErr(path, fmt.Errorf("missing enum definition"))
			}
			c.enum(path, st.Enum)
		}
	}
}

func (c *collector) enum(path []string, enum *schema_j5pb.Enum) {
	valMap := make(map[string]int32)
	for _, value := range enum.Options {
		valMap[enum.Prefix+value.Name] = value.Number
	}
	c.exports = append(c.exports, &TypeRef{
		Name: enum.Name,
		EnumRef: &EnumRef{
			Prefix: enum.Prefix,
			ValMap: valMap,
		},
		Position: c.source.getPos(path),
	})
}

func (c *collector) service(path []string, service *sourcedef_j5pb.Service) {
	for idx, method := range service.Methods {
		path := append(path, "methods", strconv.Itoa(idx))
		c.method(path, method)
	}
}

func (c *collector) method(path []string, method *sourcedef_j5pb.Method) {
	for idx, prop := range method.Request.Properties {
		path := append(path, "request", "properties", strconv.Itoa(idx))
		c.prop(path, prop)
	}

	if method.Response != nil {
		for idx, prop := range method.Response.Properties {
			path := append(path, "response", "properties", strconv.Itoa(idx))
			c.prop(path, prop)
		}
	}
}

func (c *collector) topic(path []string, topic *sourcedef_j5pb.Topic) {
	switch tt := topic.Type.Type.(type) {
	case *sourcedef_j5pb.TopicType_Publish_:
		for idx, msg := range tt.Publish.Messages {
			path := append(path, "type", "publish", "messages", strconv.Itoa(idx))
			for idx, prop := range msg.Fields {
				path := append(path, "fields", strconv.Itoa(idx))
				c.prop(path, prop)
			}
		}

	case *sourcedef_j5pb.TopicType_Reqres:
		for idx, msg := range tt.Reqres.Request.Fields {
			path := append(path, "type", "reqres", "request", "fields", strconv.Itoa(idx))
			c.prop(path, msg)
		}
		for idx, msg := range tt.Reqres.Reply.Fields {
			path := append(path, "type", "reqres", "reply", "fields", strconv.Itoa(idx))
			c.prop(path, msg)
		}
	}
}

func (c *collector) entity(path []string, entity *sourcedef_j5pb.Entity) {

	converted := convertEntity(entity)
	c.object(append(path, "_keys"), converted.keys, nil)
	c.object(append(path, "_data"), converted.data, nil)
	c.enum(append(path, "_status"), converted.status)
	c.object(append(path, "<virtual>"), converted.state, nil)
	c.oneof(append(path, "_eventType"), converted.eventType.Def, &objectCollect{
		nested: converted.eventType.Schemas,
	})
	c.object(append(path, "<virtual>"), converted.event, nil)

	c.service(append(path, "_query"), converted.query)
	for idx, command := range converted.commands {
		c.service(append(path, "commands", fmt.Sprintf("%d", idx)), command)
	}

	c.nested(path, entity.Schemas, nil)
}
