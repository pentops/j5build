package walker

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/reflwrap"
	"github.com/pentops/j5/lib/j5reflect"
)

type Context interface {
	WithTypeSelect(path PathSpec, givenName ast.Reference, fn SpanCallback) error

	SetScalar(path PathSpec, value ast.Value) error
	FindAttribute(ref ast.Reference) (reflwrap.Field, error)

	// WithBlock begins a new context with nothing in the search path, for the
	// block found by the given name.
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
	schema *schemaWalker

	// name is the full path from the root to this context, as field names
	name []string
	// depth is the nested level of walk context. It may not equal len(name)
	// as depth skips blocks
	depth int
}

func Walk(obj j5reflect.Object, spec *ConversionSpec, cb SpanCallback) error {

	err := spec.Validate()
	if err != nil {
		return err
	}

	walker, err := newRootSchemaWalker(spec, obj)
	if err != nil {
		return err
	}

	rootContext := &walkContext{
		schema: walker,
		name:   []string{"."},
	}

	rootErr := rootContext.run(func(sc Context) error {
		return cb(sc, *walker.leafBlock.spec)
	})
	if rootErr == nil {
		return nil
	}
	logError(rootErr)
	return rootErr

}

func (sc *walkContext) FindAttribute(ref ast.Reference) (reflwrap.Field, error) {
	sc.LogSelf("findAttribute(%#v)", ref)
	return sc.schema.findAttribute(ref)
}

func (sc *walkContext) WithBlock(ref ast.Reference, fn SpanCallback) error {
	sc.LogSelf("findBlock(%#v)", ref)
	container, err := sc.schema.findBlock(ref)
	if err != nil {
		return err
	}

	return sc.withSchema(container, true, fn)
}

// WithTypeSelect selects a child container from a wrapper container at path.
// It is intended to be used where exactly one option of the wrapper should be
// set, so the wrapper is not included in the callback scope.
// The node it finds at givenName should must be a block, which is appended to
// the scope and becomes the new leaf for the callback.
func (sc *walkContext) WithTypeSelect(path PathSpec, givenName ast.Reference, fn SpanCallback) error {

	containerAtPath, err := sc.schema.containerFromLeaf(path)
	if err != nil {
		return schemaError(path, err)
	}

	// new scope with just the container in it.
	scopeAtPath := sc.schema.newChild(containerAtPath, false)

	// the block should have a property with the name given by the user.
	blockForType, err := scopeAtPath.findBlock(givenName)
	if err != nil {
		// this is not a schema error, as it's the passed in user value.
		err = errpos.AddPosition(err, givenName[0].Start)
		return err
	}

	// Continue walking, the scope of the blockForType is appended and the type
	// value is set.
	return sc.withSchema(blockForType, false, fn)
}

func (sc *walkContext) SetScalar(path PathSpec, value ast.Value) error {
	sc.Logf("SetScalar(%#v, %#v)", path, value)
	leaf, err := sc.schema.fieldPathInLeaf(path)
	if err != nil {
		return fmt.Errorf("(Schema Error): %w", err)
	}
	return leaf.SetScalar(value)
}

func (wc *walkContext) run(fn func(Context) error) error {
	wc.Logf("|>>> ENTERING >>>>")
	wc.depth++
	wc.Logf("Path %s", strings.Join(wc.name, "."))
	err := fn(wc)
	if err != nil {

		scoped := &scopedError{}
		if errors.As(err, &scoped) {
			return err
		}

		return &scopedError{
			err:    err,
			schema: wc.schema,
		}
	}
	wc.depth--
	wc.LogSelf("|<<< Leaving %s <<<", wc.name)
	return nil
}

func (wc *walkContext) withSchema(container *containerField, freshScope bool, fn SpanCallback) error {
	childContext := &walkContext{
		schema: wc.schema.newChild(container, freshScope),
		name:   append(wc.name, container.path...),
		depth:  wc.depth,
	}
	return childContext.run(func(sc Context) error {
		return fn(sc, *container.spec)
	})
}

func (wc *walkContext) WrapErr(err error, pos errpos.Position) error {
	err = errpos.AddContext(err, strings.Join(wc.name, "."))
	err = errpos.SetSchemas(err, wc.schema.schemaNames())
	err = errpos.AddPosition(err, pos)
	return err
}

type logger func(format string, args ...interface{})

func prefixer(parent logger, prefix string) logger {
	return func(format string, args ...interface{}) {
		parent(prefix+format+"\n", args...)
	}
}

func (wc *walkContext) Logf(format string, args ...interface{}) {
	prefix := strings.Repeat("| ", wc.depth)
	prefixer(log.Printf, prefix)(format, args...)
}

type scopedError struct {
	err    error
	schema *schemaWalker
}

func (se *scopedError) Error() string {
	return se.err.Error()
}

func logError(err error) {
	scoped := &scopedError{}
	if !errors.As(err, &scoped) {
		fmt.Printf("Error %s\n", err)
		return
	}
	pf := prefixer(log.Printf, "ERR | ")
	pf("Error %s", err)
	scoped.schema.printScope(pf)
	pf("Got Error %s\n", err)
}

func (wc *walkContext) LogSelf(label string, args ...interface{}) {
	wc.Logf(label, args...)
	wc.Logf("Context For %q", wc.name)
	wc.schema.printScope(wc.Logf)
}
