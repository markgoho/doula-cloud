package contracts

import "testing"

// TestWithoutPriceLines is #2003's "the line is left out for the reader
// who may not see it": the line that names the price goes whole, and the
// paragraphs around it keep one blank line between them.
func TestWithoutPriceLines(t *testing.T) {
	cases := []struct {
		name  string
		prose string
		want  string
	}{
		{
			name:  "the default Template's own paragraph",
			prose: "Dates: {{engagement_start_date}}\n\nPrice: {{price}}\n\nBy signing below, both parties agree.",
			want:  "Dates: {{engagement_start_date}}\n\nBy signing below, both parties agree.",
		},
		{
			name:  "a price named with spaces inside the braces",
			prose: "Care for {{client_name}}.\nTotal: {{ price }}\nSigned.",
			want:  "Care for {{client_name}}.\nSigned.",
		},
		{
			name:  "a price on the first line",
			prose: "Price: {{price}}\n\nCare for {{client_name}}.",
			want:  "Care for {{client_name}}.",
		},
		{
			name:  "prose that never names a price",
			prose: "Care for {{client_name}}.\n\nSigned.",
			want:  "Care for {{client_name}}.\n\nSigned.",
		},
		{
			name:  "a field whose key only begins with price",
			prose: "Deposit: {{price_deposit}}",
			want:  "Deposit: {{price_deposit}}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withoutPriceLines(tc.prose); got != tc.want {
				t.Fatalf("withoutPriceLines(%q) = %q, want %q", tc.prose, got, tc.want)
			}
		})
	}
}
