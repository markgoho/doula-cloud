package csvsafe_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"doula-cloud/api/internal/csvsafe"
)

const amount = "-5.00"

func TestCell(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"equals", "=SUM(A1)", "'=SUM(A1)"},
		{"plus", "+1+1", "'+1+1"},
		{"minus text", "-cmd", "'-cmd"},
		{"at", "@SUM(A1)", "'@SUM(A1)"},
		{"tab", "\t=1", "'\t=1"},
		{"carriage return", "\r=1", "'\r=1"},
		{"plain", "Ada Lovelace", "Ada Lovelace"},
		{"empty", "", ""},
		{"inner equals", "a=b", "a=b"},
		{"negative integer", "-500", "-500"},
		{"negative decimal", amount, amount},
		{"positive amount", "500", "500"},
		{"lone minus", "-", "'-"},
		{"minus point", "-.5", "'-.5"},
		{"trailing point", "-5.", "'-5."},
		{"two points", "-5.0.0", "'-5.0.0"},
		{"number then formula", "-5+cmd", "'-5+cmd"},
		{"exponent", "-1e5", "'-1e5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := csvsafe.Cell(tt.in); got != tt.want {
				t.Fatalf("Cell(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWriter_GuardsEveryCellAndLeavesTheRecordAlone(t *testing.T) {
	var out bytes.Buffer
	w := csvsafe.NewWriter(&out)
	record := []string{"=1+1", amount, "plain"}
	if err := w.Write(record); err != nil {
		t.Fatal(err)
	}
	if err := w.Write([]string{"@x", ""}); err != nil {
		t.Fatal(err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "'=1+1,-5.00,plain\n'@x,\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if record[0] != "=1+1" {
		t.Fatalf("Write changed the caller's record: %v", record)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestWriter_ReportsAWriteFailureAfterFlush(t *testing.T) {
	w := csvsafe.NewWriter(failingWriter{})
	_ = w.Write([]string{strings.Repeat("x", 8192)})
	w.Flush()
	if w.Error() == nil {
		t.Fatal("Error() = nil, want the underlying write failure")
	}
}
