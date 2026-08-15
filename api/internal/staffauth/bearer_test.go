package staffauth

import (
	"net/http/httptest"
	"testing"
)

// TestBearerToken exercises bearerToken directly rather than over real HTTP
// (where header-value whitespace can be normalized in transit), so the
// empty-token-after-prefix case is asserted reliably.
func TestBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "missing header", header: "", wantToken: "", wantOK: false},
		{name: "wrong scheme", header: "Basic abc123", wantToken: "", wantOK: false},
		{name: "empty token", header: "Bearer ", wantToken: "", wantOK: false},
		{name: "valid token", header: "Bearer abc123", wantToken: "abc123", wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, ok := bearerToken(req)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if token != tt.wantToken {
				t.Fatalf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}
