package ast

import (
	"fmt"
	"strings"

	"github.com/pentops/bcl.go/bcl/errpos"
	"github.com/pentops/bcl.go/internal/lexer"
)

/*
func Print(tree *File) string {
	printer := &printer{
		buf: strings.Builder{},
	}
	printer.printFile(tree)
	return printer.buf.String()
}
*/

func Fmt(input string) (string, error) {
	l := lexer.NewLexer(input)

	tokens, ok, err := l.AllTokens(true)
	if err != nil {
		return "", fmt.Errorf("unexpected lexer error: %w", err)
	}
	if !ok {
		return "", errpos.AddSource(l.Errors, input)
	}
	ww := &Walker{
		tokens:   tokens,
		failFast: true,
	}
	fragments, err := ww.walkFragments()
	if err != nil {
		return "", err
	}
	printer := &printer{
		buf: strings.Builder{},
	}
	printer.printFile(fragments)
	return printer.buf.String(), nil
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

func (p *printer) printFile(ff []Fragment) {
	p.printBody(ff)
}

func (p *printer) printBody(ff []Fragment) {

	lastEnd := int(-1)

	for _, stmt := range ff {
		src := stmt.Source()
		if lastEnd >= 0 && src.Start.Line > lastEnd+1 {
			p.gap()
		}
		switch stmt := stmt.(type) {
		case BlockHeader:
			p.printBlock(stmt)

		case CloseBlock:
			p.indent--
			p.needsGap = false
			p.line("}")
			p.gap()

		case Assignment:
			p.printAssign(stmt)

		case Description:
			p.printDescription(stmt)

		case Comment:
			p.printComment(stmt)

		case EOF:
			// nothing to do

		default:
			panic(fmt.Sprintf("FMT unknown statement %T", stmt))
		}
		lastEnd = src.End.Line
	}
}

func referencesToStrings(refs []Reference) []string {
	strs := make([]string, len(refs))
	for idx, ref := range refs {
		strs[idx] = ref.String()
	}
	return strs
}

func (p *printer) printBlock(block BlockHeader) {

	nameParts := []string{}
	nameParts = append(nameParts, block.Type.String())
	for _, val := range block.Tags {
		str := tagString(val)
		nameParts = append(nameParts, str)
	}

	baseName := strings.Join(nameParts, " ")
	if len(block.Qualifiers) > 0 {
		qualParts := []string{baseName}
		for _, val := range block.Qualifiers {
			str := tagString(val)
			qualParts = append(qualParts, str)
		}
		baseName = strings.Join(qualParts, " : ")
	}

	if !block.Open {
		if block.Description == nil {
			p.line(baseName)
			return
		}
		desc := *block.Description

		if len(desc)+len(baseName) < 80 {
			p.line(baseName, " | ", desc)
			return
		}
	}
	p.line(baseName, " {")
	p.indent++
}

func (p *printer) printComment(comment Comment) {
	val := comment.Value
	p.line("//", val)
}

func (p *printer) printDescription(desc Description) {
	descStr := desc.Value.token.Lit
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
}

func (p *printer) printAssign(assign Assignment) {
	joiner := "="
	if assign.Append {
		joiner = "+="
	}
	comment := inlineComment(assign.Comment)
	p.line(assign.Key.String(), " ", joiner, " ", assign.Value.sourceString(), comment)
}

func inlineComment(comment *Comment) string {
	if comment == nil {
		return ""
	}
	return " //" + comment.Value
}

func tagString(v TagValue) string {
	pre := ""

	switch v.Mark {
	case TagMarkQuestion:
		pre = "? "
	case TagMarkBang:
		pre = "! "
	}

	if v.Value != nil {
		return pre + v.Value.sourceString()
	}
	if v.Reference != nil {
		return pre + v.Reference.String()
	}
	return pre + "<INVALID>"
}
