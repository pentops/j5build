package walker

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/pentops/j5build/internal/bcl/errpos"
	"github.com/pentops/j5build/internal/bcl/internal/parser"
	"github.com/pentops/j5build/internal/bcl/internal/walker/schema"
)

type ScopeFlag int

const (
	// Reset the scope to the new block
	ResetScope ScopeFlag = iota
	// Keep the current scope and add the new block
	KeepScope
)

func (sf ScopeFlag) String() string {
	switch sf {
	case ResetScope:
		return "ResetScope"
	case KeepScope:
		return "KeepScope"
	default:
		return "Unknown"
	}
}

type Context interface {
	BuildScope(schemaPath schema.PathSpec, userPath []parser.Ident, flag ScopeFlag) (*schema.Scope, error)
	WithScope(newScope *schema.Scope, fn SpanCallback) error

	SetDescription(desc parser.ASTValue) error
	SetAttribute(path schema.PathSpec, ref []parser.Ident, value parser.ASTValue) error
	AppendAttribute(path schema.PathSpec, ref []parser.Ident, value parser.ASTValue) error

	setContainerFromScalar(bs schema.BlockSpec, vals parser.ASTValue) error

	Logf(format string, args ...interface{})
	WrapErr(err error, pos HasPosition) error
}

type SpanCallback func(Context, schema.BlockSpec) error

type walkContext struct {
	scope *schema.Scope

	// path is the full path from the root to this context, as field names
	path []string

	// depth is the nested level of walk context. It may not equal len(name)
	// as depth skips blocks
	depth         int
	blockLocation schema.SourceLocation

	verbose bool
}

func newSchemaError(err error) error {
	return fmt.Errorf("Schema Error): %w", err)
}

type pathElement struct {
	name     string
	position *schema.SourceLocation
}

