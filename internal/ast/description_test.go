package ast

import (
	"strings"
	"testing"
)

func TestDescriptionFormat(t *testing.T) {

	s := func(s ...string) string { return strings.Join(s, "\n") }

	run := func(input string, want string, maxWidth int) {
		t.Helper()
		got := reformatDescription(input, maxWidth)
		wantLines := strings.Split(want, "\n")
		testCompareStrings(t, wantLines, got)
	}

	run("hello world", "hello world", 100)
	run("hello world", "hello\nworld", 4)
	run("one two three", "one two\nthree", 7)

	run(s(
		"line 1",
		"line 2",
	), s(
		"line 1 line 2",
	), 100)

	run(s(
		"line 1",
		"",
		"line 2",
	), s(
		"line 1",
		"",
		"line 2",
	), 100)

	run(s(
		"line 1",
		"",
		"",
		"line 2",
	), s(
		"line 1",
		"",
		"line 2",
	), 100)

}
