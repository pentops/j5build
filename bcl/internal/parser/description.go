package parser

import (
	"strings"
)

func reformatDescription(input string, maxWidth int) []string {
	lines := strings.Split(input, "\n")
	linesOut := []string{}

	pend := ""
	lastWasEmpty := false
	for idx, line := range lines {

		if idx > 0 && strings.TrimSpace(line) == "" {
			if pend != "" {
				linesOut = append(linesOut, pend)
				pend = ""
			}
			// prevent duplicate newlines
			if !lastWasEmpty {
				linesOut = append(linesOut, "")
			}
			lastWasEmpty = true
			continue
		}
		lastWasEmpty = false

		words := strings.Split(line, " ")
		for _, word := range words {
			if pend == "" {
				pend = word
				continue
			}
			if len(pend)+len(word) > maxWidth {
				linesOut = append(linesOut, pend)
				pend = word
				continue
			}
			pend += " " + word
		}
	}
	if pend != "" {
		linesOut = append(linesOut, pend)
	}
	return linesOut
}
