package pdffont

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/font"
)

func glyphIDs(d *Doc, face int, text string, rtl bool) []font.GID {
	_, out := d.shape(face, Regular, text, rtl)
	ids := make([]font.GID, len(out.Glyphs))
	for i, g := range out.Glyphs {
		ids[i] = g.GlyphID
	}
	return ids
}

const arabicFace, devanagariFace = 1, 2

// #2538: Arabic letters inside a word take their joined forms, not the
// isolated ones each letter has alone, and the word runs right to left:
// its first letter's glyph comes last.
func TestShape_JoinsArabicRightToLeft(t *testing.T) {
	d := New()
	word := []rune("سارة")
	_, out := d.shape(arabicFace, Regular, string(word), true)
	if len(out.Glyphs) != len(word) {
		t.Fatalf("glyphs = %d, want one per letter (%d)", len(out.Glyphs), len(word))
	}
	if last := out.Glyphs[len(out.Glyphs)-1]; last.TextIndex() != 0 {
		t.Fatalf("last glyph is letter %d, want the first letter: the word reads right to left", last.TextIndex())
	}
	joined := glyphIDs(d, arabicFace, string(word), true)
	var isolated []font.GID
	for _, r := range word {
		isolated = append(isolated, glyphIDs(d, arabicFace, string(r), true)...)
	}
	slices.Reverse(isolated)
	if joined[len(joined)-1] == isolated[len(isolated)-1] {
		t.Fatalf("the first letter drew in its isolated form (%d), not joined to the next", joined[len(joined)-1])
	}
}

// #2538: "प्रि" is a conjunct and its short-i sign, which is typed after
// the consonant and drawn before it, so it shapes to fewer glyphs than
// characters and the conjunct is not its first glyph.
func TestShape_FormsDevanagariConjunctsAndReordersTheShortI(t *testing.T) {
	d := New()
	cluster := "प्रि"
	shaped := glyphIDs(d, devanagariFace, cluster, false)
	if len(shaped) >= len([]rune(cluster)) {
		t.Fatalf("glyphs = %d, want fewer than the %d characters: no conjunct formed", len(shaped), len([]rune(cluster)))
	}
	conjunct := glyphIDs(d, devanagariFace, "प्र", false)
	if len(conjunct) != 1 {
		t.Fatalf("प्र shaped to %d glyphs, want one conjunct", len(conjunct))
	}
	if shaped[0] == conjunct[0] || shaped[len(shaped)-1] != conjunct[0] {
		t.Fatalf("glyphs = %v, want the short-i sign before the conjunct %d", shaped, conjunct[0])
	}
}

// A shaped run is drawn as filled outlines with the typed text laid over
// it invisibly, then the text render mode and scale are put back.
func TestDrawShaped_FillsOutlinesUnderInvisibleText(t *testing.T) {
	d := New()
	d.SetCompression(false)
	d.Paragraph(Regular, 12, 8, "Name: सारा")
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{" m\n", "\nf\n", "3 Tr ", "0 Tr 100 Tz"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Fatalf("content stream has no %q", want)
		}
	}
}

// A shaped line that starts a new page draws its outlines and its text
// layer on that page, so every page's fills sit under their own text.
func TestDrawShaped_KeepsOutlinesAndTextOnOnePage(t *testing.T) {
	d := New()
	d.SetCompression(false)
	d.Paragraph(Regular, 12, 8, strings.Repeat("هذه الاتفاقية بين دولا المدينة وسارة الحداد. ", 80))
	var buf bytes.Buffer
	if err := d.Output(&buf); err != nil {
		t.Fatal(err)
	}
	if d.PageCount() < 2 {
		t.Fatalf("pages = %d, want the paragraph to cross a page", d.PageCount())
	}
	for i, stream := range bytes.Split(buf.Bytes(), []byte("endstream")) {
		fills, layers, resets := bytes.Count(stream, []byte("\nf\n")), bytes.Count(stream, []byte("3 Tr ")), bytes.Count(stream, []byte("0 Tr 100 Tz"))
		if fills != layers || layers != resets {
			t.Fatalf("stream %d: %d outline fills, %d text layers, %d resets", i, fills, layers, resets)
		}
	}
}

// An empty line draws nothing and only moves down a line.
func TestFlush_DrawsNothingForAnEmptyLine(t *testing.T) {
	d := New()
	y := d.GetY()
	d.Paragraph(Regular, 12, 8, "  ")
	if got := d.GetY(); got != y+8 {
		t.Fatalf("y = %v, want %v", got, y+8)
	}
}

