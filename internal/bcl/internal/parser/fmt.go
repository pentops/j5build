package parser

import (
	"fmt"
	"strings"

	"github.com/pentops/j5build/internal/bcl/errpos"
)

type FmtDiff struct {
	FromLine int // 0 based
	ToLine   int // 0 based, Exclusive
	NewText  string
}

func Fmt(input string) (string, error) {
	diffs, err := collectFmtFragments(input)
	if err != nil {
		return "", err
	}

	out := make([]string, 0, len(diffs))
	lastEnd := -1
	for idx, diff := range diffs {
		if idx > 0 && (diff.FromLine > lastEnd) {
			out = append(out, "\n")
		}
		out = append(out, diff.NewText)
		lastEnd = diff.ToLine
	}
	return strings.Join(out, ""), nil
}

func FmtDiffs(input string) ([]FmtDiff, error) {
	all, err := collectFmtFragments(input)
	if err != nil {
		return nil, err
	}

	lines := &lineSet{
		lines: strings.Split(input, "\n"),
	}

	out := make([]FmtDiff, 0, len(all))
	lastEnd := -1
	for idx, diff := range all {
		if idx == 0 {
			// Remove any leading empty lines
			if diff.FromLine > 0 {
				out = append(out, FmtDiff{
					FromLine: 0,
					ToLine:   diff.FromLine,
					NewText:  "",
				})
			}
		} else if diff.FromLine > lastEnd+1 {
			// FromLine == LastEnd  means no gap
			// FromLine == LastEnd + 1  is one line gap, OK
			// FromLine > LastEnd + 1 should be one line
			out = append(out, FmtDiff{
				FromLine: lastEnd,
				ToLine:   diff.FromLine,
				NewText:  "\n",
			})
		}
		existing := lines.rangeLines(diff.FromLine, diff.ToLine)
		if existing != diff.NewText {
			out = append(out, diff)
		}
		lastEnd = diff.ToLine
	}
	return out, nil
}

type lineSet struct {
	lines []string
}

func (ls *lineSet) rangeLines(from, to int) string {
	return strings.Join(ls.lines[from:to], "\n") + "\n"
}

func collectFmtFragments(input string) ([]FmtDiff, error) {
	l := NewLexer(input)

	tokens, ok, err := l.AllTokens(true)
	if err != nil {
		return nil, fmt.Errorf("unexpected lexer error: %w", err)
	}
	if !ok {
		return nil, errpos.AddSource(l.Errors, input)
	}
	ww := &Walker{
		tokens:   tokens,
		failFast: true,
	}
	fragments, err := ww.walkFragments()
	if err != nil {
		if err == HadErrors {
			return nil, errpos.AddSource(ww.errors, input)
		}

		return nil, err
	}
	fmter := &fmter{}
	fmter.diffFile(fragments)

	return fmter.fragments, nil
}

type fmter struct {
	fragments []FmtDiff
	indent    int
}

func (p *fmter) diffFile(ff []Fragment) {

	for idx := 0; idx < len(ff); idx++ {
		stmt := ff[idx]
		switch stmt := stmt.(type) {
		case BlockHeader:
			p.doBlockHeader(stmt)

		case CloseBlock:
			p.closeBlock(stmt)

		case Assignment:
			p.doAssignment(stmt)

		case Description:
			p.doDescription(stmt)

		case Comment:
			p.printComment(stmt)

		default:
			panic(fmt.Sprintf("FMT unknown statement %T", stmt))
		}
	}
}

func tokenSource(tok Token) string {
	switch tok.Type {
	case STRING:
		return fmt.Sprintf("%q", tok.Lit)
	case REGEX:
		return fmt.Sprintf("/%s/", tok.Lit)
	case DESCRIPTION:
		return fmt.Sprintf("| %s", tok.Lit)
	case COMMENT:
		return fmt.Sprintf("//%s", tok.Lit)
	case BLOCK_COMMENT:
		return fmt.Sprintf("/*%s*/", tok.Lit)
	}
	return tok.Lit
}

func (p *fmter) singleLineTokens(src SourceNode, parts ...Token) {
	line := ""
	for _, part := range parts {
		line += tokenSource(part)
	}
	if src.Comment != nil {
		line += inlineComment(src.Comment)
	}
	line = strings.Repeat("\t", p.indent) + line + "\n"

	p.fragments = append(p.fragments, FmtDiff{
		FromLine: src.Start.Line,
		ToLine:   src.End.Line + 1, // exclusive
		NewText:  line,
	})
}

