package gogen

import (
	"bytes"
	"fmt"
	"path"
)

func quoteString(s string) string {
	return fmt.Sprintf("%q", s)
}

type StringGen struct {
	imports   map[string]string
	buf       *bytes.Buffer
	myPackage string
}

func (g *StringGen) ImportPath(importSrc string) string {
	existing, ok := g.imports[importSrc]
	if ok {
		return existing
	}

	importName := g.findUnusedPackage(path.Base(importSrc))
	g.imports[importSrc] = importName
	return g.imports[importSrc]
}

func (g *StringGen) findUnusedPackage(want string) string {
	for _, used := range g.imports {
		if used == want {
			// TODO if the name is already a number, increment it
			return g.findUnusedPackage(want + "_1")
		}
	}

	return want
}

func (g *StringGen) ChildGen() *StringGen {
	return &StringGen{
		buf:       &bytes.Buffer{},
		imports:   g.imports,
		myPackage: g.myPackage,
	}
}

// P prints a line to the generated output. It converts each parameter to a
// string following the same rules as fmt.Print. It never inserts spaces
// between parameters.
func (g *StringGen) P(v ...interface{}) {
	for _, x := range v {
		if packaged, ok := x.(DataType); ok {
			if packaged.GoPackage == "" {
				fmt.Fprint(g.buf, packaged.Prefix(), packaged.Name)
				continue
			}

			specified := packaged.GoPackage
			if specified == g.myPackage {
				fmt.Fprint(g.buf, packaged.Prefix(), packaged.Name)
			} else {
				importedName := g.ImportPath(packaged.GoPackage)
				fmt.Fprint(g.buf, packaged.Prefix(), importedName, ".", packaged.Name)
			}
		} else {
			fmt.Fprint(g.buf, x)
		}
	}
	fmt.Fprintln(g.buf)
}
