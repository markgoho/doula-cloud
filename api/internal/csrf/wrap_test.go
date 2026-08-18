package csrf_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/csrf"
)

const allowedTestOrigin = "https://app.example"

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestWrap_MismatchedOriginRejected(t *testing.T) {
	handler := csrf.Wrap([]string{allowedTestOrigin}, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestWrap_MatchingOriginAllowed(t *testing.T) {
	handler := csrf.Wrap([]string{allowedTestOrigin}, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.Header.Set("Origin", allowedTestOrigin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWrap_MissingOriginAllowed(t *testing.T) {
	handler := csrf.Wrap([]string{allowedTestOrigin}, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWrap_NonStateChangingMethodIgnoresOrigin(t *testing.T) {
	handler := csrf.Wrap([]string{allowedTestOrigin}, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWrap_MultipleAllowedOrigins(t *testing.T) {
	handler := csrf.Wrap([]string{"http://localhost:5173", "http://localhost:4173"}, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.Header.Set("Origin", "http://localhost:4173")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWrap_NoAllowedOriginsRejectsAnyOrigin(t *testing.T) {
	handler := csrf.Wrap(nil, okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.Header.Set("Origin", allowedTestOrigin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
