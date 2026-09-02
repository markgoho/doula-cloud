package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/ratelimit"
	"doula-cloud/api/internal/testdb"
)

func postJSON(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	//nolint:noctx // test-only fixed-format URL, not user input
	resp, err := http.Post(srv.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	return resp
}

// TestSessionCookieRule_KeysBySessionDigest proves SessionCookieRule
// counts a presented __session cookie, independently per token, and that
// the same session replayed past Max is refused -- #613's signed-in
// "request a fresh verification link", which has no Bearer token to key
// on.
func TestSessionCookieRule_KeysBySessionDigest(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_session", []ratelimit.Rule{ratelimit.SessionCookieRule(1, time.Hour)})(okHandler())

	reqA1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqA1.Header.Set("Cookie", authn.SessionCookieName+"=session-a")
	recA1 := httptest.NewRecorder()
	handler.ServeHTTP(recA1, reqA1)
	if recA1.Code != http.StatusOK {
		t.Fatalf("session-a, request 1: status = %d, want 200", recA1.Code)
	}

	reqB1 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqB1.Header.Set("Cookie", authn.SessionCookieName+"=session-b")
	recB1 := httptest.NewRecorder()
	handler.ServeHTTP(recB1, reqB1)
	if recB1.Code != http.StatusOK {
		t.Fatalf("session-b, request 1: status = %d, want 200 (must not share session-a's counter)", recB1.Code)
	}

	reqA2 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	reqA2.Header.Set("Cookie", authn.SessionCookieName+"=session-a")
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)
	if recA2.Code != http.StatusTooManyRequests {
		t.Fatalf("session-a, request 2: status = %d, want 429", recA2.Code)
	}
}

// TestSessionCookieRule_SkippedWithNoCookie proves a request with no
// __session cookie at all is skipped rather than counted against an
// empty key.
func TestSessionCookieRule_SkippedWithNoCookie(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_session_missing", []ratelimit.Rule{ratelimit.SessionCookieRule(1, time.Hour)})(okHandler())

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}
}

// TestJSONFieldRule_KeysByFieldValueAndPreservesBody proves JSONFieldRule
// counts by the named JSON field, independently per value, and that the
// handler behind Wrap still reads the full body -- #613's password-reset
// request, keyed on the posted email address.
func TestJSONFieldRule_KeysByFieldValueAndPreservesBody(t *testing.T) {
	db := testdb.New(t)
	var sawBody string
	echoBody := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		sawBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	})
	handler := ratelimit.Wrap(db.App, "test_email", []ratelimit.Rule{ratelimit.JSONFieldRule("email", 1, time.Hour)})(echoBody)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	first := postJSON(t, srv, `{"email":"Person@Example.com"}`)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", first.StatusCode)
	}
	if sawBody != `{"email":"Person@Example.com"}` {
		t.Fatalf("handler saw body = %q, want the original JSON restored", sawBody)
	}

	other := postJSON(t, srv, `{"email":"other@example.com"}`)
	_ = other.Body.Close()
	if other.StatusCode != http.StatusOK {
		t.Fatalf("a different email: status = %d, want 200 (must not share the first address's counter)", other.StatusCode)
	}

	// The same address, differently cased -- JSONFieldRule normalizes
	// before keying, so this must count against the first request, not
	// start a fresh counter.
	second := postJSON(t, srv, `{"email":"person@example.com"}`)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("repeated address (different case): status = %d, want 429", second.StatusCode)
	}
}

// TestJSONFieldRule_SkippedWhenFieldMissingOrBodyNotJSON proves
// JSONFieldRule is skipped -- not counted against an empty key -- for a
// request with no such field, an empty value, or a body that isn't JSON
// at all.
func TestJSONFieldRule_SkippedWhenFieldMissingOrBodyNotJSON(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_email_missing", []ratelimit.Rule{ratelimit.JSONFieldRule("email", 1, time.Hour)})(okHandler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	for _, body := range []string{`{}`, `{"email":""}`, `{"email":123}`, `not json`} {
		resp := postJSON(t, srv, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("body %q: status = %d, want 200 (skipped, not refused)", body, resp.StatusCode)
		}
	}
}

// TestHashedJSONFieldRule_KeysByFieldDigest proves HashedJSONFieldRule
// counts by the named field's value, independently per value, without
// storing that value itself as the bucket key -- #613's verification and
// reset tokens.
func TestHashedJSONFieldRule_KeysByFieldDigest(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_token", []ratelimit.Rule{ratelimit.HashedJSONFieldRule("token", 1, time.Hour)})(okHandler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	first := postJSON(t, srv, `{"token":"token-a"}`)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("token-a, request 1: status = %d, want 200", first.StatusCode)
	}

	other := postJSON(t, srv, `{"token":"token-b"}`)
	_ = other.Body.Close()
	if other.StatusCode != http.StatusOK {
		t.Fatalf("token-b, request 1: status = %d, want 200 (must not share token-a's counter)", other.StatusCode)
	}

	second := postJSON(t, srv, `{"token":"token-a"}`)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("token-a, request 2: status = %d, want 429", second.StatusCode)
	}

	var refusedKey string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT key_value FROM rate_limit_refusals WHERE endpoint = 'test_token'`,
	).Scan(&refusedKey); err != nil {
		t.Fatalf("query refusal: %v", err)
	}
	if refusedKey == "token-a" {
		t.Fatal("refusal logged the raw token, want its digest")
	}
}

// TestJSONFieldRule_NilBodySkipped proves a request with no body at all
// (nil, as opposed to empty) is skipped rather than causing a panic or a
// spurious 500.
func TestJSONFieldRule_NilBodySkipped(t *testing.T) {
	db := testdb.New(t)
	handler := ratelimit.Wrap(db.App, "test_nil_body", []ratelimit.Rule{ratelimit.JSONFieldRule("email", 1, time.Hour)})(okHandler())

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	req.Body = nil
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