func combinePath(path schema.PathSpec, ref []parser.Ident) []pathElement {
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

func (sc *walkContext) walkScopePath(path []pathElement) (*schema.Scope, error) {
	return walkScope(sc.scope, path, sc.blockLocation)
}

func walkScope(scope *schema.Scope, path []pathElement, loc schema.SourceLocation) (*schema.Scope, error) {
	for _, ident := range path {
		if ident.position != nil {
			loc = *ident.position
		}

		next, werr := scope.ChildBlock(ident.name, loc)
		if werr == nil { // INVERSION
			scope = next
			continue
		}

		if ident.position == nil {
			return nil, newSchemaError(werr)
		}

		var err error
		switch werr.Type {
		case schema.RootNotFound:
			blocks := scope.SchemaNames()
			if len(blocks) == 1 {
				err = fmt.Errorf("root type %q has no field %s - expecting %q",
					blocks[0],
					werr.Field,
					werr.Available) // ", "))
			} else if len(blocks) > 1 {
				err = fmt.Errorf("no field %q in any of %q - expecting %q",
					werr.Field,
					blocks,
					werr.Available)
			}
		case schema.NodeNotFound:
			err = fmt.Errorf("type %q has no field %q - expecting %q",
				werr.Schema,
				werr.Field,
				werr.Available) //strings.Join(werr.Available, ", "))

		default:
			err = fmt.Errorf("%s", werr.LongMessage())
		}

		err = errpos.AddPosition(err, *ident.position)
		return nil, err
	}
	return scope, nil
}

func (sc *walkContext) BuildScope(schemaPath schema.PathSpec, userPath []parser.Ident, flag ScopeFlag) (*schema.Scope, error) {
	fullPath := combinePath(schemaPath, userPath)
	if len(fullPath) == 0 {
		if flag == KeepScope {
			return sc.scope, nil
		}
		return sc.scope.TailScope(), nil
	}

	container, err := sc.walkScopePath(fullPath)
	if err != nil {
		return nil, err
	}

	switch flag {
	case ResetScope:
		return container, nil
	case KeepScope:
		return sc.scope.MergeScope(container), nil
	default:
		return nil, newSchemaError(fmt.Errorf("unknown flag %d", flag))
	}
}

type BadTypeError struct {
	WantType string
	GotType  string
}

func (bte BadTypeError) Error() string {
	return fmt.Sprintf("bad type: want %s, got %s", bte.WantType, bte.GotType)
}

func (sc *walkContext) SetDescription(description parser.ASTValue) error {
	root := sc.scope.RootBlock()
	descSpec := root.Spec().Description
	if descSpec == nil {
		return newSchemaError(fmt.Errorf("no description field"))
	}

	return sc.SetAttribute(schema.PathSpec{*descSpec}, nil, description)
}

func (sc *walkContext) AppendAttribute(path schema.PathSpec, ref []parser.Ident, val parser.ASTValue) error {
	sc.Logf("AppendAttribute(%#v, %#v, %#v, %s)", path, ref, val, val.Position())
	return sc.setAttribute(path, ref, val, true)
}

func (sc *walkContext) SetAttribute(path schema.PathSpec, ref []parser.Ident, val parser.ASTValue) error {
	sc.Logf("SetAttribute(%#v, %#v, %#v, %s)", path, ref, val, val.Position())
	return sc.setAttribute(path, ref, val, false)
}

func (sc *walkContext) setAttribute(path schema.PathSpec, ref []parser.Ident, val parser.ASTValue, appendValue bool) error {

	fullPath := combinePath(path, ref)
	if len(fullPath) == 0 {
		return newSchemaError(fmt.Errorf("empty path for SetAttribute"))
	}

	last := fullPath[len(fullPath)-1]
	pathToBlock := fullPath[:len(fullPath)-1]
	parentScope, err := sc.walkScopePath(pathToBlock)
	if err != nil {
		return err
	}

	field, walkPathErr := parentScope.Field(last.name, val.Position(), appendValue)
	if walkPathErr != nil {
		sc.Logf("parentScope.Field(%q) failed: %s", last.name, walkPathErr)
		if last.position != nil {
			return sc.WrapErr(walkPathErr, *last.position)
		} else {
			return newSchemaError(walkPathErr)
		}
	}

	_, ok := field.AsContainer()
	if ok {
		if appendValue {
			return sc.WrapErr(fmt.Errorf("cannot append to container"), val.Position())
		}
		containerScope, err := parentScope.ChildBlock(last.name, val.Position())
		if err != nil {
			return sc.WrapErr(err, val.Position())
		}

		return sc.WithScope(containerScope, func(sc Context, bs schema.BlockSpec) error {
			return sc.setContainerFromScalar(bs, val)
		})
	}

	vals, isArray := val.AsArray()
	if !isArray && appendValue {
		vals = []parser.ASTValue{val}
		isArray = true
	}
	if isArray {
		fieldArray, ok := field.AsArrayOfScalar()

		if ok { // Field and Value are both arrays.
			if !appendValue && fieldArray.Length() > 0 {
				return sc.WrapErr(fmt.Errorf("value already set"), val.Position())
			}
			for _, val := range vals {
				_, err := fieldArray.AppendASTValue(val)
				if err != nil {
					err = fmt.Errorf("SetAttribute %s, Append value: %w", field.FullTypeName(), err)
					return sc.WrapErr(err, val.Position())
				}
			}
			return nil
		}

		return sc.WrapErr(BadTypeError{
			WantType: "ArrayOfScalar",
			GotType:  field.FullTypeName(),
		}, val.Position())
	}

	sc.Logf("Attribute is not Array")

	scalarField, ok := field.AsScalar()
	if !ok {
		return sc.WrapErr(BadTypeError{
			WantType: "Scalar",
			GotType:  field.FullTypeName(),
		}, val.Position())
	}

	err = scalarField.SetASTValue(val)
	if err != nil {
		err = fmt.Errorf("SetAttribute %s: %w", field.FullTypeName(), err)
		return sc.WrapErr(err, val.Position())
	}
	return nil
}

func (sc *walkContext) setContainerFromScalar(bs schema.BlockSpec, val parser.ASTValue) error {
	ss := bs.ScalarSplit
	if ss == nil {
		return fmt.Errorf("container %s has no method to set from array", bs.ErrName())
	}

	var setVals []parser.ASTValue

	if ss.Delimiter != nil {
		strVal, err := val.AsString()
		if err != nil {
			return sc.WrapErr(err, val.Position())
		}
		sc.Logf("Splitting scalar %#v -> %q", val, strVal)
		valStrings := strings.Split(strVal, *bs.ScalarSplit.Delimiter)
		vals := make([]parser.ASTValue, len(valStrings))
		for idx, str := range valStrings {
			vals[idx] = parser.NewStringValue(str, parser.SourceNode{
				Start: val.Position().Start,
				End:   val.Position().End,
			})
		}
		setVals = vals

	} else {

		vals, isArray := val.AsArray()
		if !isArray {
			return fmt.Errorf("container %s requires an array when setting from value, got a scalar", bs.ErrName())
		}
		setVals = vals
	}
	sc.Logf("setContainerFromArray(%#v)", setVals)

	if ss.RightToLeft {
		slices.Reverse(setVals)
	}

	if len(setVals) < len(ss.Required) {
		return fmt.Errorf("container %s requires %d values, got %d", bs.ErrName(), len(ss.Required), len(setVals))
	}
	intoRequired, remaining := setVals[:len(ss.Required)], setVals[len(ss.Required):]
	for idx, val := range intoRequired {
		rr := ss.Required[idx]
		if err := sc.SetAttribute(rr, nil, val); err != nil {
			return err
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	var optional []parser.ASTValue
	if len(remaining) > len(ss.Optional) {
		optional, remaining = remaining[:len(ss.Optional)], remaining[len(ss.Optional):]
	} else {
		optional, remaining = remaining, nil
	}

	for idx, val := range optional {
		ro := ss.Optional[idx]
		if err := sc.SetAttribute(ro, nil, val); err != nil {
			return err
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	if ss.Remainder == nil {
		return fmt.Errorf("container %s has more array fields than we know what to do with", bs.ErrName())
	}

	// We reverse at the start to pop values from the end of the array, but when
	// placing back into remainder it should be in the specified order.
	// a.b.c, with RTL, pop `c` as a required element, then a.b is remainder,
	// not b.a
	if ss.RightToLeft {
		slices.Reverse(remaining)
	}

	remainingStr := make([]string, len(remaining))
	for idx, val := range remaining {
		var err error
		remainingStr[idx], err = val.AsString()
		if err != nil {
			return sc.WrapErr(err, val.Position())
		}
	}

	delim := "."
	if ss.Delimiter != nil {
		delim = *ss.Delimiter
	}
	singleString := strings.Join(remainingStr, delim)

	return sc.SetAttribute(*ss.Remainder, nil, parser.NewStringValue(singleString, parser.SourceNode{
		Start: remaining[0].Position().Start,
		End:   remaining[len(remaining)-1].Position().End,
	}))

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
			schema: wc.scope,
		}
	}
	return nil
}

func (wc *walkContext) WithScope(newScope *schema.Scope, fn SpanCallback) error {
	return wc.withSchema(newScope, fn)
}
func (wc *walkContext) withSchema(newScope *schema.Scope, fn SpanCallback) error {
	lastBlock := newScope.CurrentBlock()

	newPath := append(wc.path, lastBlock.Name())

	if wc.verbose {
		wc.Logf("|>>> Entering %q >>>", lastBlock.Name())
		prefix := strings.Repeat("| ", wc.depth) + "|> "
		entry := prefixer(log.Printf, prefix)
		entry("Src = %q", strings.Join(lastBlock.Path(), "."))
		entry("Path = %q", strings.Join(newPath, "."))

		newScope.PrintScope(entry)
	}

	childContext := &walkContext{
		scope:         newScope,
		path:          newPath,
		depth:         wc.depth + 1,
		verbose:       wc.verbose,
		blockLocation: wc.blockLocation,
	}

	err := childContext.run(func(sc Context) error {
		return fn(sc, lastBlock.Spec())
	})
	if err != nil {
		return err
	}
	if wc.verbose {
		wc.Logf("|<<< Exiting %q <<<", lastBlock.Name())
	}
	return nil
}

type HasPosition interface {
	Position() errpos.Position
}

func (wc *walkContext) WrapErr(err error, pos HasPosition) error {
	if err == nil {
		panic("WrapErr called with nil error")
	}

	wc.Logf("Wrapping Error %s with %s", err, pos.Position())
	err = errpos.AddContext(err, strings.Join(wc.path, "."))
	err = errpos.AddPosition(err, pos.Position())
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
	schema *schema.Scope
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
