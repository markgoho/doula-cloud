package contracts

import "testing"

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
	cases := []struct {
		name   string
		prose  string
		values MergeFieldValues
		want   string
	}{
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

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fillProse(tc.prose, tc.values)
			if got != tc.want {
				t.Fatalf("fillProse(%q, %+v) = %q, want %q", tc.prose, tc.values, got, tc.want)
			}
		})
	}
}
