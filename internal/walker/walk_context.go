package walker

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/reflwrap"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/lib/j5reflect"
)

type Context interface {
	WithTypeSelect(path schema.PathSpec, givenName ast.Reference, fn SpanCallback) error

	SetScalar(path schema.PathSpec, value ast.Value) error
	FindAttribute(ref ast.Reference) (reflwrap.Field, error)

	// WithBlock begins a new context with nothing in the search path, for the
	// block found by the given name.
	WithBlock(ref ast.Reference, fn SpanCallback) error

	Logf(format string, args ...interface{})
	LogSelf(label string, args ...interface{})
	WrapErr(err error, pos errpos.Position) error
}

type SpanCallback func(Context, schema.BlockSpec) error

type walkContext struct {
	schema schema.Scope

	// name is the full path from the root to this context, as field names
	name []string
	// depth is the nested level of walk context. It may not equal len(name)
	// as depth skips blocks
	depth int
}

func Walk(obj j5reflect.Object, spec *schema.ConversionSpec, cb SpanCallback) error {

	err := spec.Validate()
	if err != nil {
		return err
	}

	scope, err := schema.NewRootSchemaWalker(spec, obj)
	if err != nil {
		return err
	}

	rootContext := &walkContext{
		schema: scope,
		name:   []string{""},
	}

	rootErr := rootContext.run(func(sc Context) error {
		return cb(sc, scope.CurrentBlock().Spec())
	})
	if rootErr == nil {
		return nil
	}
	logError(rootErr)
	return rootErr

}

func (sc *walkContext) FindAttribute(ref ast.Reference) (reflwrap.Field, error) {
	//sc.LogSelf("findAttribute(%#v)", ref)
	return sc.schema.FindAttribute(ref)
}

func (sc *walkContext) WithBlock(ref ast.Reference, fn SpanCallback) error {
	//sc.LogSelf("findBlock(%#v)", ref)
	container, err := sc.schema.FindBlock(ref)
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
func (sc *walkContext) WithTypeSelect(path schema.PathSpec, givenName ast.Reference, fn SpanCallback) error {

	scopeAtPath, err := sc.schema.NewScopeAtPath(path)
	if err != nil {
		return err
	}

	// the block should have a property with the name given by the user.
	blockForType, err := scopeAtPath.FindBlock(givenName)
	if err != nil {
		// this is not a schema error, as it's the passed in user value.
		err = errpos.AddPosition(err, givenName[0].Start)
		return err
	}

	// Continue walking, the scope of the blockForType is appended and the type
	// value is set.
	return sc.withSchema(blockForType, false, fn)
}

func (sc *walkContext) SetScalar(path schema.PathSpec, value ast.Value) error {
	sc.Logf("SetScalar(%#v, %#v)", path, value)
	leaf, err := sc.schema.FieldPathInLeaf(path)
	if err != nil {
		return fmt.Errorf("(Schema Error): %w", err)
	}
	return leaf.SetScalar(value)
}

func (wc *walkContext) run(fn func(Context) error) error {
	err := fn(wc)
	if err != nil {
		scoped := &scopedError{}
		if errors.As(err, &scoped) {
			return err
		}

		err = wc.WrapErr(err, errpos.Position{})
		err = errpos.AddContext(err, strings.Join(wc.name, "."))
		err = errpos.SetSchemas(err, wc.schema.SchemaNames())

		return &scopedError{
			err:    err.(*errpos.Err),
			schema: wc.schema,
		}
	}
	wc.depth--
	wc.Logf("|<<< Leaving %s <<<", wc.name)
	return nil
}

func (wc *walkContext) withSchema(container schema.Scope, freshScope bool, fn SpanCallback) error {
	wc.Logf("|>>> ENTERING >>>>")
	wc.Logf("|> %q", strings.Join(container.CurrentBlock().Path(), "."))
	childContext := &walkContext{
		schema: container,
		name:   append(wc.name, container.CurrentBlock().Path()...),
		depth:  wc.depth + 1,
	}
	wc.Logf("|> Path = %q", strings.Join(childContext.name, "."))

	return childContext.run(func(sc Context) error {
		return fn(sc, container.CurrentBlock().Spec())
	})
}

func (wc *walkContext) WrapErr(err error, pos errpos.Position) error {
	err = errpos.AddContext(err, strings.Join(wc.name, "."))
	err = errpos.SetSchemas(err, wc.schema.SchemaNames())
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
	err    *errpos.Err
	schema schema.Scope
}

func (se *scopedError) Error() string {
	return se.err.Error()
}

func (se *scopedError) Unwrap() error {
	return se.err
}

func logError(err error) {
	scoped := &scopedError{}
	if !errors.As(err, &scoped) {
		fmt.Printf("Error %s\n", err)
		return
	}
	pf := prefixer(log.Printf, "ERR | ")
	pf("Error %s", err)
	scoped.schema.PrintScope(pf)
	pf("Got Error %s\n", err)
}

func (wc *walkContext) LogSelf(label string, args ...interface{}) {
	wc.Logf(label, args...)
	wc.Logf("Context For %q", wc.name)
	wc.schema.PrintScope(wc.Logf)
}
