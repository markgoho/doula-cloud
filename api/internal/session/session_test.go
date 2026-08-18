package session_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/session"
)

var (
	errBadToken = errors.New("invalid token")
	errMintFail = errors.New("mint failed")
)

func newServer(verifier authntest.Verifier) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /session", session.CreateHandler(verifier))
	mux.Handle("DELETE /session", session.EndHandler())
	return httptest.NewServer(mux)
}

func doRequest(t *testing.T, srv *httptest.Server, method, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// sessionCookie returns the __session cookie from resp, or nil if none
// was set.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func TestCreateHandler_MissingToken(t *testing.T) {
	srv := newServer(authntest.Verifier{})
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on missing token: %+v", c)
	}
}

func TestCreateHandler_InvalidToken(t *testing.T) {
	srv := newServer(authntest.Verifier{Err: errBadToken})
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "bad-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on invalid token: %+v", c)
	}
}

func TestCreateHandler_MintFailure(t *testing.T) {
	srv := newServer(authntest.Verifier{UID: "uid-1", MintErr: errMintFail})
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on mint failure: %+v", c)
	}
}

// TestCreateHandler_Success drives #144's core case: a valid ID token
// gets a __session cookie with the right attributes. It deliberately
// never asserts on the cookie's value -- that's an opaque server-signed
// string.
func TestCreateHandler_Success(t *testing.T) {
	srv := newServer(authntest.Verifier{UID: "uid-1"})
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "good-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if !c.Secure {
		t.Error("cookie is not Secure")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
	wantMaxAge := int(session.Lifetime.Seconds())
	if c.MaxAge != wantMaxAge {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, wantMaxAge)
	}
}

// TestEndHandler_ClearsCookie proves ending a session clears the cookie
// by setting an expiring one under the same name.
func TestEndHandler_ClearsCookie(t *testing.T) {
	srv := newServer(authntest.Verifier{})
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodDelete, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set")
	}
	if c.MaxAge >= 0 {
		t.Errorf("MaxAge = %d, want negative (expire immediately)", c.MaxAge)
	}
}

// TestEndHandler_NoSessionStillSucceeds proves end-session is idempotent:
// calling it with no session cookie at all still clears the cookie and
// reports success.
func TestEndHandler_NoSessionStillSucceeds(t *testing.T) {
	srv := newServer(authntest.Verifier{Err: errBadToken})
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if c := sessionCookie(resp); c == nil {
		t.Fatal("no __session cookie set")
	}
}
