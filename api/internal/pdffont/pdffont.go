// Package pdffont is the one way this codebase writes text into a PDF, so a
// name or prose in any script prints as the characters typed (#2359).
// fpdf's core fonts are cp1252 and cannot print Arabic, Cyrillic, CJK or
// Devanagari at all, so text goes through embedded Unicode fonts instead.
//
// No one font covers every script a Client's name may be written in, so
// a line is split into runs, each drawn in the first face that has a
// glyph for it, and the runs are laid out one after another. fpdf subsets
// each face to the characters a document uses, so a typical Contract
// carries a few kilobytes of each face it touches and nothing of the rest.
//
// fpdf draws each character's own glyph in the order typed and does no
// text shaping, which Arabic and Devanagari need: Arabic joins its letters
// and runs right to left, and Devanagari forms conjuncts and moves its
// short-i sign before its consonant. A run in either face is shaped
// instead (shape.go, #2538) and drawn as its glyph outlines, with the same
// text laid invisibly over it so it still copies out and searches as
// typed. Emoji are drawn the same way, in a monochrome face (#2544).
//
// The faces, and the license each is embedded and redistributed under,
// sit in fonts/ beside their license text:
//
//   - Noto Sans (Latin, Greek, Cyrillic, Vietnamese), Noto Sans Arabic,
//     Noto Sans Devanagari and Noto Emoji: SIL Open Font License 1.1,
//     which permits embedding, subsetting and redistribution, bundled or
//     not. Noto Emoji is the variable font's regular instance with its
//     vertical metrics and glyph names dropped (fonttools varLib.instancer
//     wght=400, then pyftsubset --unicodes='*' --layout-features='*'
//     --no-hinting --name-IDs='*' --drop-tables+=vhea,vmtx,STAT).
//   - Droid Sans Fallback (Chinese and Japanese): Apache License 2.0, with
//     its NOTICE.
//   - Nanum Gothic (Korean): SIL Open Font License 1.1. Its Reserved Font
//     Names bind a modified font distributed as a font; a subset embedded
//     in a document is not one (the OFL FAQ, on embedding a font in a
//     document).
//
// Shaping is github.com/go-text/typesetting, a Go port of HarfBuzz, under
// the Unlicense or the BSD 3-Clause License.
package pdffont

import (
	"embed"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
	"github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/norm"
)

//go:embed fonts/*.ttf
var fontFiles embed.FS

// face is one embedded typeface, as the fpdf family it is registered as.
// A face with a script is shaped rather than drawn character by character.
// A face that is outlines only is never registered with fpdf, whose font
// parser reads only a face's BMP characters: its invisible text is set in
// the first face.
type face struct {
	family       string
	regular      string
	bold         string
	script       *script
	outlinesOnly bool
}

// faces in the order a character is looked for: the first that has a
// glyph for it draws it.
var faces = []face{
	{family: "NotoSans", regular: "fonts/NotoSans-Regular.ttf", bold: "fonts/NotoSans-Bold.ttf"},
	{family: "NotoSansArabic", regular: "fonts/NotoSansArabic-Regular.ttf", bold: "fonts/NotoSansArabic-Bold.ttf", script: &arabic},
	{family: "NotoSansDevanagari", regular: "fonts/NotoSansDevanagari-Regular.ttf", bold: "fonts/NotoSansDevanagari-Bold.ttf", script: &devanagari},
	// One weight only: bold CJK prints in the regular weight.
	{family: "DroidSansFallback", regular: "fonts/DroidSansFallbackFull.ttf", bold: "fonts/DroidSansFallbackFull.ttf"},
	// Hangul, which Droid Sans Fallback does not have.
	{family: "NanumGothic", regular: "fonts/NanumGothic-Regular.ttf", bold: "fonts/NanumGothic-Bold.ttf"},
	// Last, since it also has digits, "#" and "©". One weight only.
	{family: "NotoEmoji", regular: "fonts/NotoEmoji-Regular.ttf", bold: "fonts/NotoEmoji-Regular.ttf", script: &common, outlinesOnly: true},
}

