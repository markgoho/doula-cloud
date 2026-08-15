package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelloHandler(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()

	helloHandler(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Body.String(); got != `{"message":"hello world"}`+"\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestResolvePort(t *testing.T) {
	t.Setenv("PORT", "")
	if got := resolvePort(); got != "8080" {
		t.Fatalf("resolvePort() = %q, want 8080", got)
	}

	t.Setenv("PORT", "9090")
	if got := resolvePort(); got != "9090" {
		t.Fatalf("resolvePort() = %q, want 9090", got)
	}
}
