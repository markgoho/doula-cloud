package pdftext_test

import (
	"bytes"
	"compress/zlib"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"

	"doula-cloud/api/internal/pdftext"
)

// TestText_ReadsBackWhatFpdfDrew proves Text returns every line an fpdf
// document draws, in order, with fpdf's escapes for "(", ")" and "\"
// undone.
func TestText_ReadsBackWhatFpdfDrew(t *testing.T) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 8, "Rooted (Birth) Collective", "", "", false)
	pdf.MultiCell(0, 8, `A back\slash`, "", "", false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	got := pdftext.Text(buf.Bytes())
	want := "Rooted (Birth) Collective\n" + `A back\slash`
	if got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// TestText_SkipsWhatIsNotDrawnText proves a stream that is not
// zlib-compressed, one cut short mid-inflate, a content line that shows
// no text, and a stream with no "endstream" are each passed over rather
// than failing the read.
func TestText_SkipsWhatIsNotDrawnText(t *testing.T) {
	var drawn bytes.Buffer
	w := zlib.NewWriter(&drawn)
	if _, err := w.Write([]byte("BT 0 0 Td\n(kept)Tj\nET")); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("compress: %v", err)
	}
	truncated := drawn.Bytes()[:2] // a zlib header and nothing after it

	var doc bytes.Buffer
	doc.WriteString("stream\nnot zlib at all\nendstream\n")
	doc.WriteString("stream\n")
	doc.Write(truncated)
	doc.WriteString("endstream\n")
	doc.WriteString("stream\n")
	doc.Write(drawn.Bytes())
	doc.WriteString("endstream\n")
	doc.WriteString("stream\nnever closed")

	if got := pdftext.Text(doc.Bytes()); got != "kept" {
		t.Fatalf("Text = %q, want %q", got, "kept")
	}
	if strings.Contains(pdftext.Text(nil), "kept") {
		t.Fatal("Text of no bytes drew something")
	}
}
