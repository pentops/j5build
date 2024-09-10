// package errpos provides position/syntax and context path based errors,
// compatible with linters and humans.
package errpos

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// Position represents a position within a file-like string
type Position struct {
	// Optional filename. Nothing special about the value but
	// it is printed where a filename would usually be printed.
	Filename *string

	Start Point
	End   Point
}

func (p Position) Position() Position {
	return p
}

type Point struct {
	Line   int
	Column int
}

func (p Point) String() string {
	return fmt.Sprintf("%d:%d", p.Line+1, p.Column+1)
}

func (p Position) String() string {
	prefix := ""
	if p.Filename != nil {
		prefix = *p.Filename + ":"
	}
	return fmt.Sprintf("%s%d:%d", prefix, p.Start.Line+1, p.Start.Column+1)
}

type HasPosition interface {
	error
	ErrorPosition() *Position
}

type withoutPosition interface {
	HasPosition
	WithoutPosition() error
}

func GetErrorPosition(err error) *Position {
	var posErr HasPosition
	if errors.As(err, &posErr) {
		return posErr.ErrorPosition()
	}
	return nil
}

// Context is the location of an error within a schema or other 'walkable'
// structure. The elements are arbitrary and defined by the producer.
type Context []string

func (c Context) String() string {
	return strings.Join(c, ".")
}

// Errors allows a list of errors to be treated as a single error passing
// through the tree, but split out at the end, e.g. multiple syntax errors in a
// file.
type Errors []*Err

func (e Errors) Append(err error) Errors {
	if err == nil {
		return e
	}

	if errs, ok := AsErrors(err); ok {
		return append(e, errs...)
	}

	single := &Err{}
	if errors.As(err, &single) {
		return append(e, single)
	}

	return append(e, &Err{
		Err: err,
	})
}

func (e Errors) Error() string {
	if len(e) > 0 {
		return e[0].Error()
	}
	return "multiple syntax errors"
}

func AsError(err error) (*Err, bool) {
	if err == nil {
		return nil, false
	}

	var single *Err
	if errors.As(err, &single) {
		single.mergeErr(err, "Group")
		return single, true
	}

	return nil, false

}

type multiError interface {
	Unwrap() []error
}

func AsErrors(err error) (Errors, bool) {
	if err == nil {
		log.Printf("is not an error")
		return nil, false
	}

	if errs, ok := err.(Errors); ok {
		return errs, true
	}

	var single *Err
	if errors.As(err, &single) {
		single.mergeErr(err, "Group")
		return Errors{single}, true
	}

	if multi, ok := err.(multiError); ok {
		errs := multi.Unwrap()
		multiErrs := make(Errors, 0, len(errs))
		for _, e := range errs {
			wrapped, ok := AsErrors(e)
			if ok {
				multiErrs = append(multiErrs, wrapped...)
			} else {
				multiErrs = append(multiErrs, &Err{
					Err: fmt.Errorf("<?>: %w", e),
				})
			}
		}
		return multiErrs, true
	}

	var posSingle HasPosition
	if errors.As(err, &posSingle) {
		without, ok := posSingle.(withoutPosition)
		if ok {
			err = without.WithoutPosition()
		}
		return Errors{&Err{
			Pos: posSingle.ErrorPosition(),
			Err: err,
		}}, true
	}

	return nil, false
}

// Error wraps it all together.
// Short names are annoying but - duck typing.
type Err struct {
	Pos *Position
	Ctx Context
	Err error
}

var _ HasPosition = &Err{}

func (e *Err) Error() string {
	parts := make([]string, 0)
	if e.Pos != nil {
		parts = append(parts, e.Pos.String(), " ")
	}
	if len(e.Ctx) > 0 {
		parts = append(parts, "in ", e.Ctx.String(), ": ")
	}
	if e.Err == nil {
		parts = append(parts, "<nil error>")
	} else {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, "")
}

func (e *Err) ErrorPosition() *Position {
	return e.Pos
}

// Unwrap implements errors.Wrapper, but is also useful to get the 'context
// free' error message.
func (e *Err) Unwrap() error {
	return e.Err
}

// Check if any useful wrapper was added to the error
func (e *Err) mergeErr(err error, label string) {
	if e == err || e.Err == err {
		return // no change
	}
	if errors.Is(err, e.Err) {
		return // no change
	}
	fmt.Printf("==========\nMERGE ERRORS %s\n    new: %v \n      %T \n  exist: %v\n      %T\n----\n", label, err, err, e.Err, e.Err)
	e.Err = fmt.Errorf("%w: %v", e.Err, err)
}

// WithContext adds context elements to an error.
// If err is nil, nil is returned
// If the errors.As matches `*Error` (not an interface), the input is added to
// the *start* of the existing context.
// Otherwise a new Error is returned.
func AddContext(err error, ctx ...string) error {
	if err == nil {
		return nil
	}

	existing := &Err{}
	if !errors.As(err, &existing) {
		return &Err{
			Pos: GetErrorPosition(err),
			Ctx: ctx,
			Err: err,
		}
	}

	existing.mergeErr(err, "Context")
	existing.Ctx = append(ctx, existing.Ctx...)
	return existing
}

// AddPosition adds a source position to an error.
// If the error is nil, returns nil.
// If the error already has a position (implements Position), it is returned
// unmodified, as the existing value is likely more specific and useful.
func AddPosition(err error, pos Position) error {
	if err == nil {
		return nil
	}

	existing := &Err{}
	if !errors.As(err, &existing) {
		return &Err{
			Pos: &pos,
			Err: err,
		}
	}

	if existing.Pos != nil {
		return err
	}

	existing.mergeErr(err, "Position")
	existing.Pos = &pos
	return existing
}

func AddFilename(err error, filename string) error {
	if err == nil {
		return nil
	}

	asErrors, ok := AsErrors(err)
	if !ok {
		return &Err{
			Pos: &Position{
				Filename: &filename,
			},
			Err: err,
		}
	}

	for _, existing := range asErrors {
		if existing.Pos == nil {
			existing.Pos = &Position{
				Filename: &filename,
			}
		} else {
			existing.Pos.Filename = &filename
		}
	}
	return asErrors
}
