package feedback_test

import (
	"testing"

	"doula-cloud/api/internal/feedback"
)

func TestParseBrowser(t *testing.T) {
	cases := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "Chrome on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
			want:      "Chrome 129",
		},
		{
			name:      "Edge, Chromium-based",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.0.0",
			want:      "Edge 129",
		},
		{
			name:      "Firefox on Windows",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
			want:      "Firefox 130",
		},
		{
			name:      "Safari on macOS",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
			want:      "Safari 17",
		},
		{
			name:      "unrecognized",
			userAgent: "SomeCrawlerBot/1.0",
			want:      feedback.UnknownBrowser,
		},
		{
			name:      "empty",
			userAgent: "",
			want:      feedback.UnknownBrowser,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := feedback.ParseBrowser(c.userAgent); got != c.want {
				t.Errorf("ParseBrowser(%q) = %q, want %q", c.userAgent, got, c.want)
			}
		})
	}
}
