package gogen

import (
	"strings"
)

func goTypeName(name string) string {
	// Underscores are used to separate nested-scoped types, e.g. a message
	// defined within a message in proto, this function preserves the underscores
	// but fixes up any casing in between - which basically results in capatalizing
	// the first letter.
	parts := strings.Split(name, "_")
	for i, part := range parts {
		parts[i] = exportName(part)
	}
	return strings.Join(parts, "_")
}

func exportName(name string) string {
	if len(name) == 0 {
		return name
	}

	out := make([]rune, 0, len(name))
	for i, r := range name {
		if r >= 'a' && r <= 'z' {
			if i == 0 {
				out = append(out, r-'a'+'A')
			} else {
				out = append(out, r)
			}
		} else if r >= 'A' && r <= 'Z' {
			out = append(out, r)
		} else if r >= '0' && r <= '9' {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
