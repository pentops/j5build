package errpos

import (
	"log"
	"strings"
	"testing"
)

func TestPrintAlignment(t *testing.T) {

	run := func(input []string, line, column int, expected string) {
		t.Log("input:", input)
		t.Log("pos:", line, column)
		err := &Err{
			Pos: &Position{
				Start: Point{
					Line:   line,
					Column: column,
				},
			},
		}

		str := humanString(err, input, 0)
		t.Log(str)
		logLines := strings.Split(str, "\n")
		var badLine, marker string
		for idx, line := range logLines {
			if strings.HasPrefix(line, ">>>>>") {
				badLine = logLines[idx-1]
				marker = line
				break
			}
		}

		log.Printf("   BAD: %s", badLine)
		log.Printf("  MARK: %s", marker)
		idxBadLine := strings.IndexRune(badLine, ':')
		idxMarker := strings.IndexRune(marker, ':')
		if idxBadLine != idxMarker {
			t.Fatalf("Prefixes do not have the same length")
		}
		if idxBadLine == -1 {
			t.Fatalf("Prefixes not found")
		}

		want := marker[:idxMarker] + ": " + expected
		log.Printf("  WANT: %q", want)

		if marker != want {
			t.Fatalf("Marker did not match. \n GOT  %q\n WANT %q", marker, want)
		}

	}

	run([]string{"123"}, 0, 0, "^")
	run([]string{"123"}, 0, 1, " ^")
	run([]string{"123"}, 0, 2, "  ^")
	run([]string{"123"}, 0, 3, "   ^")

	// allows for the column to reference the EOF or EOL
	run([]string{"123"}, 0, 4, "<column 5 out of range>")

	run([]string{"\t123"}, 0, 1, "  ^")

}
