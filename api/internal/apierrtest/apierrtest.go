// Package apierrtest is the one place a test reads docs/api-design.md
// section 7's error envelope back off the wire, the way authntest is the
// one place a test builds an authn.Verifier double. #811 replaces three
// named copies of the same six lines (client.readAPIError,
// website.decodeError, staffauth.decodeRefusal) and about twenty inline
// copies, four of which decoded into a local anonymous struct of their
// own and so could not have noticed the envelope's JSON tags moving.
package apierrtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
)

// Decode reads resp's body as one section 7 error envelope, for an
// assertion that has to see a refusal's code, message or details rather
// than only its status. A body that will not decode fails the test where
// it is read, so a caller never has to answer an error it has nothing to
// do about.
//
// The body is left open: every call site already holds a
// `defer resp.Body.Close()` from where it made the request, and closing
// here would put a second Close on the same body.
//
// A recorder-based test passes rec.Result().
func Decode(t *testing.T, resp *http.Response) apierr.APIError {
	t.Helper()
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("decode section 7 error envelope: %v", err)
	}
	return out
}
