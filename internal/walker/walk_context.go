package walker

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/reflwrap"
	"github.com/pentops/j5/lib/j5reflect"
)

type Context interface {
	WithTagProperty(path PathSpec, givenName string, fn SpanCallback) error

	SetScalar(path PathSpec, value ast.Value) error
	FindAttribute(ref ast.Reference) (reflwrap.Field, error)
	WithBlock(ref ast.Reference, fn SpanCallback) error

	Logf(format string, args ...interface{})
	LogSelf(label string, args ...interface{})
	WrapErr(err error, pos errpos.Position) error
}

type SpanCallback func(Context, BlockSpec) error

type SchemaError struct {
	err  error
	path PathSpec
}

func (se *SchemaError) Error() string {
	return fmt.Sprintf("<Schema>: %s", se.err)
}

func schemaError(path PathSpec, err error) error {
	return &SchemaError{
		err:  err,
		path: path,
	}
}

type walkContext struct {
	schemaSet *SchemaSet

	schemaWalker
	blockSet  blockScope
	leafBlock *containerField
	parent    *walkContext

	name  string
	depth int
}

func Walk(obj j5reflect.Object, spec *ConversionSpec, cb SpanCallback) error {

	err := spec.Validate()
	if err != nil {
		return err
	}

	ss := &SchemaSet{
		givenSpecs:  spec.GlobalDefs,
		cachedSpecs: map[string]*BlockSpec{},
	}
	if ss.givenSpecs == nil {
		ss.givenSpecs = map[string]*BlockSpec{}
	}

	rootContainer := reflwrap.NewContainerField(obj)
	rootBlock, err := ss.blockSpec(rootContainer)
	if err != nil {
		return err
	}

	rootSchema, err := newContainerField(rootContainer, rootBlock)
	if err != nil {
		return err
	}

	rootContext := &walkContext{
		schemaSet: ss,
		name:      "root",
		depth:     0,
	}

	return rootContext.withSchema(rootSchema.rootName, rootSchema, cb)
}

func (sc *walkContext) FindAttribute(ref ast.Reference) (reflwrap.Field, error) {
	sc.LogSelf("FindAttribute(%#v)", ref)
	if len(ref) != 1 {
		return nil, fmt.Errorf("TODO: Namespace Tags %#v", ref)
	}

	name := ref[0]

	node, err := sc.blockSet.fieldForAttribute(name.String())
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (sc *walkContext) WithBlock(ref ast.Reference, fn SpanCallback) error {
	sc.LogSelf("findBlock(%#v)", ref)
	if len(ref) != 1 {
		return fmt.Errorf("TODO: Namespace Tags %#v", ref)
	}

	name := ref[0]

	node, err := sc.blockSet.containerForBlock(name.String())
	if err != nil {
		sc.Logf("Did not find block %#v", ref)
		return err
	}

	return sc.withFieldBlock(node, fn)
}

func (sc *walkContext) WithTagProperty(path PathSpec, givenName string, fn SpanCallback) error {

	atPath, err := walkPath(sc.leafBlock.container, path)
	if err != nil {
		return schemaError(path, err)
	}

	container, err := atPath.AsContainer()
	if err != nil {
		return err
	}

	propField, err := container.Property(givenName)
	if err != nil {
		return err
	}

	propContainer, err := propField.AsContainer()
	if err != nil {
		return err
	}

	return sc.withFieldBlock(propContainer, fn)
}

func (sc *walkContext) withFieldBlock(container reflwrap.ContainerField, fn SpanCallback) error {

	spec, err := sc.schemaSet.blockSpec(container)
	if err != nil {
		return err
	}

	blockContext, err := newContainerField(container, spec)
	if err != nil {
		return err
	}

	return sc.withSchema(blockContext.rootName, blockContext, fn)
}

func (sc *walkContext) SetScalar(path PathSpec, value ast.Value) error {
	sc.Logf("SetScalar(%#v, %#v)", path, value)
	leaf, err := walkPath(sc.leafBlock.container, path)
	if err != nil {
		return fmt.Errorf("(Schema Error): %w", err)
	}
	return leaf.SetScalar(value)
}

type markedError struct {
	err error
}

func (me *markedError) Error() string {
	return me.err.Error()
}

func (me *markedError) Unwrap() error {
	return me.err
}

func (wc *walkContext) run(fn func(Context) error) error {
	wc.LogSelf(">> ENTERING >>")
	err := fn(wc)
	if err != nil {
		_, ok := err.(*markedError)
		if ok {
			return err
		}
		wc.logError(err)
		err = &markedError{err: err}
		return err
	}
	wc.LogSelf("<< Leaving %s <<", wc.name)
	return nil
}

func (wc *walkContext) withSchema(name string, container *containerField, fn SpanCallback) error {
	child := wc.emptyChild(name)
	child.blockSet = append(wc.blockSet, *container)
	child.leafBlock = container
	return child.run(func(sc Context) error {
		return fn(sc, *container.spec)
	})
}

func (wc *walkContext) emptyChild(name string) *walkContext {
	return &walkContext{
		schemaSet: wc.schemaSet,
		parent:    wc,
		depth:     wc.depth + 1,
		name:      name,
	}
}

func (wc *walkContext) WrapErr(err error, pos errpos.Position) error {
	err = errpos.AddContext(err, wc.name)
	err = errpos.SetSchemas(err, wc.blockSet.SchemaNames())
	err = errpos.AddPosition(err, pos)
	return err
}

type logger func(format string, args ...interface{})

func prefixer(prefix string) logger {
	return func(format string, args ...interface{}) {
		fmt.Printf(prefix+format+"\n", args...)
	}
}

func (wc *walkContext) Logf(format string, args ...interface{}) {
	prefix := fmt.Sprintf("%s%s| ",
		strings.Repeat("| ", wc.depth),
		wc.name)

	prefixer(prefix)(format, args...)
}

func (wc *walkContext) logError(err error) {
	wc.Logf("ERR | Error %s", err)
	fmt.Printf("ERR | Context For %q\n", wc.name)
	wc.printContext(prefixer("ERR |> "))
	fmt.Printf("ERR | Got Error %s\n", err)
}

func (wc *walkContext) printContext(logf func(string, ...interface{})) {
	logf("  available blocks:")
	for _, block := range wc.blockSet {
		logf("  from %s : %s %q", block.rootName, block.spec.source, block.spec.DebugName)
		for name, block := range block.spec.Blocks {
			logf("   - block[%s] %s", name, block)
		}
	}

	if wc.leafBlock == nil {
		logf("  no leaf spec")
		return
	}

	spec := wc.leafBlock.spec
	logf("  leaf spec: %s", spec.errName())
	if spec.Name != nil {
		logf("   - tag[name]: %#v", spec.Name)
	}
	if spec.TypeSelect != nil {
		logf("   - tag[type]: %#v", spec.TypeSelect)
	}
}

func (wc *walkContext) LogSelf(label string, args ...interface{}) {
	wc.Logf(label, args...)
	wc.Logf("Context For %q", wc.name)
	wc.printContext(wc.Logf)
}