func (p *fmter) multiLineToken(src SourceNode, prefix string, lines []string) {

	fullPrefix := strings.Repeat("\t", p.indent) + prefix
	for idx, part := range lines {
		// remove trailing space INCLUDING anything after the prefix,
		// for descriptions "| " becomes "\t|"
		lines[idx] = strings.TrimRight(fullPrefix+part, " ")
	}

	p.fragments = append(p.fragments, FmtDiff{
		FromLine: src.Start.Line,
		ToLine:   src.End.Line + 1, // exclusive
		NewText:  strings.Join(lines, "\n") + "\n",
	})
}

func (p *fmter) closeBlock(block CloseBlock) {
	p.indent--
	if p.indent < 0 {
		p.indent = 0
		// This is kind of OK for a fmt
	}
	p.singleLineTokens(block.SourceNode, block.Token)
}

func newToken(ty TokenType, value string) Token {
	return Token{
		Type: ty,
		Lit:  value,
	}
}

func (p *fmter) doBlockHeader(block BlockHeader) {

	nameParts := referenceTokens(block.Type)
	for _, val := range block.Tags {
		nameParts = append(nameParts, newToken(SPACE, " "))
		nameParts = append(nameParts, tagString(val)...)
	}

	for _, val := range block.Qualifiers {
		// no spaces between qualifiers
		nameParts = append(nameParts, newToken(COLON, ":"))
		nameParts = append(nameParts, tagString(val)...)
	}

	if block.Open {
		nameParts = append(nameParts, newToken(SPACE, " "), newToken(LBRACE, "{"))
	}
	if block.Description != nil {
		nameParts = append(nameParts, newToken(SPACE, " "))
		nameParts = append(nameParts, block.Description.Tokens...)
	}

	p.singleLineTokens(block.SourceNode, nameParts...)

	if block.Open {
		p.indent++
	}

}

func (p *fmter) printComment(comment Comment) {
	p.singleLineTokens(comment.SourceNode, comment.Token)
}

func (p *fmter) doDescription(desc Description) {
	linesOut := reformatDescription(desc.Value, 80-p.indent*4)
	p.multiLineToken(desc.SourceNode, "| ", linesOut)
}

func (p *fmter) doAssignment(assign Assignment) {
	tokens := referenceTokens(assign.Key)

	if assign.Append {
		tokens = append(tokens,
			newToken(SPACE, " "),
			newToken(PLUS, "+"),
			newToken(ASSIGN, "="),
			newToken(SPACE, " "),
		)
	} else {
		tokens = append(tokens,
			newToken(SPACE, " "),
			newToken(ASSIGN, "="),
			newToken(SPACE, " "),
		)
	}
	tokens = append(tokens, valueTokens(assign.Value)...)
	p.singleLineTokens(assign.SourceNode, tokens...)
}

func valueTokens(v Value) []Token {
	if v.array == nil {
		return []Token{v.token}
	}

	toks := []Token{}
	toks = append(toks, newToken(LBRACK, "["))
	for idx, val := range v.array {
		if idx > 0 {
			toks = append(toks,
				newToken(COMMA, ","),
				newToken(SPACE, " "))
		}
		toks = append(toks, valueTokens(val)...)
	}
	toks = append(toks, newToken(RBRACK, "]"))
	return toks
}

func inlineComment(comment *Comment) string {
	if comment == nil {
		return ""
	}
	return " //" + comment.Value
}

func tagString(v TagValue) []Token {
	toks := []Token{}

	if v.Mark != TagMarkNone {
		toks = append(toks, v.MarkToken, newToken(SPACE, " "))
	}

	if v.Value != nil {
		toks = append(toks, v.Value.token)
	}
	if v.Reference != nil {
		toks = append(toks, referenceTokens(*v.Reference)...)
	}
	return toks
}

func referenceTokens(r Reference) []Token {
	toks := []Token{}
	for idx, part := range r.Idents {
		if idx == 0 {
			toks = append(toks, part.Token)
		} else {
			toks = append(toks, newToken(DOT, "."), part.Token)
		}
	}
	return toks
}
