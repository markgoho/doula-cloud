// Package pdftext reads back the text a PDF this codebase rendered with
// fpdf draws, for a test to assert on what a person reading the file
// would see. fpdf compresses each page's content stream, so the words are
// not in the bytes as written.
//
// It began as a test-only helper inside plans (#2209, the Birth Plan
// masthead) and moved here when the signed Contract PDF became its second
// reader (#2214): a helper in a _test.go file cannot be imported across
// packages. It is a reader for the PDFs pdffont writes, not a general PDF
// parser: it knows only the "x y Td (...) Tj" form fpdf emits, with each
// string in UTF-16BE, which is how fpdf writes text in an embedded
// Unicode font (#2359). pdffont writes each line's text in the order it
// was typed, right-to-left text included (#2546), so a line reads back in
// the order its strings are shown.
package pdftext

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"strings"
	"unicode/utf16"
)

// Text inflates every content stream in pdf and returns its drawn text,
// one line per baseline, in drawing order. Strings drawn one after
// another on the same baseline, as fpdf draws a line written in more than
// one font, are joined into one line. A stream that is not
// zlib-compressed, or does not read as a content stream (an embedded
// font, say), is skipped. A code a ToUnicode CMap maps one by one, as
// pdffont maps a character outside the BMP (#2550), reads as what it maps
// to.
func Text(pdf []byte) string {
	codes := mappedCodes(pdf)
	var lines []string
	rest := pdf
	for {
		start := bytes.Index(rest, []byte("stream\n"))
		if start < 0 {
			break
		}
		rest = rest[start+len("stream\n"):]
		end := bytes.Index(rest, []byte("endstream"))
		if end < 0 {
			break
		}
		raw := rest[:end]
		rest = rest[end+len("endstream"):]
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			continue
		}
		content, err := io.ReadAll(r)
		if err != nil && len(content) == 0 {
			continue
		}
		for _, line := range drawnLines(content) {
			lines = append(lines, strings.Map(func(r rune) rune {
				if m, ok := codes[r]; ok {
					return m
				}
				return r
			}, line))
		}
	}
	return strings.Join(lines, "\n")
}

// mappedCodes reads every beginbfchar entry of pdf's ToUnicode CMaps,
// which fpdf writes uncompressed, that maps one code to one character.
func mappedCodes(pdf []byte) map[rune]rune {
	codes := map[rune]rune{}
	rest := pdf
	for {
		start := bytes.Index(rest, []byte("beginbfchar\n"))
		if start < 0 {
			return codes
		}
		rest = rest[start+len("beginbfchar\n"):]
		var entries []byte
		entries, rest, _ = bytes.Cut(rest, []byte("endbfchar"))
		for line := range strings.SplitSeq(string(entries), "\n") {
			from, to, ok := strings.Cut(line, " ")
			if !ok {
				continue
			}
			code := []rune(decodeHex([]byte(strings.Trim(from, "<>"))))
			char := []rune(decodeHex([]byte(strings.Trim(to, "<>"))))
			if len(code) == 1 && len(char) == 1 {
				codes[code[0]] = char[0]
			}
		}
	}
}

// drawnLines walks one content stream's tokens. It remembers the last
// two numbers before a Td as the baseline, and each literal string a Tj
// shows is added to the line on that baseline. Inside a marked-content
// span with an ActualText, as pdffont writes around a character outside
// the BMP (#2544), the span's first Tj shows the ActualText in place of
// its own string and the rest show nothing, the way a reader copies it. A
// byte outside a string that no content stream holds ends the walk with
// nothing: the stream is binary, not drawing.
func drawnLines(content []byte) []string {
	var lines []string
	var numbers []string
	var pending, hexText, actual string
	inSpan := false
	baseline, lineBaseline := "", "\x00"
	for i := 0; i < len(content); {
		c := content[i]
		switch {
		case c == '(':
			s, next := literal(content, i+1)
			pending, i = s, next
		case c == '<' && i+1 < len(content) && content[i+1] == '<', c == '>' && i+1 < len(content) && content[i+1] == '>':
			i += 2
		case c == '<':
			end := bytes.IndexByte(content[i:], '>')
			if end < 0 {
				return nil
			}
			hexText, i = decodeHex(content[i+1:i+end]), i+end+1
		case c == ' ' || c == '\n' || c == '\r' || c == '\t':
			i++
		case c < 0x20 || c > 0x7e:
			return nil
		default:
			j := i
			for j < len(content) && !isDelimiter(content[j]) {
				j++
			}
			word := string(content[i:j])
			switch word {
			case "Td":
				if len(numbers) >= 1 {
					baseline = numbers[len(numbers)-1]
				}
			case "BDC":
				if len(numbers) > 0 && numbers[len(numbers)-1] == "/ActualText" {
					actual, inSpan = hexText, true
				}
			case "EMC":
				inSpan = false
			case "Tj":
				if inSpan {
					pending, actual = actual, ""
				}
				if baseline == lineBaseline && len(lines) > 0 {
					lines[len(lines)-1] += pending
				} else {
					lines = append(lines, pending)
					lineBaseline = baseline
				}
			}
			numbers = append(numbers, word)
			if j == i {
				j++
			}
			i = j
		}
	}
	return lines
}

func isDelimiter(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '(' || c == ')' || c == '<' || c == '>'
}

// decodeHex decodes a hexadecimal string's UTF-16BE, after its byte order
// mark.
func decodeHex(digits []byte) string {
	raw, err := hex.DecodeString(string(digits))
	if err != nil {
		return ""
	}
	return decodeUTF16(bytes.TrimPrefix(raw, []byte{0xFE, 0xFF}))
}

// literal reads a PDF literal string starting just after its "(", undoing
// fpdf's escapes, and decodes its UTF-16BE. It returns the text and the
// index just past the closing ")".
func literal(content []byte, i int) (string, int) {
	var raw []byte
	for i < len(content) {
		c := content[i]
		i++
		switch c {
		case '\\':
			if i < len(content) {
				e := content[i]
				i++
				if e == 'r' {
					e = '\r'
				}
				raw = append(raw, e)
			}
		case ')':
			return decodeUTF16(raw), i
		default:
			raw = append(raw, c)
		}
	}
	return decodeUTF16(raw), i
}

func decodeUTF16(raw []byte) string {
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	return string(utf16.Decode(units))
}
