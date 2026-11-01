package plans

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"

	"doula-cloud/api/internal/pdftext"
)

// #2359: the Birth Plan PDF prints a heading, a name, a Practice and an answer in any
// script as the characters typed, through pdffont's embedded faces.
func TestRenderPlanPDF_PrintsEveryScriptAsTyped(t *testing.T) {
	cases := []struct{ script, heading, name, practice, answer string }{
		{"Vietnamese", "Kế hoạch sinh", "Nguyễn Thị Hương", "Sài Gòn Doulas", "Ở nhà"},
		{"Polish", "Plan porodu", "Łucja Żółkiewska", "Pomoc Porodowa", "W domu"},
		{"Arabic", "خطة الولادة", "سارة الحداد", "دولا المدينة", "في المنزل"},
		{"Chinese", "分娩计划", "王小明", "北京导乐", "在家"},
		{"Devanagari", "जन्म योजना", "प्रिया शर्मा", "जन्म सहयोग", "घर पर"},
		{"Cyrillic", "План родов", "Анна Петрова", "Доула Москва", "Дома"},
		{"a combining mark", "Birth Plan", "Zoë Brontë", "Rooted Birth Collective", "At home"},
	}
	fields := []Field{
		{ID: "section", Type: fieldTypeSectionHeader, Label: "Birth Setting"},
		{ID: "place", Type: fieldTypeSingleSelect, Label: "Place of birth"},
	}
	for _, tc := range cases {
		t.Run(tc.script, func(t *testing.T) {
			m := masthead{clientName: tc.name, doulas: []string{tc.name}, practiceName: tc.practice, changedOn: time.Date(2026, time.October, 27, 0, 0, 0, 0, time.UTC)}
			out, err := renderPlanPDF(tc.heading, m, fields, Answers{"place": tc.answer})
			if err != nil {
				t.Fatalf("renderPlanPDF: %v", err)
			}
			got := strings.Split(pdftext.Text(out), "\n")
			want := []string{
				tc.heading,
				"Name: " + tc.name,
				"Doula: " + tc.name,
				"Practice: " + tc.practice,
				"Last changed: Oct 27, 2026",
				"Birth Setting",
				"Place of birth",
				tc.answer,
			}
			if strings.Join(got, "|") != norm.NFC.String(strings.Join(want, "|")) {
				t.Fatalf("PDF text = %q, want %q", got, want)
			}
		})
	}
}
