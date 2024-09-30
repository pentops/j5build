package bcl

import "github.com/pentops/bcl.go/internal/parser"

func Fmt(data string) (string, error) {
	fixed, err := parser.Fmt(string(data))
	if err != nil {
		return "", err
	}
	return fixed, nil
}
