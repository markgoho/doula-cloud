package pdffont

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/bidi"
	ubidi "golang.org/x/text/unicode/bidi"
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
	x     float64
	order int
}

// flush draws one line. Its runs are cut where the embedding level
// changes and joined where neighbors share a face and a level, so a
// shaped run is shaped whole; the trailing space is dropped (rule L1);
// and the pieces are reordered for reading (rule L2), each right-to-left
// piece shaped right to left, with its brackets mirrored (rule L4). A
// right-to-left paragraph's line is set against the right margin. A line
// holding right-to-left text is drawn as outlines, with its text laid
// over it by typedLayer.
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

	mixed := slices.ContainsFunc(segments, func(s segment) bool { return s.level%2 == 1 })
	var total float64
	for i := range segments {
		s := &segments[i]
		if mixed {
			s.width = d.shapedWidth(s.face, style, size, s.text, s.level%2 == 1)
		} else {
			s.width = d.drawnWidth(s.face, style, size, s.text, false)
		}
		total += s.width
	}
	x := d.GetX()
	if base%2 == 1 {
		_, _, right, _ := d.GetMargins()
		pageWidth, _ := d.GetPageSize()
		x = pageWidth - right - total
	}
	for i := range segments {
		segments[i].x = x
		x += segments[i].width
	}
	for _, s := range segments {
		d.SetX(s.x)
		switch {
		case mixed:
			d.drawOutlines(s.face, style, size, lineHeight, s.text, s.level%2 == 1)
		case outlined(s.face, s.text, false):
			d.drawShaped(s.face, style, size, lineHeight, s.text)
		default:
			d.use(s.face, style, size)
			d.cell(s.text, s.width, lineHeight)
		}
	}
	if mixed {
		slices.SortFunc(segments, func(a, b segment) int { return a.order - b.order })
		d.typedLayer(segments, style, size, lineHeight)
	}
	d.SetX(x)
}

// typedLayer lays a line holding right-to-left text over its outlines,
// invisibly and in the order it was typed (#2546). A reader copying a line
// stored in drawn order rebuilds the typed order itself, and gets it wrong
// around a bracket that ends a right-to-left run (rule BD16) or a neutral
// character, such as a space or an emoji, that a reader sees as starting
// a run of its own.
//
// Each strong right-to-left character (bidi class R, AL or AN), with the
// marks after it, is placed one advance to the left of the one typed before
// it, which a reader takes as text written in logical order. Every other
// stretch is written left to right in one piece, so it never starts a run
// of its own.
func (d *Doc) typedLayer(segments []segment, style Style, size, lineHeight float64) {
	type char struct {
		r           rune
		face        int
		stretch     float64
		left, right float64
	}
	var chars []char
	for _, s := range segments {
		face := layerFace(s.face)
		d.use(face, style, size)
		stretch := 1.0
		if natural := d.GetStringWidth(shownText(s.text)); natural > 0 && s.width > 0 {
			stretch = s.width / natural
		}
		left, right := s.x, s.x+s.width
		for _, r := range s.text {
			w := d.GetStringWidth(shownText(string(r))) * stretch
			if s.level%2 == 1 {
				chars = append(chars, char{r, face, stretch, right - w, right})
				right -= w
			} else {
				chars = append(chars, char{r, face, stretch, left, left + w})
				left += w
			}
		}
	}

	d.RawWriteStr("3 Tr")
	scale := 0.0
	setScale := func(s float64) {
		if s != scale {
			d.RawWriteStr(fmt.Sprintf("%.3f Tz", 100*s))
			scale = s
		}
	}
	var x float64
	chained := false
	for i := 0; i < len(chars); {
		j := i + 1
		if strongRightToLeft(chars[i].r) {
			for j < len(chars) && unicode.Is(unicode.Mn, chars[j].r) {
				j++
			}
			c := chars[i]
			// Font size, not Tz, takes the stretch: MuPDF measures a
			// Tz-scaled step by its root and poppler by Tz (#2549).
			setScale(1)
			d.use(c.face, style, size*c.stretch)
			w := d.GetStringWidth(shownText(string(c.r)))
			if !chained {
				x = c.right
			}
			x -= w
			var b strings.Builder
			for _, m := range chars[i:j] {
				b.WriteRune(m.r)
			}
			// fpdf sets a cell's baseline by its font size; this keeps the
			// line's own.
			y := d.GetY()
			d.SetXY(x, y+0.3*size*(1-c.stretch)/d.GetConversionRatio())
			d.layer(b.String(), w, lineHeight, scale)
			d.SetY(y)
			chained = true
			i = j
			continue
		}
		for j < len(chars) && !strongRightToLeft(chars[j].r) {
			j++
		}
		stretch := chars[i:j]
		setScale(stretch[0].stretch)
		x = stretch[0].left
		for _, c := range stretch {
			x = min(x, c.left)
		}
		for k := 0; k < len(stretch); {
			l := k + 1
			for l < len(stretch) && stretch[l].face == stretch[k].face && !outsideBMP(stretch[l].r) && !outsideBMP(stretch[l-1].r) {
				l++
			}
			var b strings.Builder
			for _, c := range stretch[k:l] {
				b.WriteRune(c.r)
			}
			d.use(stretch[k].face, style, size)
			w := d.GetStringWidth(shownText(b.String())) * scale
			d.SetX(x)
			d.layer(b.String(), w, lineHeight, scale)
			x += w
			k = l
		}
		chained = false
		i = j
	}
	d.RawWriteStr("0 Tr 100 Tz")
}

func strongRightToLeft(r rune) bool {
	p, _ := ubidi.LookupRune(r)
	return slices.Contains([]ubidi.Class{ubidi.R, ubidi.AL, ubidi.AN}, p.Class())
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
			out = append(out, segment{face: r.face, text: string(c), level: level, order: len(out)})
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
