package plans

import (
	"bytes"
	"testing"
)

// TestPlanFieldAnswerText_MirrorsBirthPlanView proves planFieldAnswerText
// matches BirthPlanView.svelte's textValue/checkboxValue/selectedOptions
// exactly for every field type this package supports.
func TestPlanFieldAnswerText_MirrorsBirthPlanView(t *testing.T) {
	cases := []struct {
		name    string
		field   Field
		answers Answers
		want    string
	}{
		{
			name:    "short_text filled",
			field:   Field{ID: "f1", Type: fieldTypeShortText},
			answers: Answers{"f1": "Water birth preferred"},
			want:    "Water birth preferred",
		},
		{
			name:    "short_text blank renders em dash",
			field:   Field{ID: "f1", Type: fieldTypeShortText},
			answers: Answers{},
			want:    "—",
		},
		{
			name:    "single_select blank renders em dash",
			field:   Field{ID: "f1", Type: fieldTypeSingleSelect},
			answers: Answers{},
			want:    "—",
		},
		{
			name:    "single_select filled",
			field:   Field{ID: "f1", Type: fieldTypeSingleSelect},
			answers: Answers{"f1": "Birth center"},
			want:    "Birth center",
		},
		{
			name:    "checkbox checked",
			field:   Field{ID: "f1", Type: fieldTypeCheckbox},
			answers: Answers{"f1": true},
			want:    "Yes",
		},
		{
			name:    "checkbox unchecked",
			field:   Field{ID: "f1", Type: fieldTypeCheckbox},
			answers: Answers{"f1": false},
			want:    "No",
		},
		{
			name:    "checkbox unanswered defaults to No",
			field:   Field{ID: "f1", Type: fieldTypeCheckbox},
			answers: Answers{},
			want:    "No",
		},
		{
			name:    "multi_select empty renders em dash",
			field:   Field{ID: "f1", Type: fieldTypeMultiSelect},
			answers: Answers{},
			want:    "—",
		},
		{
			name:    "multi_select joins selected options",
			field:   Field{ID: "f1", Type: fieldTypeMultiSelect},
			answers: Answers{"f1": []any{"Partner", "Doula"}},
			want:    "Partner, Doula",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planFieldAnswerText(tc.field, tc.answers)
			if got != tc.want {
				t.Fatalf("planFieldAnswerText(%+v, %+v) = %q, want %q", tc.field, tc.answers, got, tc.want)
			}
		})
	}
}

// TestRenderPlanPDF_ProducesAValidPDF proves renderPlanPDF returns
// non-empty bytes starting with the PDF magic header, across a field list
// carrying a section_header (BirthPlanView.svelte's grouping cue) and
// every other field type.
func TestRenderPlanPDF_ProducesAValidPDF(t *testing.T) {
	fields := []Field{
		{ID: "loc", Type: fieldTypeSingleSelect, Label: "Planned birth location"},
		{ID: "sec", Type: fieldTypeSectionHeader, Label: "Support"},
		{ID: "support", Type: fieldTypeMultiSelect, Label: "Who is in the room"},
		{ID: "notes", Type: "long_text", Label: "Anything else"},
		{ID: "pain", Type: fieldTypeCheckbox, Label: "Wants pain medication discussed"},
	}
	answers := Answers{
		"loc":     "Home",
		"support": []any{"Partner", "Doula"},
		"pain":    true,
	}

	out, err := renderPlanPDF("Birth Plan", fields, answers)
	if err != nil {
		t.Fatalf("renderPlanPDF: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("renderPlanPDF returned no bytes")
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("renderPlanPDF output does not start with the PDF magic header: %q", out[:min(16, len(out))])
	}
}
