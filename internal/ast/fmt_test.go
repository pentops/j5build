package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/internal/ast/testdata"
)

func TestFmtDiff(t *testing.T) {
	type testCase struct {
		input    string
		expected []FmtDiff
	}
	s := func(s ...string) string { return strings.Join(s, "\n") }

	xrun := func(name string, _ testCase) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Skip("skipping")
		})
	}
	run := func(name string, tc testCase) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()

			actual, err := FmtDiffs(tc.input)
			if err != nil {
				printErr(t, err)
				t.Fatal(err)
			}

			testCompare(t, tc.expected, actual, func(a, b FmtDiff) bool {
				return a.FromLine == b.FromLine && a.ToLine == b.ToLine && a.NewText == b.NewText
			}, func(a FmtDiff) string {
				return fmt.Sprintf("%d %d %s", a.FromLine, a.ToLine, a.NewText)
			})
		})
	}

	run("simple valid", testCase{
		input:    "a = 1\n",
		expected: []FmtDiff{},
	})

	run("leading", testCase{
		input:    "\na = 1\n",
		expected: []FmtDiff{{0, 1, ""}},
	})

	xrun("trailing", testCase{
		input:    "a = 1\n\n",
		expected: []FmtDiff{{1, 2, ""}},
	})

	run("fix space", testCase{
		input: s(
			"a=1",
			"a  =  1",
			"a = 1 ",
		),
		expected: []FmtDiff{
			{0, 1, "a = 1\n"},
			{1, 2, "a = 1\n"},
			{2, 3, "a = 1\n"},
		},
	})

	run("fix gaps", testCase{
		input: s(
			"a = 1",
			"",
			"",
			"b = 2",
		),
		expected: []FmtDiff{
			{1, 3, "\n"},
		},
	})
}

type slicePop[T any] struct {
	val []T
	idx int
}

func (s *slicePop[T]) pop() (val T, ok bool) {
	s.idx++
	if s.idx < len(s.val) {
		ok = true
		val = s.val[s.idx]
	}
	return
}

func (s *slicePop[T]) peek(offset int) (val T, ok bool) {
	idx := s.idx + offset
	if idx < len(s.val) {
		ok = true
		val = s.val[idx]
	}
	return
}

func testCompareStrings(t *testing.T, want, got []string) {
	t.Helper()
	t.Logf("GOT %q", got)
	t.Logf("WAN %q", want)
	testCompare(t, want, got,
		func(a, b string) bool { return a == b },
		func(s string) string { return s },
	)
}
func testCompare[T any](t *testing.T, wantSlice, gotSlice []T, cmp func(T, T) bool, str func(T) string) {
	t.Helper()

	want := slicePop[T]{val: wantSlice, idx: -1}
	got := slicePop[T]{val: gotSlice, idx: -1}

	logGot := func() {
		t.Helper()
		t.Logf("%03d % 3s %s", got.idx, "---", str(got.val[got.idx]))
	}
	logWant := func() {
		t.Helper()
		t.Logf("% 3s %03d %s", "---", want.idx, str(want.val[want.idx]))
	}
	logMatch := func() {
		t.Helper()
		t.Logf("%03d %03d %s", got.idx, want.idx, str(want.val[want.idx]))
	}

	t.Logf("% 3s % 3s, Comparing", "got", "wan")
	fail := false
	for {
		wantVal, hasWant := want.pop()
		gotVal, hasGot := got.pop()
		if !hasWant && !hasGot {
			break
		}
		if !hasWant {
			logGot()
			fail = true
			continue
		} else if !hasGot {
			logWant()
			fail = true
			continue
		}

		if cmp(wantVal, gotVal) {
			logMatch()
			continue
		}
		fail = true

		// got matches the next want, got is missing a value
		if nextWant, has := want.peek(1); has {
			if cmp(nextWant, gotVal) {
				logWant()
				want.pop()
				logMatch()
				continue
			}

		}

		// next got matches the want, got has an extra value
		if nextGot, has := got.peek(1); has {
			if cmp(wantVal, nextGot) {
				logGot()
				got.pop()
				logMatch()
				continue
			}
		}

		logWant()
		logGot()
	}
	if fail {
		t.Fail()
	}
}

func TestFmt(t *testing.T) {
	s := func(s ...string) string { return strings.Join(s, "\n") }

	type fmtCase struct {
		expected string
		inputs   []string
	}

	run := func(name string, tc fmtCase) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			t.Helper()

			// all expected output will end with a newline, this makes the input
			// a bit easier to read
			if !strings.HasSuffix(tc.expected, "\n") {
				tc.expected += "\n"
			}
			for idx, input := range tc.inputs {
				t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
					actual, err := Fmt(input)
					if err != nil {
						printErr(t, err)
						t.Fatal(err)
					}

					testCompare(t,
						strings.Split(tc.expected, "\n"),
						strings.Split(actual, "\n"),
						func(a, b string) bool {
							return a == b
						}, func(a string) string {
							return a
						})

				})
			}
		})
	}

	run("simple", fmtCase{
		expected: s(`a = 1`),
		inputs: []string{
			s(`a = 1`),
			s(`a = 1`),
			s(`a=1`),
			s(`  a=1   `),
		},
	})

	run("multiple", fmtCase{
		expected: s(`a = 1`, `b = 2`),
		inputs: []string{
			s(`a = 1`, `b = 2`),
			s(`a = 1`, `b = 2`),
			s(`a = 1`, `  b = 2`),
			s(`  a = 1`, `b = 2`),
		},
	})

	run("block", fmtCase{
		expected: `a b ! c d:e | f`,
		inputs: []string{
			`a b ! c d : e | f`,
			`a b!c d:e   |    f`,
		},
	})

	run("fmt.bcl", fmtCase{
		testdata.FmtInput,
		[]string{
			testdata.FmtInput,
			testdata.FmtBad1,
		},
	})

}
