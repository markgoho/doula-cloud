package plans

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/pdftext"
)

// TestRenderPlanPDF_OpensWithItsMasthead proves the PDF names the plan
// before its first question (#2209): her name, the Practice, and the day it
// last changed, printed the way the page prints a calendar day.
func TestRenderPlanPDF_OpensWithItsMasthead(t *testing.T) {
	fields := []Field{{ID: "where", Type: fieldTypeSingleSelect, Label: "Where to give birth"}}
	m := masthead{clientName: "Hannah Sorensen", practiceName: "Rooted (Birth) Collective", changedOn: time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC)}

	out, err := renderPlanPDF(birthPlanPDFHeading, m, fields, Answers{"where": "At home"})
	if err != nil {
		t.Fatalf("renderPlanPDF: %v", err)
	}
	got := strings.Split(pdftext.Text(out), "\n")
	want := []string{
		"Birth Plan",
		"Name: Hannah Sorensen",
		"Doula: Rooted (Birth) Collective",
		"Last changed: Oct 27, 2026",
		"Where to give birth",
		"At home",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("PDF text = %q, want %q", got, want)
	}
}
