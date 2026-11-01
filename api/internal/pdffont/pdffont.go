// Package pdffont is the one way this codebase writes text into a PDF, so a
// name or prose in any script prints as the characters typed (#2359).
// fpdf's core fonts are cp1252 and cannot print Arabic, Cyrillic, CJK or
// Devanagari at all, so text goes through embedded Unicode fonts instead.
//
// No one font covers every script a Client's name may be written in, so
// a line is split into runs, each drawn in the first face that has a
// glyph for it, and fpdf's flowing Write lays the runs out one after
// another. fpdf subsets each face to the characters a document uses, so a
// typical Contract carries a few kilobytes of each face it touches and
// nothing of the rest.
//
// The faces, and the license each is embedded and redistributed under,
// sit in fonts/ beside their license text:
//
//   - Noto Sans (Latin, Greek, Cyrillic, Vietnamese), Noto Sans Arabic and
//     Noto Sans Devanagari: SIL Open Font License 1.1, which permits
//     embedding, subsetting and redistribution, bundled or not.
//   - Droid Sans Fallback (Chinese and Japanese): Apache License 2.0, with
//     its NOTICE.
//   - Nanum Gothic (Korean): SIL Open Font License 1.1. Its Reserved Font
//     Names bind a modified font distributed as a font; a subset embedded
//     in a document is not one (the OFL FAQ, 1.x on embedding).
//
// fpdf does not shape text: Arabic prints in its isolated letter forms,
// left to right, and Devanagari without its conjuncts. Every character is
// a real glyph rather than a "?", and a reader that copies the text gets
// the characters typed; shaping is #2538.
package pdffont

import (
	"embed"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/norm"
)

//go:embed fonts/*.ttf
var fontFiles embed.FS

// face is one embedded typeface, as the fpdf family it is registered as.
type face struct {
	family  string
	regular string
	bold    string
}

// faces in the order a character is looked for: the first that has a
// glyph for it draws it.
var faces = []face{
	{family: "NotoSans", regular: "fonts/NotoSans-Regular.ttf", bold: "fonts/NotoSans-Bold.ttf"},
	{family: "NotoSansArabic", regular: "fonts/NotoSansArabic-Regular.ttf", bold: "fonts/NotoSansArabic-Bold.ttf"},
	{family: "NotoSansDevanagari", regular: "fonts/NotoSansDevanagari-Regular.ttf", bold: "fonts/NotoSansDevanagari-Bold.ttf"},
	// One weight only: bold CJK prints in the regular weight.
	{family: "DroidSansFallback", regular: "fonts/DroidSansFallbackFull.ttf", bold: "fonts/DroidSansFallbackFull.ttf"},
	// Hangul, which Droid Sans Fallback does not have.
	{family: "NanumGothic", regular: "fonts/NanumGothic-Regular.ttf", bold: "fonts/NanumGothic-Bold.ttf"},
}

var (
	coverageOnce sync.Once
	coverage     []*sfnt.Font
)

// covers reports whether face i has a glyph for r.
func covers(i int, r rune) bool {
	coverageOnce.Do(func() {
		coverage = make([]*sfnt.Font, len(faces))
		for j, f := range faces {
			coverage[j] = mustParse(f.regular)
		}
	})
	var buf sfnt.Buffer
	index, err := coverage[i].GlyphIndex(&buf, r)
	return err == nil && index != 0
}

func mustParse(name string) *sfnt.Font {
	f, err := sfnt.Parse(mustRead(name))
	if err != nil {
		// coverage:ignore reason: the embedded fonts are fixed at build time and parse, which every test exercises
		panic("pdffont: parse " + name + ": " + err.Error())
	}
	return f
}

func mustRead(name string) []byte {
	b, err := fontFiles.ReadFile(name)
	if err != nil {
		// coverage:ignore reason: the go:embed pattern above holds every name in faces, so a read cannot miss
		panic("pdffont: read " + name + ": " + err.Error())
	}
	return b
}

// Style is fpdf's font style: "" for regular, "B" for bold.
type Style string

// The two styles a Doc writes in.
const (
	Regular Style = ""
	Bold    Style = "B"
)

// Doc is an A4 portrait PDF that writes text through the embedded faces.
type Doc struct {
	*fpdf.Fpdf
	registered map[string]bool
}

// New starts a Doc with its first page added. Cells carry no inner
// margin, so the runs of one line sit flush against each other.
func New() *Doc {
	d := &Doc{Fpdf: fpdf.New("P", "mm", "A4", ""), registered: map[string]bool{}}
	d.SetCellMargin(0)
	d.AddPage()
	return d
}

// Paragraph writes text at size points in style, wrapping at the right
// margin with lines lineHeight apart, and ends the line after it. A "\n"
// in text starts a new line.
//
// It lays the line out itself rather than through fpdf's Write, which
// reads a zero-width glyph (a combining mark, a Devanagari vowel sign) as
// a glyph wider than the page and breaks the line at it. GetStringWidth
// reads those widths correctly, so each piece is measured with it and
// drawn as a cell of exactly that width.
func (d *Doc) Paragraph(style Style, size, lineHeight float64, text string) {
	for line := range strings.SplitSeq(printable(text), "\n") {
		for _, word := range words(runs(line)) {
			d.place(word, style, size, lineHeight)
		}
		d.Ln(lineHeight)
	}
}

