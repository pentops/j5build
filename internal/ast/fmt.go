package ast

import (
	"fmt"
	"strings"
)

func Print(tree *File) string {

	printer := &printer{
		buf: strings.Builder{},
	}
	printer.printFile(tree)
	return printer.buf.String()
}

type printer struct {
	buf      strings.Builder
	indent   int
	needsGap bool
}

func (p *printer) gap() {
	p.needsGap = true
}

func (p *printer) line(s ...string) {
	if p.needsGap {
		p.buf.WriteString("\n")
		p.needsGap = false
	}

	p.buf.WriteString(strings.Repeat("\t", p.indent))
	for _, str := range s {
		p.buf.WriteString(str)
	}
	p.buf.WriteString("\n")
}

func (p *printer) printFile(f *File) {
	p.printPackage(f.Package)
	p.printBody(f.Body)
}

func (p *printer) printPackage(pkg string) {
	if pkg == "" {
		return
	}
	p.line("package ", pkg)
	p.line()
}

func (p *printer) printImports(imp ImportStatement) {
	if imp.IsFile {
		p.line("import ", fmt.Sprintf("%q", imp.Path))
	}
	if imp.Alias == "" {
		p.line("import ", imp.Path)
	}
	p.line("import ", imp.Path, " as ", imp.Alias)
	p.line()
}

func (p *printer) printBody(body Body) {
	lastWasStatement := false

	lastBlockType := ""
	for idx, stmt := range body.Statements {
		switch stmt := stmt.(type) {
		case ImportStatement:
			p.printImports(stmt)
			lastWasStatement = false
			lastBlockType = ".I"
		case BlockStatement:
			thisType := stmt.RootName()
			if thisType != lastBlockType && idx > 0 {
				p.gap()
			}
			p.printBlock(stmt)
			lastWasStatement = false
			lastBlockType = thisType

		case Assignment:
			if lastWasStatement {
				p.needsGap = false
			}
			p.printAssign(stmt)
			p.gap()
			lastBlockType = ".S"
		default:
			panic(fmt.Sprintf("unknown statement %T", stmt))
		}
	}
}

func referencesToStrings(refs []Reference) []string {
	strs := make([]string, len(refs))
	for idx, ref := range refs {
		strs[idx] = ref.String()
	}
	return strs
}

func (p *printer) printBlock(block BlockStatement) {
	baseName := strings.Join(referencesToStrings(block.Name), " ")
	if len(block.Qualifiers) > 0 {
		qualStrs := referencesToStrings(block.Qualifiers)
		baseName += ":" + strings.Join(qualStrs, ":")
	}

	desc := block.DescriptionString()
	if block.Start.Line == block.End.Line && len(block.Body.Statements) == 0 {
		if desc == "" {
			p.line(baseName)
			return
		}
		if len(desc)+len(baseName) < 80 {
			p.line(baseName, " | ", desc)
			return
		}
	}
	p.line(baseName, " {")
	p.indent++
	if block.Description != nil {
		descStr := block.BlockHeader.DescriptionString()
		lines := strings.Split(descStr, "\n")
		for idx, line := range lines {
			if idx > 0 {
				p.line("|")
			}
			pend := ""
			words := strings.Split(line, " ")
			for _, word := range words {
				if len(pend)+len(word)+(p.indent*4) > 80 {
					p.line("| ", pend)
					pend = ""
				}
				if pend != "" {
					pend += " "
				}
				pend += word
			}
			if pend != "" {
				p.line("| ", pend)
			}

		}
		p.gap()
	}
	p.printBody(block.Body)
	p.indent--
	p.needsGap = false
	p.line("}")
	p.gap()
}

func (p *printer) printAssign(assign Assignment) {
	p.line(assign.Key.String(), " = ", assign.Value.token.Lit)
}
