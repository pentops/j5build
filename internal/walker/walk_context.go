package walker

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/ast"
	"github.com/pentops/bcl.go/internal/walker/schema"
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
	WithContainer(loc *schema.SourceLocation, path schema.PathSpec, ref ast.Reference, resetScope ScopeFlag, fn SpanCallback) error
	SetLocation(loc schema.SourceLocation)
	SetName(name string)

	Logf(format string, args ...interface{})
	WrapErr(err error, pos schema.SourceLocation) error
}

type SpanCallback func(Context, schema.BlockSpec) error

type walkContext struct {
	schema schema.Scope

	// path is the full path from the root to this context, as field names
	path []string

	name string

	// depth is the nested level of walk context. It may not equal len(name)
	// as depth skips blocks
	depth         int
	blockLocation schema.SourceLocation

	verbose bool
}

func WalkSchema(scope schema.Scope, body ast.Body, verbose bool) error {

	rootContext := &walkContext{
		schema:  scope,
		path:    []string{""},
		verbose: verbose,
	}

	rootErr := rootContext.run(func(sc Context) error {
		return doBody(sc, body)
	})
	if rootErr == nil {
		return nil
	}
	if rootContext.verbose {
		logError(rootErr)
	}
	return rootErr

}

func newSchemaError(err error) error {
	return fmt.Errorf("Schema Error): %w", err)
}

type pathElement struct {
	name     string
	position *schema.SourceLocation
}

func combinePath(path schema.PathSpec, ref ast.Reference) []pathElement {
	pathToBlock := make([]pathElement, len(path)+len(ref))
	for i, ident := range path {
		pathToBlock[i] = pathElement{
			name: ident,
		}
	}

	for i, ident := range ref {
		pathToBlock[i+len(path)] = pathElement{
			name: ident.String(),
			position: &schema.SourceLocation{
				Start: ident.Start,
				End:   ident.End,
			},
		}
	}
	return pathToBlock
}

func (sc *walkContext) SetLocation(loc schema.SourceLocation) {
	sc.blockLocation = loc
}

func (sc *walkContext) walkPath(path []pathElement) (schema.Scope, error) {
	container := sc.schema
	for _, ident := range path {
		loc := sc.blockLocation
		if ident.position != nil {
			loc = *ident.position
		}

		next, err := container.ChildBlock(ident.name, loc)
		if err == nil { // INVERSION
			container = next
			continue
		}

		werr := &schema.WalkPathError{}
		if errors.As(err, &werr) {

			switch werr.Type {
			case schema.NodeNotFound:

				err = fmt.Errorf("Schema Error, no property in schema-defined path %q in %s.%s, want (%s)",
					strings.Join(werr.Path, "."),
					werr.Schema,
					werr.Field,
					strings.Join(werr.Available, ", "))

			default:
				err = fmt.Errorf("%s", werr.LongMessage())
			}

		}

		if ident.position != nil {
			return nil, sc.WrapErr(err, *ident.position)
		} else {
			return nil, err
		}
	}
	return container, nil
}

func (sc *walkContext) SetName(name string) {
	sc.name = name
	sc.path[len(sc.path)-1] = fmt.Sprintf("%s(%s)", sc.path[len(sc.path)-1], name)
}

func (sc *walkContext) WithContainer(newLoc *schema.SourceLocation, path schema.PathSpec, ref ast.Reference, scopeFlag ScopeFlag, fn SpanCallback) error {
	sc.Logf("WithContainer(%#v, %#v)", path, ref)
	fullPath := combinePath(path, ref)
	if len(fullPath) == 0 {
		if scopeFlag == ResetScope {
			// call back with just the tail schema
			return sc.withSchema(sc.schema, ResetScope, fn)
		}

		return newSchemaError(fmt.Errorf("empty path for WithContainer and KeepScope"))
	}

	if newLoc != nil {
		sc.SetLocation(*newLoc)
		sc.Logf("New Location %v", newLoc)
	} else {
		sc.Logf("Keep Location %v", sc.blockLocation)
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

	loc := schema.SourceLocation{
		Start: val.Start,
		End:   val.End,
	}

	field, err := container.Field(last.name, loc)
	if err != nil {
		if last.position != nil {
			return sc.WrapErr(err, *last.position)
		} else {
			return newSchemaError(err)
		}
	}

	err = field.SetASTValue(val)
	if err != nil {
		err = fmt.Errorf("SetAttribute %s: %w", field.FullTypeName(), err)
		return sc.WrapErr(err, loc)
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

	newPath := append(wc.path, lastBlock.Name())

	newScope := container
	switch scopeFlag {
	case ResetScope:
	case KeepScope:
		newScope = wc.schema.MergeScope(container)
	}

	if wc.verbose {
		wc.Logf("|>>> Entering %q >>>", lastBlock.Name())
		prefix := strings.Repeat("| ", wc.depth) + "|> "
		entry := prefixer(log.Printf, prefix)
		entry("Src = %q", strings.Join(lastBlock.Path(), "."))
		entry("Path = %q", strings.Join(newPath, "."))

		switch scopeFlag {
		case ResetScope:
			entry("ResetScope")
		case KeepScope:
			entry("KeepScope")
		}
		newScope.PrintScope(entry)
	}

	childContext := &walkContext{
		schema:        newScope,
		path:          newPath,
		depth:         wc.depth + 1,
		verbose:       wc.verbose,
		blockLocation: wc.blockLocation,
	}

	err := childContext.run(func(sc Context) error {
		return fn(sc, container.CurrentBlock().Spec())
	})
	if err != nil {
		return err
	}
	if wc.verbose {
		wc.Logf("|<<< Exiting %q <<<", lastBlock.Name())
	}
	if err := lastBlock.RunCloseHooks(); err != nil {
		return err
	}
	return nil
}

func (wc *walkContext) WrapErr(err error, pos schema.SourceLocation) error {
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
	if !wc.verbose {
		return
	}
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
	pf("Location: %s", scoped.err.Pos)
	pf("Scope:")
	scoped.schema.PrintScope(pf)
	pf("Got Error %s\n", msg)
}
