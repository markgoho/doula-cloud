package pdffont_test

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"doula-cloud/api/internal/pdffont"
	"doula-cloud/api/internal/pdftext"
)

func render(t *testing.T, write func(d *pdffont.Doc)) []byte {
	t.Helper()
	d := pdffont.New()
	write(d)
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.Bytes()
}

// #2359: a name in any of these scripts prints as the characters typed.
var names = []struct{ script, name string }{
	{"Vietnamese", "Nguyễn Thị Hương"},
	{"Polish", "Łucja Żółkiewska-Wróbel"},
	{"Cyrillic", "Анна Петрова"},
	{"Greek", "Ελένη Παπαδοπούλου"},
	{"Arabic", "سارة الحداد"},
	{"Chinese", "王小明"},
	{"Japanese", "さくら 山田"},
	{"Korean", "김지수"},
	{"Devanagari", "प्रिया शर्मा"},
	{"a combining mark", "Zoë Brontë"},
	{"Latin and CJK on one line", "Mei 王 Lin"},
}

func TestParagraph_PrintsANameInAnyScriptAsTyped(t *testing.T) {
	for _, tc := range names {
		t.Run(tc.script, func(t *testing.T) {
			for _, style := range []pdffont.Style{pdffont.Regular, pdffont.Bold} {
				out := render(t, func(d *pdffont.Doc) {
					d.Paragraph(style, 12, 8, "Signed electronically by: "+tc.name)
				})
				if got, want := pdftext.Text(out), norm.NFC.String("Signed electronically by: "+tc.name); got != want {
					t.Fatalf("style %q: Text = %q, want %q", style, got, want)
				}
			}
		})
	}
}

// fpdf writes UTF-16 with no surrogate pairs, so a character outside the
// Basic Multilingual Plane prints as U+FFFD rather than corrupting the
// line around it.
func TestParagraph_ReplacesWhatFpdfCannotEncode(t *testing.T) {
	out := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, "Ada \U0001F476 Lovelace \xff")
	})
	if got, want := pdftext.Text(out), "Ada � Lovelace �"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// A character no face has stays in the run it falls in.
func TestParagraph_KeepsACharacterNoFaceHas(t *testing.T) {
	out := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, " private use")
	})
	if got, want := pdftext.Text(out), " private use"; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
}

// Each face is subset to what the document uses, and a face it never
// needs is never embedded, so a Latin-only Contract stays small.
func TestParagraph_EmbedsOnlyWhatTheDocumentUses(t *testing.T) {
	prose := strings.Repeat("This agreement is between Rooted Birth Collective and Ada Lovelace for doula services. ", 40)
	latin := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, prose)
		d.Paragraph(pdffont.Regular, 12, 8, "Signed electronically by: Ada Lovelace")
	})
	if bytes.Contains(latin, []byte("DroidSansFallback")) || bytes.Contains(latin, []byte("NotoSansArabic")) {
		t.Fatal("a Latin-only document embedded a face it never used")
	}
	if len(latin) > 60_000 {
		t.Fatalf("a Latin-only Contract is %d bytes, want at most 60000", len(latin))
	}

	var every strings.Builder
	for _, tc := range names {
		every.WriteString(tc.name + " ")
	}
	mixed := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, prose)
		d.Paragraph(pdffont.Regular, 12, 8, every.String())
	})
	if len(mixed) > 200_000 {
		t.Fatalf("a Contract naming every script is %d bytes, want at most 200000", len(mixed))
	}
	t.Logf("Latin-only: %d bytes; every script: %d bytes", len(latin), len(mixed))
}

// Prose wraps at the right margin between words, with no space opening a
// line; a newline starts a line; a word longer than a whole line is broken
// between characters.
func TestParagraph_WrapsBetweenWords(t *testing.T) {
	sentence := strings.TrimSpace(strings.Repeat("Continuous labor support at home and in hospital. ", 6))
	long := strings.Repeat("x", 200)
	out := render(t, func(d *pdffont.Doc) {
		d.Paragraph(pdffont.Regular, 12, 8, sentence+"\nSecond line\n"+long)
	})
	text := pdftext.Text(out)
	lines := strings.Split(text, "\n")
	if len(lines) < 5 {
		t.Fatalf("lines = %q, want the sentence wrapped, a second line and the long word broken", lines)
	}
	squeeze := func(s string) string { return strings.ReplaceAll(s, " ", "") }
	if got, want := squeeze(strings.Join(lines, "")), squeeze(sentence+"Second line"+long); got != want {
		t.Fatalf("text = %q, want every character kept in order", got)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, " ") {
			t.Fatalf("line %q opens with a space", line)
		}
	}
	if !strings.Contains(text, "\nSecond line\n") {
		t.Fatalf("text = %q, want the newline kept as a line of its own", text)
	}
}
