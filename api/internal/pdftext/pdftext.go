// Package pdftext reads back the text a PDF this codebase rendered with
// fpdf draws, for a test to assert on what a person reading the file
// would see. fpdf compresses each page's content stream, so the words are
// not in the bytes as written.
//
// It began as a test-only helper inside plans (#2209, the Birth Plan
// masthead) and moved here when the signed Contract PDF became its second
// reader (#2214): a helper in a _test.go file cannot be imported across
// packages. It is a reader for the PDFs fpdf writes, not a general PDF
// parser -- it knows only the "(...)Tj" text-showing form fpdf emits.
package pdftext

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
)

// Text inflates every content stream in pdf and returns each string a
// text-showing operator draws, one per line, in drawing order. A stream
// that is not zlib-compressed (an embedded font, say) is skipped.
func Text(pdf []byte) string {
	var lines []string
	rest := pdf
	for {
		start := bytes.Index(rest, []byte("stream\n"))
		if start < 0 {
			break
		}
		rest = rest[start+len("stream\n"):]
		end := bytes.Index(rest, []byte("endstream"))
		if end < 0 {
			break
		}
		raw := rest[:end]
		rest = rest[end+len("endstream"):]
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		content, err := io.ReadAll(r)
		if err != nil && len(content) == 0 {
			continue
		}
		lines = append(lines, shownStrings(string(content))...)
	}
	return strings.Join(lines, "\n")
}

// shownStrings pulls each "(...)Tj" literal out of a content stream,
// undoing the escapes fpdf writes for "(", ")" and "\".
func shownStrings(content string) []string {
	var out []string
	for line := range strings.SplitSeq(content, "\n") {
		open := strings.Index(line, "(")
		closeAt := strings.LastIndex(line, ")Tj")
		if open < 0 || closeAt <= open {
			continue
		}
		s := line[open+1 : closeAt]
		s = strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`).Replace(s)
		out = append(out, s)
	}
	return out
}