// place draws word at the current position, first moving to a new line
// when it would cross the right margin, so a line breaks only at a space
// and a word set in two faces (a name, then its full stop) stays whole. A
// space that would open a line is dropped, and a word wider than a whole
// line is broken between characters.
func (d *Doc) place(word []run, style Style, size, lineHeight float64) {
	left, _, right, _ := d.GetMargins()
	pageWidth, _ := d.GetPageSize()
	limit := pageWidth - right
	var width float64
	for _, r := range word {
		d.use(r.face, style, size)
		width += d.GetStringWidth(r.text)
	}
	if d.GetX()+width > limit && d.GetX() > left {
		d.Ln(lineHeight)
		word[0].text = strings.TrimLeft(word[0].text, " ")
	}
	for _, r := range word {
		d.use(r.face, style, size)
		piece := r.text
		if d.GetX()+d.GetStringWidth(piece) > limit {
			var b strings.Builder
			for _, c := range piece {
				if b.Len() > 0 && d.GetX()+d.GetStringWidth(b.String()+string(c)) > limit {
					d.cell(b.String(), lineHeight)
					d.Ln(lineHeight)
					b.Reset()
				}
				b.WriteRune(c)
			}
			piece = b.String()
		}
		d.cell(piece, lineHeight)
	}
}

// cell draws text at the current position in a cell exactly as wide.
// fpdf reads a cell width of 0 as "to the right margin", so text that is
// only a zero-width mark is drawn in a cell barely wider.
func (d *Doc) cell(text string, lineHeight float64) {
	if text == "" {
		return
	}
	d.CellFormat(max(d.GetStringWidth(text), zeroWidth), lineHeight, text, "", 0, "", false, 0, "")
}

const zeroWidth = 1e-6

// words groups runs into the stretches a line may not break inside: each
// ends after a space, and may span runs in more than one face.
func words(rs []run) [][]run {
	var out [][]run
	var word []run
	for _, r := range rs {
		for _, piece := range strings.SplitAfter(r.text, " ") {
			if piece == "" {
				continue
			}
			word = append(word, run{face: r.face, text: piece})
			if strings.HasSuffix(piece, " ") {
				out = append(out, word)
				word = nil
			}
		}
	}
	if len(word) > 0 {
		out = append(out, word)
	}
	return out
}

// use registers face i in style the first time a document needs it, so a
// Contract written only in Latin script never loads the CJK face.
func (d *Doc) use(i int, style Style, size float64) {
	f := faces[i]
	key := f.family + string(style)
	if !d.registered[key] {
		file := f.regular
		if style == Bold {
			file = f.bold
		}
		d.AddUTF8FontFromBytes(f.family, string(style), mustRead(file))
		d.registered[key] = true
	}
	d.SetFont(f.family, string(style), size)
}

type run struct {
	face int
	text string
}

// runs splits text into the longest stretches one face can draw. Each
// character goes to the first face that has it, so a Latin word after a
// Korean one is back in Noto Sans; a space or a combining mark stays in
// the face drawing the run it follows, when that face has it, so it sits
// beside the letters around it. A character no face has stays where it
// is and prints as that face's missing glyph.
func runs(text string) []run {
	var out []run
	var b strings.Builder
	current := -1
	for _, r := range text {
		next := current
		if current < 0 || (!unicode.IsSpace(r) && !unicode.Is(unicode.Mn, r)) || !covers(current, r) {
			next = firstFace(r, current)
		}
		if next != current && b.Len() > 0 {
			out = append(out, run{face: current, text: b.String()})
			b.Reset()
		}
		current = next
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, run{face: current, text: b.String()})
	}
	return out
}

func firstFace(r rune, fallback int) int {
	for i := range faces {
		if covers(i, r) {
			return i
		}
	}
	if fallback < 0 {
		return 0
	}
	return fallback
}

// printable is text in Unicode's composed form, so a letter typed with a
// combining mark ("e" and U+0308) prints as the one precomposed glyph
// ("ë") rather than as a mark fpdf cannot place over its letter. It also
// drops what fpdf cannot write: fpdf encodes text as UTF-16 without
// surrogate pairs, so a character outside the Basic Multilingual Plane
// (an emoji, a rare CJK ideograph) would corrupt the line. Each prints as
// U+FFFD instead, and invalid UTF-8 likewise.
func printable(text string) string {
	text = norm.NFC.String(text)
	if !strings.ContainsFunc(text, func(r rune) bool { return r > 0xFFFF || r == utf8.RuneError }) {
		return text
	}
	return strings.Map(func(r rune) rune {
		if r > 0xFFFF {
			return utf8.RuneError
		}
		return r
	}, text)
}
