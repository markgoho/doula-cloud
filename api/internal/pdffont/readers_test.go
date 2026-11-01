package pdffont_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"doula-cloud/api/internal/pdffont"
)

// The #2549 cases, each a paragraph or a few, as a Contract or a Birth
// Plan prints them.
var readerCases = map[string][]string{
	"bracket":   {"Signed electronically by: سارة (سارة) [الحداد]"},
	"heart":     {"Name: سارة ❤️ الحداد"},
	"baby":      {"Signed electronically by: سارة 👶 الحداد"},
	"brackets2": {"سارة (سارة) الحداد"},
	"wrapped":   {strings.TrimSpace(strings.Repeat("هذه الاتفاقية بين دولا المدينة وسارة الحداد. ", 5))},
	"marks":     {"بِسمِ سارة", "Signed electronically by: بِسمِ سارة"},
	"digits":    {"شارع 12 رقم 3", "العنوان: 12 Main St, شقة 4"},
	"mixedname": {"This agreement is between Rooted Birth Collective and سارة الحداد for doula services."},
	"plan":      {"خطة الولادة ❤️", "Doula: Lena (سارة)", "Practice: دولا [المدينة]", "في المنزل 12 34 (مع زوجي) 👶"},
	"latin":     {"Signed electronically by: Ada Lovelace", "प्रिया शर्मा", babyLine},
}

const pdfPath = "{pdf}"

// reader runs an outside PDF reader on pdf, named in args by pdfPath.
// Where the reader is not installed the test skips, except in CI, which
// installs both.
func reader(t *testing.T, pdf []byte, name string, args ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.pdf")
	if err := os.WriteFile(path, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	args = slices.Clone(args)
	for i, a := range args {
		if a == pdfPath {
			args[i] = path
		}
	}
	out, err := exec.CommandContext(t.Context(), name, args...).Output() //nolint:gosec // G204: name and args are this file's own constants and a temp path
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s %v: %v", name, args, err)
		}
		t.Skipf("%s is not available here: %v", name, err)
	}
	return string(out)
}

func readerText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r >= '\u202a' && r <= '\u202e' || r == '\f' {
			return -1
		}
		return r
	}, norm.NFC.String(s))
	return strings.Join(strings.Fields(s), " ")
}

var arabicWord = regexp.MustCompile(`\p{Arabic}+`)

// arabicWords is every run of Arabic letters, marks left out, sorted.
func arabicWords(s string) []string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFC.String(s))
	words := arabicWord.FindAllString(s, -1)
	slices.Sort(words)
	return words
}

// #2549: PyMuPDF, the reader the layer is laid out for, copies every case
// out as typed, and poppler keeps each Arabic word's letters together and
// in order. The package doc names what poppler, pdf.js and PDFium still
// get wrong.
func TestParagraph_CopiesOutInPyMuPDFAndPoppler(t *testing.T) {
	for name, lines := range readerCases {
		t.Run(name, func(t *testing.T) {
			out := render(t, func(d *pdffont.Doc) {
				for _, l := range lines {
					d.Paragraph(pdffont.Regular, 12, 8, l)
				}
			})
			want := readerText(strings.Join(lines, "\n"))
			mupdf := reader(t, out, "python3", "-I", "-c", "import sys, pymupdf; print(''.join(p.get_text() for p in pymupdf.open(sys.argv[1])))", pdfPath)
			if got := readerText(mupdf); got != want {
				t.Errorf("PyMuPDF = %q, want %q", got, want)
			}
			poppler := reader(t, out, "pdftotext", "-enc", "UTF-8", "-q", pdfPath, "-")
			if got, want := arabicWords(poppler), arabicWords(want); !slices.Equal(got, want) {
				t.Errorf("poppler's Arabic words = %q, want %q (poppler read %q)", got, want, readerText(poppler))
			}
		})
	}
}
