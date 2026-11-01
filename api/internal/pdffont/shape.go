package pdffont

import (
	"bytes"
	"fmt"
	"slices"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// script is what a shaped face is shaped as. rtl is its own direction,
// which the line's embedding level confirms or overrides when it is drawn.
type script struct {
	rtl      bool
	script   language.Script
	language language.Language
}

var (
	arabic     = script{rtl: true, script: language.Arabic, language: language.NewLanguage("ar")}
	devanagari = script{script: language.Devanagari, language: language.NewLanguage("hi")}
)

// parsed holds each shaped face's file parsed once per process. A
// font.Font is read-only once parsed; the font.Face that wraps it carries
// caches, so each Doc makes its own.
var parsed sync.Map

type parsedFont struct {
	once sync.Once
	font *font.Font
}

func parsedFace(name string) *font.Font {
	entry, _ := parsed.LoadOrStore(name, &parsedFont{})
	p := entry.(*parsedFont)
	p.once.Do(func() {
		f, err := font.ParseTTF(bytes.NewReader(mustRead(name)))
		if err != nil {
			// coverage:ignore reason: the embedded fonts are fixed at build time and parse, which every test exercises
			panic("pdffont: parse " + name + " for shaping: " + err.Error())
		}
		p.font = f.Font
	})
	return p.font
}

type shapingFace struct {
	face *font.Face
	upem float64
}

func (d *Doc) shapingFace(i int, style Style) *shapingFace {
	name := faces[i].file(style)
	if sf, ok := d.shapers[name]; ok {
		return sf
	}
	f := font.NewFace(parsedFace(name))
	sf := &shapingFace{face: f, upem: float64(f.Upem())}
	d.shapers[name] = sf
	return sf
}

// shape runs text through HarfBuzz in face i, right to left when rtl. The
// glyphs come back in visual order, left to right, with advances and
// offsets in font units.
func (d *Doc) shape(i int, style Style, text string, rtl bool) (*shapingFace, shaping.Output) {
	sf := d.shapingFace(i, style)
	s := faces[i].script
	direction := di.DirectionLTR
	if rtl {
		direction = di.DirectionRTL
	}
	runes := []rune(text)
	return sf, d.shaper.Shape(shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: direction,
		Face:      sf.face,
		Size:      fixed.I(int(sf.upem)),
		Script:    s.script,
		Language:  s.language,
	})
}

// unitsPerPoint is the Doc's own length per font unit at size points.
func (d *Doc) unitsPerFontUnit(sf *shapingFace, size float64) float64 {
	return size / d.GetConversionRatio() / sf.upem
}

func (d *Doc) shapedWidth(i int, style Style, size float64, text string) float64 {
	sf, out := d.shape(i, style, text, faces[i].script.rtl)
	return fromFixed(out.Advance) * d.unitsPerFontUnit(sf, size)
}

func fromFixed(v fixed.Int26_6) float64 { return float64(v) / 64 }

// drawShaped draws text in face i as its shaped glyph outlines, filled in
// the text color on the baseline fpdf would give a cell on this line, then
// lays the same text over it invisibly (text render mode 3), stretched to
// the drawn width, so the PDF still holds the characters typed.
func (d *Doc) drawShaped(i int, style Style, size, lineHeight float64, text string, rtl bool) {
	sf, out := d.shape(i, style, text, rtl)
	scale := d.unitsPerFontUnit(sf, size)
	x, y := d.GetX(), d.GetY()
	baseline := y + 0.5*lineHeight + 0.3*size/d.GetConversionRatio()

	r, g, b := d.GetTextColor()
	fr, fg, fb := d.GetFillColor()
	d.SetFillColor(r, g, b)
	pen, drawn := x, false
	for _, glyph := range out.Glyphs {
		gx := pen + fromFixed(glyph.XOffset)*scale
		gy := baseline - fromFixed(glyph.YOffset)*scale
		if outline, ok := sf.face.GlyphData(glyph.GlyphID).(font.GlyphOutline); ok {
			drawn = d.outline(outline, gx, gy, scale) || drawn
		}
		pen += fromFixed(glyph.Advance) * scale
	}
	if drawn {
		d.DrawPath("F")
	}
	d.SetFillColor(fr, fg, fb)
	// fpdf's path calls move its current position to the last point drawn.
	d.SetXY(x, y)

	width := pen - x
	stored := text
	if rtl {
		stored = visualOrder(text)
	}
	d.use(i, style, size)
	natural := d.GetStringWidth(stored)
	if natural <= 0 {
		d.SetX(x + width)
		return
	}
	d.RawWriteStr(fmt.Sprintf("3 Tr %.3f Tz\n", 100*max(width, zeroWidth)/natural))
	d.cell(stored, width, lineHeight)
	d.RawWriteStr("0 Tr 100 Tz\n")
}

// outline adds one glyph's contours to the current path, its origin at
// x, y, and reports whether it had any.
func (d *Doc) outline(o font.GlyphOutline, x, y, scale float64) bool {
	at := func(p ot.SegmentPoint) (float64, float64) {
		return x + float64(p.X)*scale, y - float64(p.Y)*scale
	}
	for _, seg := range o.Segments {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			d.MoveTo(at(seg.Args[0]))
		case ot.SegmentOpLineTo:
			d.LineTo(at(seg.Args[0]))
		case ot.SegmentOpQuadTo:
			cx, cy := at(seg.Args[0])
			ex, ey := at(seg.Args[1])
			d.CurveTo(cx, cy, ex, ey)
		case ot.SegmentOpCubeTo:
			// coverage:ignore reason: the shaped faces are TrueType, whose outlines are quadratic; a cubic segment comes only from a CFF face
			c1x, c1y := at(seg.Args[0])
			c2x, c2y := at(seg.Args[1])
			ex, ey := at(seg.Args[2])
			d.CurveBezierCubicTo(c1x, c1y, c2x, c2y, ex, ey)
		}
	}
	return len(o.Segments) > 0
}

// visualOrder is a right-to-left run as it reads on the page, left to
// right: each letter with its marks, in reverse. A PDF holds text in the
// order it is drawn, and a reader reverses a right-to-left run back to
// the order it was typed when the text is copied or searched.
func visualOrder(text string) string {
	clusters := marked(text)
	slices.Reverse(clusters)
	var b bytes.Buffer
	for _, c := range clusters {
		b.WriteString(c)
	}
	return b.String()
}

// marked splits text into each letter and the marks that follow it.
func marked(text string) []string {
	var out []string
	for _, r := range text {
		if n := len(out); n > 0 && unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc) {
			out[n-1] += string(r)
			continue
		}
		out = append(out, string(r))
	}
	return out
}
