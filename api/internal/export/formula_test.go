package export_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
)

func TestHandler_GuardsATypedCellASpreadsheetWouldReadAsAFormula(t *testing.T) {
	const typed = `=HYPERLINK("http://example.invalid","click")`
	e := newEnv(t, fixture.WithClient(fixture.ClientName(typed)))

	guarded := false
	for _, cell := range e.csvFile("client.csv")[1] {
		if strings.HasPrefix(cell, "=") {
			t.Fatalf("client.csv holds an unguarded formula cell %q", cell)
		}
		guarded = guarded || strings.HasPrefix(cell, "'=HYPERLINK(")
	}
	if !guarded {
		t.Fatal("client.csv has no cell carrying the typed formula behind a leading '")
	}
	if !strings.Contains(e.archive()["README.txt"], "leading '") {
		t.Fatal("README.txt does not say what the leading ' means")
	}
}
