package ast

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pentops/bcl.go/internal/ast/testdata"
)

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

					if actual != tc.expected {

						t.Logf("MISMATCH")
						lines1 := strings.Split(tc.expected, "\n")
						lines2 := strings.Split(actual, "\n")
						for i := 0; i < len(lines1) || i < len(lines2); i++ {
							if i < len(lines1) && i < len(lines2) {
								if lines1[i] != lines2[i] {
									t.Logf("%03d w: `%s`", i, lines1[i])
									t.Logf("%03d g: `%s`", i, lines2[i])
								} else {
									t.Logf("%03d  : `%s`", i, lines1[i])
								}
							} else if i < len(lines1) {
								t.Logf("%03d w: %s", i, lines1[i])
								t.Logf("%03d g: <EOF>", i)
							} else {
								t.Logf("%03d w: <EOF>", i)
								t.Logf("%03d g: %s", i, lines2[i])
							}
						}

						t.Errorf(" Mismatch in case %d", idx)
					}
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
		expected: `a b ! c d : e | f`,
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
