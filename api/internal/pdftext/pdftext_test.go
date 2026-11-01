package pdftext_test

import (
	"bytes"
	"compress/zlib"
	"strings"
	"testing"

	"doula-cloud/api/internal/pdffont"
	"doula-cloud/api/internal/pdftext"
)

// TestText_ReadsBackWhatFpdfDrew proves Text returns every line a
// pdffont document draws, in order, with fpdf's escapes for "(", ")"
// and "\" undone, and a line drawn in two faces read back as one line.
func TestText_ReadsBackWhatFpdfDrew(t *testing.T) {
	d := pdffont.New()
	d.Paragraph(pdffont.Regular, 12, 8, "Rooted (Birth) Collective")
	d.Paragraph(pdffont.Regular, 12, 8, `A back\slash`)
	d.Paragraph(pdffont.Regular, 12, 8, "Mei 王 Lin")
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	got := pdftext.Text(buf.Bytes())
	want := "Rooted (Birth) Collective\n" + `A back\slash` + "\nMei 王 Lin"
	if got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// TestText_SkipsWhatIsNotDrawnText proves a stream that is not
// zlib-compressed, one cut short mid-inflate, one that inflates to binary
// rather than drawing, and a stream with no "endstream" are each passed
// over rather than failing the read.
func TestText_SkipsWhatIsNotDrawnText(t *testing.T) {
	drawn := deflate(t, "BT 0 0 Td\n) (\x00k\x00e\x00p\x00t)Tj\nET")
	truncated := drawn[:2] // a zlib header and nothing after it

	var doc bytes.Buffer
	doc.WriteString("stream\nnot zlib at all\nendstream\n")
	doc.WriteString("stream\n")
	doc.Write(truncated)
	doc.WriteString("endstream\n")
	doc.WriteString("stream\n")
	doc.Write(deflate(t, "\x01\x02 binary (\x00x)Tj"))
	doc.WriteString("endstream\n")
	doc.WriteString("stream\n")
	doc.Write(drawn)
	doc.WriteString("endstream\n")
	doc.WriteString("stream\nnever closed")

	if got := pdftext.Text(doc.Bytes()); got != "kept" {
		t.Fatalf("Text = %q, want %q", got, "kept")
	}
	if strings.Contains(pdftext.Text(nil), "kept") {
		t.Fatal("Text of no bytes drew something")
	}
}

// TestText_ReadsAnEscapedReturnAndAStringCutShort proves fpdf's escape
// for a carriage return reads back as one, and a string with no closing
// parenthesis, which no Tj shows, draws nothing.
func TestText_ReadsAnEscapedReturnAndAStringCutShort(t *testing.T) {
	doc := append([]byte("stream\n"), deflate(t, "BT 0 9 Td (\x00a\x00\\r)Tj ET BT 0 8 Td (\x00c\\")...)
	doc = append(doc, []byte("endstream\n")...)
	if got, want := pdftext.Text(doc), "a\r"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// #2544: inside a span with an ActualText, the span's first Tj reads as
// the ActualText and its others as nothing; a span with other properties
// reads its own strings; a hexadecimal string that is not hexadecimal
// reads as nothing, and one never closed ends the stream's text.
func TestText_ReadsAnActualTextInPlaceOfItsStrings(t *testing.T) {
	stream := func(content string) []byte {
		doc := append([]byte("stream\n"), deflate(t, content)...)
		return append(doc, []byte("endstream\n")...)
	}
	for name, tc := range map[string]struct{ content, want string }{
		"ActualText": {"BT 0 9 Td (\x00a)Tj ET /Span <</ActualText <FEFFD83DDC76>>> BDC BT 9 9 Td (\xff\xfd)Tj (\xff\xfd)Tj ET EMC BT 18 9 Td (\x00b)Tj ET", "a\U0001F476b"},
		"other span": {"/OC <</Name /x>> BDC BT 0 9 Td (\x00c)Tj ET EMC", "c"},
		"bad hex":    {"/Span <</ActualText <ZZ>>> BDC BT 0 9 Td (\x00d)Tj ET EMC", ""},
		"open hex":   {"BT 0 9 Td (\x00e)Tj ET <FEFF", ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := pdftext.Text(stream(tc.content)); got != tc.want {
				t.Fatalf("Text = %q, want %q", got, tc.want)
			}
		})
	}
}

func deflate(t *testing.T, content string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zlib.NewWriter(&out)
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("compress: %v", err)
	}
	return out.Bytes()
}
