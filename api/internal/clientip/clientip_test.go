package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/clientip"
)

// TestFrom_UsesXFFFirstEntry proves the GFE-set header wins over
// RemoteAddr, and that only the first (closest-to-GFE) entry is used.
func TestFrom_UsesXFFFirstEntry(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")

	if got := clientip.From(r); got != "203.0.113.7" {
		t.Fatalf("From() = %q, want 203.0.113.7", got)
	}
}

// TestFrom_FallsBackToRemoteAddr proves the local-dev/test path -- no GFE
// in front of the process, so no X-Forwarded-For header is set.
func TestFrom_FallsBackToRemoteAddr(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:54321"

	if got := clientip.From(r); got != "127.0.0.1" {
		t.Fatalf("From() = %q, want 127.0.0.1", got)
	}
}
