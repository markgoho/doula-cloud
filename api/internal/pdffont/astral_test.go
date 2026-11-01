package pdffont_test

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"doula-cloud/api/internal/pdffont"
	"doula-cloud/api/internal/pdftext"
)

const babyLine = "Ada ❤️ Lovelace 👶"

// #2550: a character outside the BMP is shown as a code the ToUnicode CMap
// maps back to it, not as U+FFFD in an ActualText span, which pdf.js and
// PDFium do not read.
func TestParagraph_MapsACharacterOutsideTheBMPInToUnicode(t *testing.T) {
	out := render(t, func(d *pdffont.Doc) {
		d.SetCompression(false)
		d.Paragraph(pdffont.Regular, 12, 8, babyLine)
	})
	if !bytes.Contains(out, []byte("<FDD0> <D83DDC76>")) {
		t.Fatal("no ToUnicode entry maps a code to U+1F476")
	}
	if bytes.Contains(out, []byte("/ActualText")) {
		t.Fatal("the layer still wraps U+1F476 in an ActualText span")
	}
	assertCrossReferences(t, out)
	pdfium := reader(t, out, "python3", "-I", "-c", "import sys, pypdfium2; d = pypdfium2.PdfDocument(sys.argv[1]); print(''.join(p.get_textpage().get_text_range() for p in d))", pdfPath)
	if got, want := readerText(pdfium), babyLine; got != want {
		t.Fatalf("PDFium = %q, want %q", got, want)
	}
}

// Past the last code, a character outside the BMP falls back to an
// ActualText span, and a noncharacter typed from the codes' own range
// prints as U+FFFD, so neither reads as another character.
func TestParagraph_FallsBackToActualTextPastTheLastCode(t *testing.T) {
	var many strings.Builder
	for r := rune(0x1F600); r < 0x1F600+40; r++ {
		many.WriteRune(r)
	}
	raw := render(t, func(d *pdffont.Doc) {
		d.SetCompression(false)
		d.Paragraph(pdffont.Regular, 12, 8, many.String()+" \uFDD0")
	})
	if !bytes.Contains(raw, []byte("/ActualText")) {
		t.Fatal("no ActualText span past the last code")
	}
	assertCrossReferences(t, raw)
	out := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, many.String()+" \uFDD0")
	})
	if got, want := strings.ReplaceAll(pdftext.Text(out), "\n", ""), many.String()+" \uFFFD"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// Every cross-reference entry still points at its object, and startxref
// at the table, once the ToUnicode CMaps have grown.
func assertCrossReferences(t *testing.T, pdf []byte) {
	t.Helper()
	m := regexp.MustCompile(`\nxref\n0 (\d+)\n`).FindSubmatchIndex(pdf)
	if m == nil {
		t.Fatal("no cross-reference table")
	}
	n, _ := strconv.Atoi(string(pdf[m[2]:m[3]]))
	entries := pdf[m[1]:]
	for i := 1; i < n; i++ {
		offset, _ := strconv.Atoi(string(entries[i*20 : i*20+10]))
		if !bytes.HasPrefix(pdf[offset:], fmt.Appendf(nil, "%d 0 obj", i)) {
			t.Fatalf("object %d's offset %d points at %q", i, offset, pdf[offset:offset+12])
		}
	}
	start := bytes.LastIndex(pdf, []byte("startxref\n")) + len("startxref\n")
	end := bytes.IndexByte(pdf[start:], '\n')
	offset, _ := strconv.Atoi(string(pdf[start : start+end]))
	if !bytes.HasPrefix(pdf[offset:], []byte("xref\n")) {
		t.Fatalf("startxref %d points at %q", offset, pdf[offset:offset+12])
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOutput_ReturnsEachError(t *testing.T) {
	d := pdffont.New()
	if err := d.Output(failingWriter{}); err == nil {
		t.Fatal("Output to a failing writer returned no error")
	}
	d = pdffont.New()
	d.SetError(errors.New("broken"))
	var buf bytes.Buffer
	if err := d.Output(&buf); err == nil {
		t.Fatal("Output of a broken document returned no error")
	}
}
