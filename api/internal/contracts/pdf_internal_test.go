package contracts

import (
	"os"
	"strings"
	"testing"
)

// TestSignConsentLine_IsTheSigningFormsCheckbox proves the consent line
// the signed PDF prints is, word for word, the checkbox the Client checked
// on the signing form (SIGN_CONSENT_LABEL, app/src/lib/clientRegister.ts):
// the PDF quotes what she confirmed, so the two cannot drift (#2214).
func TestSignConsentLine_IsTheSigningFormsCheckbox(t *testing.T) {
	src, err := os.ReadFile("../../../app/src/lib/clientRegister.ts")
	if err != nil {
		t.Fatalf("read clientRegister.ts: %v", err)
	}
	want := "export const SIGN_CONSENT_LABEL = '" + signConsentLine + "';"
	if !strings.Contains(string(src), want) {
		t.Fatalf("clientRegister.ts does not declare %s", want)
	}
}

const (
	fillProseTestName = "Jamie"
	fillProseTestKey  = "client_name"
)

// TestFillProse_SubstitutesAndBlanksUnfilled proves fillProse mirrors
// app/src/lib/contract.ts's fillProse exactly: a filled merge field is
// substituted, an unfilled one is blanked (not left as the raw
// placeholder), a key repeated in prose is substituted at every
// occurrence, and whitespace inside the {{ }} delimiters is tolerated the
// same way mergeFieldPattern already allows for extractMergeFields.
func TestFillProse_SubstitutesAndBlanksUnfilled(t *testing.T) {
	cases := []fillProseCase{
		{
			name:   "filled key substituted",
			prose:  "Agreement for {{client_name}}.",
			values: MergeFieldValues{fillProseTestKey: fillProseTestName},
			want:   "Agreement for " + fillProseTestName + ".",
		},
		{
			name:   "unfilled key blanked, not left as placeholder",
			prose:  "Agreement for {{client_name}} at {{price}}.",
			values: MergeFieldValues{fillProseTestKey: fillProseTestName},
			want:   "Agreement for " + fillProseTestName + " at .",
		},
		{
			name:   "repeated key substituted at every occurrence",
			prose:  "{{client_name}} agrees. Signed, {{client_name}}.",
			values: MergeFieldValues{fillProseTestKey: fillProseTestName},
			want:   fillProseTestName + " agrees. Signed, " + fillProseTestName + ".",
		},
		{
			name:   "whitespace inside delimiters tolerated",
			prose:  "Agreement for {{ client_name }}.",
			values: MergeFieldValues{fillProseTestKey: fillProseTestName},
			want:   "Agreement for " + fillProseTestName + ".",
		},
		{
			name:   "nil values blanks every field",
			prose:  "Agreement for {{client_name}}.",
			values: nil,
			want:   "Agreement for .",
		},
	}

	runFillProseCases(t, cases)
}

// TestFillProse_PrintsADateFieldAsACalendarDay proves #2222 at fill time:
// a date merge field holding a YYYY-MM-DD day is printed the way the
// product prints a calendar day everywhere else ("Oct 27, 2026",
// app/src/lib/dates.ts's formatCalendarDay), so the signed PDF reads like
// the portal page. app/src/lib/contract.spec.ts asserts the same strings
// for the same values. Anything else passes through as typed.
func TestFillProse_PrintsADateFieldAsACalendarDay(t *testing.T) {
	const isoDay = "2026-10-27"
	runFillProseCases(t, []fillProseCase{
		{
			name:   "both date fields formatted",
			prose:  "Dates of care: {{engagement_start_date}} through {{engagement_end_date}}",
			values: MergeFieldValues{mergeFieldStartDate: isoDay, mergeFieldEndDate: "2027-04-05"},
			want:   "Dates of care: Oct 27, 2026 through Apr 5, 2027",
		},
		{
			name:   "a date field typed as words passes through",
			prose:  "From {{engagement_start_date}}",
			values: MergeFieldValues{mergeFieldStartDate: "next spring"},
			want:   "From next spring",
		},
		{
			name:   "an impossible day passes through",
			prose:  "From {{engagement_start_date}}",
			values: MergeFieldValues{mergeFieldStartDate: "2026-02-30"},
			want:   "From 2026-02-30",
		},
		{
			name:   "a field that is not a date is untouched",
			prose:  "Scope: {{scope_of_service}}",
			values: MergeFieldValues{"scope_of_service": isoDay},
			want:   "Scope: " + isoDay,
		},
	})
}

type fillProseCase struct {
	name   string
	prose  string
	values MergeFieldValues
	want   string
}

func runFillProseCases(t *testing.T, cases []fillProseCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fillProse(tc.prose, tc.values)
			if got != tc.want {
				t.Fatalf("fillProse(%q, %+v) = %q, want %q", tc.prose, tc.values, got, tc.want)
			}
		})
	}
}
