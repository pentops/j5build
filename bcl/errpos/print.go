package errpos

import (
	"errors"
	"fmt"
	"strings"
)

type ErrorsWithSource struct {
	lines  []string
	Errors Errors
}

func (e ErrorsWithSource) HumanString(contextLines int) string {
	if len(e.Errors) == 0 {
		// should not happen, this is not an error.
		return "<ErrorsWithWource[]>"
	}

	lines := make([]string, 0)

	for idx, err := range e.Errors {
		if idx > 0 {
			lines = append(lines, "-----")
		}

		str := humanString(err, e.lines, contextLines)
		lines = append(lines, str)
	}

	return strings.Join(lines, "\n")
}

func (e ErrorsWithSource) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	// should not happen, this is not an error.
	return "<ErrorsWithWource[]>"
}

func AsErrorsWithSource(err error) (*ErrorsWithSource, bool) {
	var posErr *ErrorsWithSource
	ok := errors.As(err, &posErr)
	if ok {
		return posErr, true
	}

	return nil, false
}

func humanString(err *Err, lines []string, context int) string {
	out := &strings.Builder{}

	out.WriteString("Parser Error: ")
	out.WriteString("\n")

	func() {
		if err.Pos == nil {
			out.WriteString("<no position information>")
			out.WriteString("\n")
			return
		}
		out.WriteString(fmt.Sprintf("Position: %s\n", err.Pos.String()))

		pos := *err.Pos

		if pos.Line > len(lines) {
			out.WriteString(fmt.Sprintf("<line %d out of range (%d)>", pos.Line, len(lines)))
			out.WriteString("\n")
			return
		}

		for context > 0 && pos.Line-context > 0 {
			lineNum := pos.Line - context
			line := lines[lineNum-1]
			out.WriteString(fmt.Sprintf("  > %03d: ", lineNum))
			out.WriteString(tabsToSpaces(line))
			out.WriteString("\n")
			context--
		}

		if pos.Line > len(lines) || pos.Line < 1 {
			out.WriteString(fmt.Sprintf("<line %d out of range (%d)>", pos.Line, len(lines)))
			out.WriteString("\n")
			return
		}

		errLine := lines[pos.Line-1]

		prefix := fmt.Sprintf("  > %03d", pos.Line)
		out.WriteString(prefix)
		out.WriteString(": ")
		out.WriteString(tabsToSpaces(errLine))
		out.WriteString("\n")

		if pos.Column == len(errLine)+1 {
			// allows for the column to reference the EOF or EOL
			errLine += " "
		}

		if pos.Column < 1 || pos.Column > len(errLine) {
			// negative columns should not occur but let's not crash.
			out.WriteString(strings.Repeat(">", len(prefix)))
			out.WriteString(": ")
			fmt.Fprintf(out, "<column %d out of range>\n", pos.Column)
			out.WriteString("\n")
			return
		}

		errCol := replaceRunes(errLine[:pos.Column-1], func(r string) string {
			if r == "\t" {
				return "  "
			}
			return " "
		})

		out.WriteString(strings.Repeat(">", len(prefix)))
		out.WriteString(": ")
		out.WriteString(errCol)
		out.WriteString("^\n")

	}()
	if err.Ctx != nil {
		out.WriteString("Context: ")
		out.WriteString(err.Ctx.String())
		out.WriteString("\n")
	}
	if err.Err != nil {
		out.WriteString("Message: ")
		out.WriteString(err.Err.Error())
		out.WriteString("\n")
	}
	return out.String()
}

func tabsToSpaces(s string) string {
	return replaceRunes(s, func(r string) string {
		if r == "\t" {
			return "  "
		}
		return r
	})
}

func replaceRunes(s string, cb func(string) string) string {
	runes := []rune(s)
	out := make([]string, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		out = append(out, cb(string(runes[i])))
	}
	return strings.Join(out, "")
}

func MustAddSource(err error, fileSource string) (*ErrorsWithSource, error) {
	input, ok := AsErrors(err)
	if !ok {
		return nil, fmt.Errorf("error not valid for source: (%T) %w", err, err)
	}

	return &ErrorsWithSource{
		lines:  strings.Split(fileSource, "\n"),
		Errors: input,
	}, nil
}

func setFilenames(input Errors, filename string) Errors {
	for idx, err := range input {
		if err.Pos == nil {
			err.Pos = &Position{
				Filename: &filename,
			}
		} else {
			err.Pos.Filename = &filename
		}
		input[idx] = err
	}

	return input
}

func AddSourceFile(err error, filename string, fileData string) error {
	if withSource, ok := AsErrorsWithSource(err); ok {
		errors := setFilenames(withSource.Errors, filename)
		return &ErrorsWithSource{
			lines:  strings.Split(fileData, "\n"),
			Errors: errors,
		}
	}

	input, ok := AsErrors(err)
	if !ok {
		return err
	}

	input = setFilenames(input, filename)

	return &ErrorsWithSource{
		lines:  strings.Split(fileData, "\n"),
		Errors: input,
	}
}

func AddSource(err error, fileData string) error {
	input, ok := AsErrors(err)
	if !ok {
		return err
	}

	return &ErrorsWithSource{
		lines:  strings.Split(fileData, "\n"),
		Errors: input,
	}
}
