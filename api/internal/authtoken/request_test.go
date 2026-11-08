package authtoken_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authtoken"
)

func TestRequireToken(t *testing.T) {
	token := "  abc  "
	w := httptest.NewRecorder()
	if !authtoken.RequireToken(w, &token) || token != "abc" {
		t.Fatalf("got %q, want the trimmed token accepted", token)
	}

	blank := "   "
	w = httptest.NewRecorder()
	if authtoken.RequireToken(w, &blank) {
		t.Fatal("a blank token was accepted")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