// Text that measures nothing in its face draws nothing over the outlines,
// and the position still moves past them.
func TestDrawShaped_SkipsTheTextLayerWhenItMeasuresNothing(t *testing.T) {
	d := New()
	d.use(arabicFace, Regular, 12)
	x := d.GetX()
	d.drawShaped(arabicFace, Regular, 12, 8, "\u200d", false)
	if d.GetX() < x {
		t.Fatalf("x moved back from %v to %v", x, d.GetX())
	}
}

const emojiFace = 5

// #2544: in a right-to-left run a bracket draws as its mirror, by the
// Unicode Bidi Mirroring property, so "(" opens on the right.
func TestShape_MirrorsBracketsRightToLeft(t *testing.T) {
	d := New()
	for typed, mirror := range map[string]string{"(": ")", "[": "]", "«": "»"} {
		if got, want := glyphIDs(d, 0, typed, true), glyphIDs(d, 0, mirror, false); !slices.Equal(got, want) {
			t.Fatalf("%q right to left drew %v, want the glyph of %q, %v", typed, got, mirror, want)
		}
	}
}

// #2544: an emoji sequence joined by U+200D, and an emoji with a skin
// tone, each draw as one glyph.
func TestShape_JoinsEmojiSequences(t *testing.T) {
	d := New()
	if faces[emojiFace].family != "NotoEmoji" {
		t.Fatalf("face %d is %s, want NotoEmoji", emojiFace, faces[emojiFace].family)
	}
	for _, sequence := range []string{"👩‍🍼", "👍🏽"} {
		if got := glyphIDs(d, emojiFace, sequence, false); len(got) != 1 {
			t.Fatalf("%q drew %d glyphs, want 1", sequence, len(got))
		}
	}
}

// #2544: the joiner inside an emoji sequence stays with its emoji, and
// the space after an emoji goes back to Noto Sans.
func TestRuns_KeepsAnEmojiSequenceWholeAndItsSpaceOut(t *testing.T) {
	got := runs("a 👩‍🍼 b")
	want := []run{{face: 0, text: "a ", start: 0}, {face: emojiFace, text: "👩‍🍼", start: 2}, {face: 0, text: " b", start: 5}}
	if !slices.Equal(got, want) {
		t.Fatalf("runs = %+v, want %+v", got, want)
	}
}

// #2544: a character outside the BMP is shown as U+FFFD in the invisible
// text, inside a span whose ActualText is the character in UTF-16 with a
// surrogate pair; text inside the BMP has no span.
func TestDrawShaped_CarriesACharacterOutsideTheBMPAsActualText(t *testing.T) {
	content := func(text string) []byte {
		d := New()
		d.SetCompression(false)
		d.Paragraph(Regular, 12, 8, text)
		var buf bytes.Buffer
		if err := d.Output(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	baby := content("Ada \U0001F476")
	for _, want := range []string{"/Span <</ActualText <FEFFD83DDC76>>> BDC\n3 Tr ", "(\xff\xfd)Tj ET\n0 Tr 100 Tz\n\nEMC\n"} {
		if !bytes.Contains(baby, []byte(want)) {
			t.Fatalf("content stream has no %q", want)
		}
	}
	if heart := content("Ada ❤️"); bytes.Contains(heart, []byte("ActualText")) {
		t.Fatal("a heart, inside the BMP, was given an ActualText")
	}
}

// Rule L2: the highest level is reversed first, then each lower odd one.
func TestReorder_ReversesFromTheHighestLevelDown(t *testing.T) {
	segments := []segment{{text: "a", level: 0}, {text: "b", level: 1}, {text: "1", level: 2}, {text: "c", level: 1}, {text: "d", level: 0}}
	reorder(segments)
	var got []string
	for _, s := range segments {
		got = append(got, s.text)
	}
	if want := []string{"a", "c", "1", "b", "d"}; !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	reorder(nil)
}

// A paragraph takes its direction from its first letter.
func TestBidiLevels_TakesTheParagraphFromItsFirstLetter(t *testing.T) {
	for text, want := range map[string]bidi.Level{"سارة and Ada": 1, "Ada and سارة": 0, "12 سارة": 1, "": 0} {
		if _, got := bidiLevels(text); got != want {
			t.Fatalf("bidiLevels(%q) base = %d, want %d", text, got, want)
		}
	}
}

// A right-to-left run's stored text is its letters in reverse, each with
// its marks.
func TestVisualOrder_KeepsMarksWithTheirLetter(t *testing.T) {
	if got, want := visualOrder("بِسم"), "مسبِ"; got != want {
		t.Fatalf("visualOrder = %q, want %q", got, want)
	}
}
