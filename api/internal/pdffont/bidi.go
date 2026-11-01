package pdffont

import (
	"slices"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/bidi"
)

// bidiLevels is the Unicode Bidirectional Algorithm's embedding level for
// each rune of a paragraph, and the paragraph's own level: 1 when its
// first letter is right-to-left, 0 otherwise.
func bidiLevels(text string) ([]bidi.Level, bidi.Level) {
	base, direction := bidi.Level(0), bidi.LeftToRight
	for _, r := range text {
		if isRightToLeft(r) {
			base, direction = 1, bidi.RightToLeft
			break
		}
		if unicode.IsLetter(r) {
			break
		}
	}
	runes := []rune(text)
	levels := make([]bidi.Level, len(runes))
	var p bidi.Paragraph
	segments := p.Segment(runes, direction)
	for i := range segments.NumRuns() {
		r := segments.Run(i)
		for j := r.Start; j < r.End; j++ {
			levels[j] = r.Level
		}
	}
	return levels, base
}

func isRightToLeft(r rune) bool {
	return unicode.In(r, unicode.Arabic, unicode.Hebrew, unicode.Syriac, unicode.Thaana, unicode.Nko)
}

// segment is a piece of a line in one face at one embedding level.
type segment struct {
	face  int
	text  string
	level bidi.Level
	width float64
}

// flush draws one line. Its runs are cut where the embedding level
// changes and joined where neighbors share a face and a level, so a
// shaped run is shaped whole; the trailing space is dropped (rule L1);
// and the pieces are reordered for reading (rule L2), each right-to-left
// piece drawn in its own order reversed. A right-to-left paragraph's line
// is set against the right margin.
func (d *Doc) flush(line []run, levels []bidi.Level, base bidi.Level, style Style, size, lineHeight float64) {
	segments := cut(line, levels)
	for len(segments) > 0 {
		last := &segments[len(segments)-1]
		last.text = strings.TrimRight(last.text, " ")
		if last.text != "" {
			break
		}
		segments = segments[:len(segments)-1]
	}
	reorder(segments)
	if len(segments) == 0 {
		return
	}
	// The page break fpdf's CellFormat takes for itself, taken first, so a
	// shaped segment's outlines and its text layer land on one page.
	auto, breakMargin := d.GetAutoPageBreak()
	_, pageHeight := d.GetPageSize()
	if auto && d.GetY()+lineHeight > pageHeight-breakMargin {
		x := d.GetX()
		d.AddPage()
		d.SetX(x)
	}

	var total float64
	for i := range segments {
		segments[i].width = d.width(segments[i].face, style, size, segments[i].text)
		total += segments[i].width
	}
	if base%2 == 1 {
		_, _, right, _ := d.GetMargins()
		pageWidth, _ := d.GetPageSize()
		d.SetX(pageWidth - right - total)
	}
	for _, s := range segments {
		rtl := s.level%2 == 1
		if faces[s.face].script != nil {
			d.drawShaped(s.face, style, size, lineHeight, s.text, rtl)
			continue
		}
		text := s.text
		if rtl {
			text = visualOrder(text)
		}
		d.use(s.face, style, size)
		d.cell(text, s.width, lineHeight)
	}
}

// cut splits a line's runs into segments of one face and one level.
func cut(line []run, levels []bidi.Level) []segment {
	var out []segment
	for _, r := range line {
		index := r.start
		for _, c := range r.text {
			level := levels[index]
			index++
			if n := len(out); n > 0 && out[n-1].face == r.face && out[n-1].level == level {
				out[n-1].text += string(c)
				continue
			}
			out = append(out, segment{face: r.face, text: string(c), level: level})
		}
	}
	return out
}

// reorder is rule L2: from the highest level down to the lowest odd one,
// every stretch of segments at that level or higher is reversed.
func reorder(segments []segment) {
	if len(segments) == 0 {
		return
	}
	highest, lowestOdd := bidi.Level(0), bidi.Level(127)
	for _, s := range segments {
		highest = max(highest, s.level)
		if s.level%2 == 1 {
			lowestOdd = min(lowestOdd, s.level)
		}
	}
	for level := highest; level >= lowestOdd && level > 0; level-- {
		for i := 0; i < len(segments); {
			if segments[i].level < level {
				i++
				continue
			}
			j := i
			for j < len(segments) && segments[j].level >= level {
				j++
			}
			slices.Reverse(segments[i:j])
			i = j
		}
	}
}
