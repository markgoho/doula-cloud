package plans

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
	"testing"
	"time"
)

// PDFText is the text a PDF this package rendered draws, for a test to
// read back (#2209): fpdf compresses each page's content stream, so the
// words are not in the bytes as written. It inflates every stream and keeps
// each string a text-showing operator draws, one per line. Exported only to
// this package's own tests, external ones included, because it lives in a
// _test.go file.
func PDFText(t *testing.T, pdf []byte) string {
	t.Helper()
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

// TestRenderPlanPDF_OpensWithItsMasthead proves the PDF names the plan
// before its first question (#2209): her name, the Practice, and the day it
// last changed, printed the way the page prints a calendar day.
func TestRenderPlanPDF_OpensWithItsMasthead(t *testing.T) {
	fields := []Field{{ID: "where", Type: fieldTypeSingleSelect, Label: "Where to give birth"}}
	m := masthead{clientName: "Hannah Sorensen", practiceName: "Rooted (Birth) Collective", changedOn: time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC)}

	out, err := renderPlanPDF(birthPlanPDFHeading, m, fields, Answers{"where": "At home"})
	if err != nil {
		t.Fatalf("renderPlanPDF: %v", err)
	}
	got := strings.Split(PDFText(t, out), "\n")
	want := []string{
		"Birth Plan",
		"Name: Hannah Sorensen",
		"Doula: Rooted (Birth) Collective",
		"Last changed: Oct 27, 2026",
		"Where to give birth",
		"At home",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("PDF text = %q, want %q", got, want)
	}
}
