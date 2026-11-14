// Package csvsafe is the one CSV writer in the BFF. A cell a spreadsheet
// would read as a formula (OWASP "CSV Injection") is written with a leading
// apostrophe, so text a Client or Staff member typed opens as text.
//
// A cell that starts with =, +, -, @, a tab or a carriage return is guarded,
// with one exemption by value shape and never by column: a plain negative
// decimal such as -500 or -5.00 is written as is, because it can hold no
// formula and a leading apostrophe would turn an amount into text a
// spreadsheet cannot sum. Anything else that starts with "-", "-5+cmd" for
// one, is guarded. A reader who wants the original text drops one leading
// apostrophe from a cell that has one.
package csvsafe

import (
	"encoding/csv"
	"fmt"
	"io"
)

// Writer writes CSV records, guarding every cell with Cell.
type Writer struct {
	w   *csv.Writer
	buf []string
}

// NewWriter returns a Writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: csv.NewWriter(w)}
}

// Write guards each cell of record and writes it. record is not modified.
func (w *Writer) Write(record []string) error {
	w.buf = w.buf[:0]
	for _, cell := range record {
		w.buf = append(w.buf, Cell(cell))
	}
	if err := w.w.Write(w.buf); err != nil {
		return fmt.Errorf("csvsafe: write: %w", err)
	}
	return nil
}

// Flush sends buffered records to the underlying writer.
func (w *Writer) Flush() { w.w.Flush() }

// Error reports a write failure; check it after Flush.
func (w *Writer) Error() error {
	if err := w.w.Error(); err != nil {
		return fmt.Errorf("csvsafe: flush: %w", err)
	}
	return nil
}

// Cell returns s, with a leading apostrophe when a spreadsheet would read it
// as a formula.
func Cell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + s
	case '-':
		if isNegativeDecimal(s) {
			return s
		}
		return "'" + s
	}
	return s
}

func isNegativeDecimal(s string) bool {
	digits := s[1:]
	if digits == "" {
		return false
	}
	seenPoint := false
	for i := range len(digits) {
		switch c := digits[i]; {
		case c >= '0' && c <= '9':
		case c == '.' && !seenPoint && i > 0 && i < len(digits)-1:
			seenPoint = true
		default:
			return false
		}
	}
	return true
}
