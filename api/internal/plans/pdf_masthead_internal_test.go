package plans

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/pdftext"
)

// TestRenderPlanPDF_OpensWithItsMasthead proves the PDF names the plan
// before its first question (#2209, #2355): her name, her Doula or Doulas,
// the Practice, and the day it last changed, printed the way the page
// prints a calendar day. An Engagement nobody is on has no Doula line.
func TestRenderPlanPDF_OpensWithItsMasthead(t *testing.T) {
	fields := []Field{{ID: "where", Type: fieldTypeSingleSelect, Label: "Where to give birth"}}
	cases := []struct {
		name       string
		doulas     []string
		doulaLines []string
	}{
		{"no doula", nil, nil},
		{"one doula", []string{"Maya Okonkwo"}, []string{"Doula: Maya Okonkwo"}},
		{"two doulas", []string{"Maya Okonkwo", "Lena Vasquez"}, []string{"Doulas: Maya Okonkwo, Lena Vasquez"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := masthead{clientName: "Hannah Sorensen", doulas: tc.doulas, practiceName: "Rooted (Birth) Collective", changedOn: time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC)}

			out, err := renderPlanPDF(birthPlanPDFHeading, m, fields, Answers{"where": "At home"})
			if err != nil {
				t.Fatalf("renderPlanPDF: %v", err)
			}
			got := strings.Split(pdftext.Text(out), "\n")
			want := append(append([]string{"Birth Plan", "Name: Hannah Sorensen"}, tc.doulaLines...),
				"Practice: Rooted (Birth) Collective",
				"Last changed: Oct 27, 2026",
				"Where to give birth",
				"At home",
			)
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("PDF text = %q, want %q", got, want)
			}
		})
	}
}