type coverageFont struct {
	once sync.Once
	font *sfnt.Font
}

// coverage is each face parsed the first time a character is looked for
// in it, so a document that never reaches the emoji face never parses it.
var coverage = make([]coverageFont, len(faces))

// covers reports whether face i has a glyph for r.
func covers(i int, r rune) bool {
	c := &coverage[i]
	c.once.Do(func() { c.font = mustParse(faces[i].regular) })
	var buf sfnt.Buffer
	index, err := c.font.GlyphIndex(&buf, r)
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

func (f face) file(style Style) string {
	if style == Bold {
		return f.bold
	}
	return f.regular
}

// Doc is an A4 portrait PDF that writes text through the embedded faces.
type Doc struct {
	*fpdf.Fpdf
	registered map[string]bool
	shaper     shaping.HarfbuzzShaper
	shapers    map[string]*shapingFace
}

// New starts a Doc with its first page added. Cells carry no inner
// margin, so the runs of one line sit flush against each other.
func New() *Doc {
	d := &Doc{Fpdf: fpdf.New("P", "mm", "A4", ""), registered: map[string]bool{}, shapers: map[string]*shapingFace{}}
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
// a glyph wider than the page and breaks the line at it. Each piece is
// measured (GetStringWidth, or its shaped width) and drawn at exactly that
// width. A line is wrapped in the order it was typed and gathered whole
// before it is drawn, then its pieces are put in reading order by the
// Unicode Bidirectional Algorithm (bidi.go), so right-to-left words read
// right to left. A paragraph whose first letter is right-to-left reads
// right to left as a whole and is set against the right margin.
func (d *Doc) Paragraph(style Style, size, lineHeight float64, text string) {
	for line := range strings.SplitSeq(printable(text), "\n") {
		levels, base := bidiLevels(line)
		d.layout(words(runs(line)), levels, base, style, size, lineHeight)
		d.Ln(lineHeight)
	}
}

// layout places words on lines, moving to a new line before a word that
// would cross the right margin, so a line breaks only at a space and a
// word set in two faces (a name, then its full stop) stays whole. A space
// that would open a line is dropped, and a word wider than a whole line
// is broken between characters.
func (d *Doc) layout(ws [][]run, levels []bidi.Level, base bidi.Level, style Style, size, lineHeight float64) {
	left, _, right, _ := d.GetMargins()
	pageWidth, _ := d.GetPageSize()
	limit := pageWidth - right
	var line []run
	x := d.GetX()
	newLine := func() {
		d.flush(line, levels, base, style, size, lineHeight)
		d.Ln(lineHeight)
		line, x = nil, left
	}
	for _, word := range ws {
		width := d.wordWidth(word, style, size)
		if x+width > limit && x > left {
			newLine()
			trimmed := strings.TrimLeft(word[0].text, " ")
			word[0].start += utf8.RuneCountInString(word[0].text) - utf8.RuneCountInString(trimmed)
			word[0].text = trimmed
			width = d.wordWidth(word, style, size)
		}
		if x+width <= limit {
			line = append(line, word...)
			x += width
			continue
		}
		for _, r := range word {
			var b strings.Builder
			start := r.start
			for _, c := range r.text {
				next := b.String() + string(c)
				if b.Len() > 0 && x+d.width(r.face, style, size, next) > limit {
					line = append(line, run{face: r.face, text: b.String(), start: start})
					start += utf8.RuneCountInString(b.String())
					newLine()
					b.Reset()
				}
				b.WriteRune(c)
			}
			line = append(line, run{face: r.face, text: b.String(), start: start})
			x += d.width(r.face, style, size, b.String())
		}
	}
	d.flush(line, levels, base, style, size, lineHeight)
}

func (d *Doc) wordWidth(word []run, style Style, size float64) float64 {
	var width float64
	for _, r := range word {
		width += d.width(r.face, style, size, r.text)
	}
	return width
}

// width is how wide text draws in face i, in the face's own direction.
func (d *Doc) width(i int, style Style, size float64, text string) float64 {
	rtl := faces[i].script != nil && faces[i].script.rtl
	return d.drawnWidth(i, style, size, text, rtl)
}

// drawnWidth is how wide text draws in face i: shaped where it is drawn
// as outlines, fpdf's own measure otherwise.
func (d *Doc) drawnWidth(i int, style Style, size float64, text string, rtl bool) float64 {
	if outlined(i, text, rtl) {
		return d.shapedWidth(i, style, size, text, rtl)
	}
	d.use(i, style, size)
	return d.GetStringWidth(text)
}

// outlined reports whether text in face i is drawn as shaped outlines
// rather than as fpdf text: in a shaped face; right to left, so HarfBuzz
// mirrors a bracket by the Unicode Bidi Mirroring property; or with a
// character outside the BMP, which fpdf cannot encode.
func outlined(i int, text string, rtl bool) bool {
	return faces[i].script != nil || rtl || strings.ContainsFunc(text, outsideBMP)
}

func outsideBMP(r rune) bool { return r > 0xFFFF }

// cell draws text at the current position in a cell width wide. fpdf
// reads a cell width of 0 as "to the right margin", so text that is only
// a zero-width mark is drawn in a cell barely wider.
func (d *Doc) cell(text string, width, lineHeight float64) {
	if text == "" {
		return
	}
	d.CellFormat(max(width, zeroWidth), lineHeight, text, "", 0, "", false, 0, "")
}

const zeroWidth = 1e-6

// words groups runs into the stretches a line may not break inside: each
// ends after a space, and may span runs in more than one face.
func words(rs []run) [][]run {
	var out [][]run
	var word []run
	for _, r := range rs {
		start := r.start
		for _, piece := range strings.SplitAfter(r.text, " ") {
			if piece == "" {
				continue
			}
			word = append(word, run{face: r.face, text: piece, start: start})
			start += utf8.RuneCountInString(piece)
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
		d.AddUTF8FontFromBytes(f.family, string(style), mustRead(f.file(style)))
		d.registered[key] = true
	}
	d.SetFont(f.family, string(style), size)
}

// run is text one face draws, and where it starts in its paragraph, in
// runes.
type run struct {
	face  int
	text  string
	start int
}

// runs splits text into the longest stretches one face can draw. Each
// character goes to the first face that has it, so a Latin word after a
// Korean one is back in Noto Sans; a space, a combining mark or a format
// character (the joiner inside an emoji sequence) stays in the face
// drawing the run it follows, when that face has it, so it sits beside
// the letters around it. A space after an emoji is the exception: the
// emoji face's space is an emoji wide. A character followed by U+FE0F,
// which asks for its emoji form ("1️⃣", "©️"), goes to the emoji face. A
// character no face has stays where it is and prints as that face's
// missing glyph.
func runs(text string) []run {
	var out []run
	var b strings.Builder
	current, start := -1, 0
	runes := []rune(text)
	for index, r := range runes {
		next := current
		switch {
		case index+1 < len(runes) && runes[index+1] == emojiPresentation && covers(emojiFace, r):
			next = emojiFace
		case current < 0 || !stays(current, r):
			next = firstFace(r, current)
		}
		if next != current && b.Len() > 0 {
			out = append(out, run{face: current, text: b.String(), start: start})
			b.Reset()
			start = index
		}
		current = next
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, run{face: current, text: b.String(), start: start})
	}
	return out
}

const emojiPresentation = '️'

var emojiFace = len(faces) - 1

func stays(current int, r rune) bool {
	space := unicode.IsSpace(r) && !faces[current].outlinesOnly
	return (space || unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf)) && covers(current, r)
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
// ("ë") rather than as a mark fpdf cannot place over its letter. Invalid
// UTF-8 prints as U+FFFD.
func printable(text string) string {
	return strings.ToValidUTF8(norm.NFC.String(text), string(utf8.RuneError))
}
