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
// Unicode font (#2359).
package pdftext

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
	"unicode/utf16"
)

// Text inflates every content stream in pdf and returns its drawn text,
// one line per baseline, in drawing order. Strings drawn one after
// another on the same baseline, as fpdf draws a line written in more than
// one font, are joined into one line. A stream that is not
// zlib-compressed, or does not read as a content stream (an embedded
// font, say), is skipped.
func Text(pdf []byte) string {
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
		lines = append(lines, drawnLines(content)...)
	}
	return strings.Join(lines, "\n")
}

// drawnLines walks one content stream's tokens. It remembers the last
// two numbers before a Td as the baseline, and each literal string a Tj
// shows is added to the line on that baseline. A byte outside a string
// that no content stream holds ends the walk with nothing: the stream is
// binary, not drawing.
func drawnLines(content []byte) []string {
	var lines []string
	var numbers []string
	var pending string
	baseline, lineBaseline := "", "\x00"
	for i := 0; i < len(content); {
		c := content[i]
		switch {
		case c == '(':
			s, next := literal(content, i+1)
			pending, i = s, next
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
			case "Tj":
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
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '(' || c == ')'
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
