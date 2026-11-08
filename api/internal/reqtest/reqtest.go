// Package reqtest is the one place a route test builds an HTTP request,
// sends it and reads the response back, so a test names what it asks for
// (a method, a URL, a session, a body) and not the six lines of plumbing
// around it (#2743). Every failure ends the calling test where the helper
// was called.
package reqtest

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
)

type request struct {
	session     string
	body        []byte
	contentType string
	headers     [][2]string
	err         error
}

// Option shapes the request Do sends.
type Option func(*request)

// Session signs the request in with the cookie authntest.SeedSession
// returned. An empty token adds no cookie, so a test can pass the same
// variable for the signed-in and the signed-out case.
func Session(token string) Option {
	return func(r *request) { r.session = token }
}

// JSON sends v marshaled as the JSON body.
func JSON(v any) Option {
	return func(r *request) {
		r.contentType = "application/json"
		r.body, r.err = json.Marshal(v)
	}
}

// RawJSON sends payload as-is under a JSON content type, for a test whose
// subject is a body that does not decode.
func RawJSON(payload []byte) Option {
	return Body("application/json", payload)
}

// Body sends payload as-is under contentType.
func Body(contentType string, payload []byte) Option {
	return func(r *request) {
		r.contentType = contentType
		r.body = payload
	}
}

// Header sets one request header.
func Header(key, value string) Option {
	return func(r *request) { r.headers = append(r.headers, [2]string{key, value}) }
}

// HeaderIf sets the header only when cond holds, so a test that is
// table-driven over the header being present passes one option for both cases.
func HeaderIf(cond bool, key, value string) Option {
	if !cond {
		return func(*request) {}
	}
	return Header(key, value)
}

// Do sends one request with http.DefaultClient and returns the response.
// The caller owns resp.Body.
func Do(t *testing.T, method, url string, opts ...Option) *http.Response {
	t.Helper()
	var r request
	for _, opt := range opts {
		opt(&r)
	}
	if r.err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("reqtest: marshal body: %v", r.err)
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, body)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("reqtest: build request: %v", err)
	}
	if r.session != "" {
		authntest.AddSessionCookie(req, r.session)
	}
	if r.contentType != "" {
		req.Header.Set("Content-Type", r.contentType)
	}
	for _, h := range r.headers {
		req.Header.Set(h[0], h[1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("reqtest: request: %v", err)
	}
	return resp
}

// Get is Do with GET.
func Get(t *testing.T, url string, opts ...Option) *http.Response {
	t.Helper()
	return Do(t, http.MethodGet, url, opts...)
}

// Post is Do with POST.
func Post(t *testing.T, url string, opts ...Option) *http.Response {
	t.Helper()
	return Do(t, http.MethodPost, url, opts...)
}

// Put is Do with PUT.
func Put(t *testing.T, url string, opts ...Option) *http.Response {
	t.Helper()
	return Do(t, http.MethodPut, url, opts...)
}

// Delete is Do with DELETE.
func Delete(t *testing.T, url string, opts ...Option) *http.Response {
	t.Helper()
	return Do(t, http.MethodDelete, url, opts...)
}

// ReadBody returns the whole body and closes it.
func ReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("reqtest: read body: %v", err)
	}
	return body
}

// Decode reads the body as JSON into a T and closes it.
func Decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(ReadBody(t, resp), &out); err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("reqtest: decode response: %v", err)
	}
	return out
}

// RequireStatus fails the test, naming the body the response carried,
// unless resp carries want.
func RequireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, want, ReadBody(t, resp))
	}
}

// DecodeStatus is RequireStatus then Decode, for a test that asserts the
// status and reads the body in one step.
func DecodeStatus[T any](t *testing.T, resp *http.Response, want int) T {
	t.Helper()
	RequireStatus(t, resp, want)
	return Decode[T](t, resp)
}
