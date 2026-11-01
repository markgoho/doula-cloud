package pdffont

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// fpdf writes text as two-byte codes, one per BMP character, so a
// character outside the BMP cannot be a code of its own. The invisible
// layer shows each one as a BMP noncharacter instead, which no text
// interchanges, and every face's ToUnicode CMap maps that code back to
// the character (#2550). pdf.js and PDFium read a ToUnicode CMap but not
// an ActualText span. Past the last code, the layer falls back to U+FFFD
// inside an ActualText span, which only MuPDF and poppler read.
const firstAstralCode, lastAstralCode rune = 0xFDD0, 0xFDEF

// layerText is text as the invisible layer shows it, and whether every
// character outside the BMP got a code of its own.
func (d *Doc) layerText(text string) (string, bool) {
	coded := true
	shown := strings.Map(func(r rune) rune {
		if !outsideBMP(r) {
			return r
		}
		code, ok := d.astralCode(r)
		if !ok {
			coded = false
			return utf8.RuneError
		}
		return code
	}, text)
	return shown, coded
}

func (d *Doc) astralCode(r rune) (rune, bool) {
	if code, ok := d.astral[r]; ok {
		return code, true
	}
	if len(d.astral) > int(lastAstralCode-firstAstralCode) {
		return 0, false
	}
	code := firstAstralCode + rune(len(d.astral)) //nolint:gosec // G115: len(d.astral) is at most 31 here
	d.astral[r] = code
	return code, true
}

// Output writes the document to w, with the layer's codes mapped back to
// the characters outside the BMP they stand for.
func (d *Doc) Output(w io.Writer) error {
	var buf bytes.Buffer
	if err := d.Fpdf.Output(&buf); err != nil {
		return fmt.Errorf("pdffont: render: %w", err)
	}
	if _, err := w.Write(mapAstral(buf.Bytes(), d.astral)); err != nil {
		return fmt.Errorf("pdffont: write: %w", err)
	}
	return nil
}

// fpdfToUnicode is the one identity CMap fpdf writes for every face it
// embeds (utf8fontfile.go's toUnicode).
const (
	fpdfToUnicode = "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo\n<</Registry (Adobe)\n/Ordering (UCS)\n/Supplement 0\n>> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfrange\n<0000> <FFFF> <0000>\nendbfrange\nendcmap\nCMapName currentdict /CMap defineresource pop\nend\nend"
	identityRange = "1 beginbfrange\n<0000> <FFFF> <0000>\nendbfrange"
)

func cmapStream(cmap string) []byte {
	return fmt.Appendf(nil, "<</Length %d>>\nstream\n%s\nendstream", len(cmap), cmap)
}

// mapAstral rewrites each face's ToUnicode CMap in pdf, as fpdf wrote it,
// to map astral's codes, and moves the cross-reference table's offsets by
// what that adds.
func mapAstral(pdf []byte, astral map[rune]rune) []byte {
	if len(astral) == 0 {
		return pdf
	}
	var b strings.Builder
	fmt.Fprintf(&b, "2 beginbfrange\n<0000> <%04X> <0000>\n<%04X> <FFFF> <%04X>\nendbfrange\n", firstAstralCode-1, lastAstralCode+1, lastAstralCode+1)
	fmt.Fprintf(&b, "%d beginbfchar\n", len(astral))
	for _, r := range slices.Sorted(maps.Keys(astral)) {
		fmt.Fprintf(&b, "<%04X> <%s>\n", astral[r], utf16Hex(string(r)))
	}
	b.WriteString("endbfchar")
	old := cmapStream(fpdfToUnicode)
	replacement := cmapStream(strings.Replace(fpdfToUnicode, identityRange, b.String(), 1))
	delta := len(replacement) - len(old)

	var out []byte
	var at []int
	rest := pdf
	for {
		i := bytes.Index(rest, old)
		if i < 0 {
			out = append(out, rest...)
			break
		}
		at = append(at, len(pdf)-len(rest)+i)
		out = append(out, rest[:i]...)
		out = append(out, replacement...)
		rest = rest[i+len(old):]
	}
	shift := func(offset int) int {
		n := 0
		for _, a := range at {
			if a < offset {
				n++
			}
		}
		return offset + n*delta
	}

	xref := bytes.LastIndex(out, []byte("\nxref\n0 ")) + len("\nxref\n0 ")
	count, entries := cutAt(out[xref:], '\n')
	n, _ := strconv.Atoi(string(count))
	for i := 1; i < n; i++ {
		entry := entries[i*20 : i*20+10]
		offset, _ := strconv.Atoi(string(entry))
		copy(entry, fmt.Sprintf("%010d", shift(offset)))
	}
	startxref := bytes.LastIndex(out, []byte("\nstartxref\n")) + len("\nstartxref\n")
	offset, tail := cutAt(out[startxref:], '\n')
	o, _ := strconv.Atoi(string(offset))
	result := make([]byte, 0, len(out)+8)
	result = append(result, out[:startxref]...)
	result = strconv.AppendInt(result, int64(shift(o)), 10)
	result = append(result, '\n')
	return append(result, tail...)
}

func cutAt(b []byte, sep byte) ([]byte, []byte) {
	before, after, _ := bytes.Cut(b, []byte{sep})
	return before, after
}
