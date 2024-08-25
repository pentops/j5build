package walker

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker/schema"
	"github.com/pentops/j5/lib/j5reflect"
)

type ScopeFlag int

const (
	// Reset the scope to the new block
	ResetScope ScopeFlag = iota
	// Keep the current scope and add the new block
	KeepScope
)

type Context interface {
	SetAttribute(path schema.PathSpec, ref ast.Reference, value ast.Value) error
	WithBlock(path schema.PathSpec, ref ast.Reference, resetScope ScopeFlag, fn SpanCallback) error

	Logf(format string, args ...interface{})
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

func newSchemaError(err error) error {
	if err == nil {
		panic("newSchemaError called with nil error")

	}
	if fmt.Sprintf("%s", err) == "<nil>" {
		panic("newSchemaError called with nil error")
	}
	return fmt.Errorf("Schema Error): %w", err)
}

type pathElement struct {
	name     string
	position *errpos.Position
}

func combinePath(path schema.PathSpec, ref ast.Reference) []pathElement {
	pathToBlock := make([]pathElement, len(path)+len(ref))
	for i, ident := range path {
		pathToBlock[i] = pathElement{
			name: ident,
		}
	}

	for i, ident := range ref {
		start := ident.Start
		pathToBlock[i+len(path)] = pathElement{
			name:     ident.String(),
			position: &start,
		}
	}
	return pathToBlock
}

func (sc *walkContext) walkPath(path []pathElement) (schema.Scope, error) {
	container := sc.schema
	for _, ident := range path {
		next, err := container.ChildBlock(ident.name)
		if err != nil {
			if ident.position != nil {
				return nil, sc.WrapErr(err, *ident.position)
			} else {
				return nil, newSchemaError(err)
			}
		}
		container = next
	}
	return container, nil
}

func (sc *walkContext) WithBlock(path schema.PathSpec, ref ast.Reference, scopeFlag ScopeFlag, fn SpanCallback) error {
	sc.Logf("WithBlock(%#v, %#v)", path, ref)
	fullPath := combinePath(path, ref)
	if len(fullPath) == 0 {
		if scopeFlag == ResetScope {
			// call back with just the tail schema
			return sc.withSchema(sc.schema, ResetScope, fn)
		}

		return newSchemaError(fmt.Errorf("empty path for WithBlock and KeepScope"))
	}

	container, err := sc.walkPath(fullPath)
	if err != nil {
		return err
	}

	// Then call back with the schema of the end node in scope. Scope does not
	// get modified until the end
	return sc.withSchema(container, scopeFlag, fn)
}

type BadTypeError struct {
	WantType string
	GotType  string
}

func (bte BadTypeError) Error() string {
	return fmt.Sprintf("bad type: want %s, got %s", bte.WantType, bte.GotType)
}

func (sc *walkContext) SetAttribute(path schema.PathSpec, ref ast.Reference, val ast.Value) error {
	sc.Logf("SetAttribute(%#v, %#v, %#v)", path, ref, val)
	fullPath := combinePath(path, ref)
	if len(fullPath) == 0 {
		return newSchemaError(fmt.Errorf("empty path for SetAttribute"))
	}
	last := fullPath[len(fullPath)-1]
	pathToBlock := fullPath[:len(fullPath)-1]
	container, err := sc.walkPath(pathToBlock)
	if err != nil {
		return err
	}

	field, err := container.Field(last.name)
	if err != nil {
		if last.position != nil {
			return sc.WrapErr(err, *last.position)
		} else {
			return newSchemaError(err)
		}
	}

	err = field.SetASTValue(val)
	if err != nil {
		return sc.WrapErr(err, val.Start)
	}
	return nil
}

func (wc *walkContext) run(fn func(Context) error) error {
	err := fn(wc)
	if err != nil {
		// already scoped, pass it up the tree.
		scoped := &scopedError{}
		if errors.As(err, &scoped) {
			return err
		}

		wc.Logf("New Error %s", err)

		posErr, ok := errpos.AsError(err)
		if !ok {
			wc.Logf("Not errpos")
			posErr = &errpos.Err{
				Err: err,
			}
		}

		return &scopedError{
			err:    posErr,
			schema: wc.schema,
		}
	}
	return nil
}

func (wc *walkContext) withSchema(container schema.Scope, scopeFlag ScopeFlag, fn SpanCallback) error {
	lastBlock := container.CurrentBlock()

	pathName := strings.Join(lastBlock.Path(), ".")
	wc.Logf("|>>> Entering %q >>>", pathName)
	defer wc.Logf("|<<< Leaving %q <<<", pathName)
	prefix := strings.Repeat("| ", wc.depth) + "|> "
	entry := prefixer(log.Printf, prefix)

	newName := append(wc.name, lastBlock.Path()...)
	entry("Path = %q", strings.Join(newName, "."))

	newScope := container
	switch scopeFlag {
	case ResetScope:
		entry("Reset Scope")
	case KeepScope:
		entry("Keep Scope")
		newScope = wc.schema.MergeScope(container)
	}
	newScope.PrintScope(entry)

	childContext := &walkContext{
		schema: newScope,
		name:   newName,
		depth:  wc.depth + 1,
	}
	return childContext.run(func(sc Context) error {
		return fn(sc, container.CurrentBlock().Spec())
	})
}

func (wc *walkContext) WrapErr(err error, pos errpos.Position) error {
	//	err = errpos.AddContext(err, strings.Join(wc.name, "."))
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
	msg := scoped.err.Err.Error()
	pf("Error: %s", msg)
	pf("Scope:")
	scoped.schema.PrintScope(pf)
	pf("Got Error %s\n", msg)
}
