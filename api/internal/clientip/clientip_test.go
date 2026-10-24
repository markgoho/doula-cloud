package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/clientip"
)

// caller is the real address in every row; proxyPeer is the RemoteAddr
// Cloud Run hands the container, which is never the caller.
const (
	caller    = "203.0.113.7"
	proxyPeer = "169.254.1.1:12345"
)

// TestFrom reads X-Forwarded-For from the right, the side the proxies
// write, and skips at most one Firebase Hosting edge. Each row is one shape
// a request can arrive in.
func TestFrom(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{
			name: "direct to Cloud Run, no caller header: Google's front end wrote the one entry",
			xff:  caller,
			want: caller,
		},
		{
			name: "direct to Cloud Run with a forged first entry: the appended caller wins",
			xff:  "198.51.100.4, 203.0.113.7",
			want: caller,
		},
		{
			name: "through the Hosting rewrite: the Hosting edge on the right is skipped",
			xff:  "203.0.113.7, 74.125.210.71",
			want: caller,
		},
		{
			name: "through the Hosting rewrite with a forged first entry",
			xff:  "198.51.100.4, 203.0.113.7, 66.102.8.131",
			want: caller,
		},
		{
			name: "direct with a forged Hosting-edge entry: the forged edge is not the right-most, so it is never reached",
			xff:  "198.51.100.4, 74.125.210.71, 203.0.113.7",
			want: caller,
		},
		{
			name: "two Hosting-edge entries on the right: only the one Hosting hop is skipped",
			xff:  "198.51.100.4, 74.125.1.1, 66.102.8.131",
			want: "74.125.1.1",
		},
		{
			name: "every entry is a Hosting edge: the left-most one is the caller",
			xff:  "192.178.11.4, 66.249.84.229",
			want: "192.178.11.4",
		},
		{
			name: "spaces and an empty entry from a caller-written prefix are tolerated",
			xff:  " junk value ,, 203.0.113.7 ,74.125.210.71",
			want: caller,
		},
		{
			name: "an entry that is not an address is never skipped as a proxy",
			xff:  "198.51.100.4, not-an-address",
			want: "not-an-address",
		},
		{
			name: "an IPv4-mapped IPv6 Hosting edge is still the Hosting edge",
			xff:  "203.0.113.7, ::ffff:74.125.210.71",
			want: caller,
		},
		{
			name: "an IPv6 caller through the Hosting rewrite",
			xff:  "2001:db8::1, 74.125.210.71",
			want: "2001:db8::1",
		},
		{
			name:       "no header: the local and e2e stack, with no proxy in front",
			remoteAddr: "127.0.0.1:54321",
			want:       "127.0.0.1",
		},
		{
			name:       "a header with no entries in it falls back to the peer",
			xff:        " , ",
			remoteAddr: "127.0.0.1:54321",
			want:       "127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r.RemoteAddr = proxyPeer
			if tt.remoteAddr != "" {
				r.RemoteAddr = tt.remoteAddr
			}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientip.From(r); got != tt.want {
				t.Fatalf("From() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFrom_JoinsRepeatedHeaders proves a second X-Forwarded-For header
// line is read as the continuation it is: a proxy may append a new line
// rather than extend the first one, and the right-most entry is then on
// the last line.
func TestFrom_JoinsRepeatedHeaders(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.Header.Add("X-Forwarded-For", "198.51.100.4")
	r.Header.Add("X-Forwarded-For", "203.0.113.7")

	if got := clientip.From(r); got != "203.0.113.7" {
		t.Fatalf("From() = %q, want 203.0.113.7", got)
	}
}
