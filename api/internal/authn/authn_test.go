package authn_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, ok := authn.BearerToken(req)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if token != tt.wantToken {
				t.Fatalf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestBegin_MissingBearerToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{}, nil)
	if ok {
		t.Fatalf("expected ok=false, got true (tx=%v, uid=%q)", tx, uid)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Body.String(); got != "missing bearer token\n" {
		t.Fatalf("body = %q, want %q", got, "missing bearer token\n")
	}
}

func TestBegin_InvalidToken(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{Err: errors.New("bad token")}, nil)
	if ok {
		t.Fatalf("expected ok=false, got true (tx=%v, uid=%q)", tx, uid)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Body.String(); got != "invalid token\n" {
		t.Fatalf("body = %q, want %q", got, "invalid token\n")
	}
}

func TestBegin_Success(t *testing.T) {
	db := testdb.New(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	tx, uid, ok := authn.Begin(rec, req, authntest.Verifier{UID: "test-uid"}, db.App)
	if !ok {
		t.Fatalf("expected ok=true, got false")
	}
	defer func() { _ = tx.Rollback() }()
	if uid != "test-uid" {
		t.Fatalf("uid = %q, want %q", uid, "test-uid")
	}
}
