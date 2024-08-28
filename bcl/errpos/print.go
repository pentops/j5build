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

		startLine := pos.Start.Line + 1
		startCol := pos.Start.Column + 1
		if startLine > len(lines) {
			out.WriteString(fmt.Sprintf("<line %d out of range (%d)>", startLine, len(lines)))
			out.WriteString("\n")
			return
		}

		for lineNum := startLine - context; lineNum < startLine; lineNum++ {
			if lineNum < 1 {
				continue
			}
			line := lines[lineNum-1]
			out.WriteString(fmt.Sprintf("  > %03d: ", lineNum))
			out.WriteString(tabsToSpaces(line))
			out.WriteString("\n")
			context--
		}

		if startLine > len(lines) || startLine < 1 {
			out.WriteString(fmt.Sprintf("<line %d out of range (%d)>", startLine, len(lines)))
			out.WriteString("\n")
			return
		}

		errLine := lines[startLine-1]

		prefix := fmt.Sprintf("  > %03d", startLine)
		out.WriteString(prefix)
		out.WriteString(": ")
		out.WriteString(tabsToSpaces(errLine))
		out.WriteString("\n")

		if startCol == len(errLine)+1 {
			// allows for the column to reference the EOF or EOL
			errLine += " "
		}

		if startCol < 1 || startCol > len(errLine) {
			// negative columns should not occur but let's not crash.
			out.WriteString(strings.Repeat(">", len(prefix)))
			out.WriteString(": ")
			fmt.Fprintf(out, "<column %d out of range>\n", startCol)
			out.WriteString("\n")
			return
		}

		errCol := replaceRunes(errLine[:startCol-1], func(r string) string {
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
