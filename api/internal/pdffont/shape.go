package pdffont

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

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
	// common is what emoji, and a face with no script of its own drawn as
	// outlines, are shaped as.
	common = script{script: language.Common}
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
	if s == nil {
		s = &common
	}
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

func (d *Doc) shapedWidth(i int, style Style, size float64, text string, rtl bool) float64 {
	sf, out := d.shape(i, style, text, rtl)
	return fromFixed(out.Advance) * d.unitsPerFontUnit(sf, size)
}

func fromFixed(v fixed.Int26_6) float64 { return float64(v) / 64 }

// drawShaped draws text in face i as its shaped glyph outlines, then lays
// the same text over it invisibly (text render mode 3), stretched to the
// drawn width, so the PDF still holds the characters typed.
func (d *Doc) drawShaped(i int, style Style, size, lineHeight float64, text string) {
	x := d.GetX()
	width := d.drawOutlines(i, style, size, lineHeight, text, false)
	d.use(layerFace(i), style, size)
	natural := d.GetStringWidth(shownText(text))
	if natural <= 0 {
		d.SetX(x + width)
		return
	}
	scale := max(width, zeroWidth) / natural
	d.RawWriteStr(fmt.Sprintf("3 Tr %.3f Tz\n", 100*scale))
	d.layer(text, width, lineHeight, scale)
	d.RawWriteStr("0 Tr 100 Tz\n")
}

// layerFace is the face text in face i is laid in invisibly: the first
// face for one fpdf never registers.
func layerFace(i int) int {
	if faces[i].outlinesOnly {
		return 0
	}
	return i
}

// drawOutlines fills text's shaped glyph outlines in face i, in the text
// color on the baseline fpdf would give a cell on this line, and returns
// the width drawn. The position is left where it was.
func (d *Doc) drawOutlines(i int, style Style, size, lineHeight float64, text string, rtl bool) float64 {
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
	return pen - x
}

// layer writes text into the invisible text layer at the current
// position and horizontal scaling. Each character outside the BMP is
// shown as its code (tounicode.go), scaled to the width of the U+FFFD it
// is measured as. Once the codes run out, the text is wrapped in a
// marked-content span whose ActualText (ISO 32000-1, 14.9.4) is the text
// as typed.
func (d *Doc) layer(text string, width, lineHeight, scale float64) {
	shown, coded := d.layerText(text)
	measured := shownText(text)
	if shown != measured {
		d.RawWriteStr(fmt.Sprintf("%.3f Tz", 100*scale*d.GetStringWidth(measured)/d.GetStringWidth(shown)))
	}
	if !coded {
		d.RawWriteStr("/Span <</ActualText <FEFF" + utf16Hex(text) + ">>> BDC")
	}
	d.cell(shown, width, lineHeight)
	if !coded {
		d.RawWriteStr("EMC")
	}
	if shown != measured {
		d.RawWriteStr(fmt.Sprintf("%.3f Tz", 100*scale))
	}
}

// shownText is text as the layout measures it, each character outside
// the BMP as U+FFFD.
func shownText(text string) string {
	return strings.Map(func(r rune) rune {
		if outsideBMP(r) {
			return utf8.RuneError
		}
		return r
	}, text)
}

func utf16Hex(text string) string {
	var b strings.Builder
	for _, unit := range utf16.Encode([]rune(text)) {
		fmt.Fprintf(&b, "%04X", unit)
	}
	return b.String()
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
