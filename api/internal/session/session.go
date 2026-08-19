// Package session implements the two endpoints that bracket a browser
// session (#144): exchanging a verified ID token for an HttpOnly session
// cookie, and clearing that cookie on sign-out. Neither endpoint is
// scoped to a Practice or an Engagement -- both serve the Staff
// population and the Client population, so this package sits alongside
// staffauth and clientauth rather than inside either.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
)

// CookieName is the session cookie's name -- see authn.SessionCookieName
// for why it must be exactly this value. Aliased here so callers that
// only need the cookie's name, not the Verifier interface or
// BearerToken, don't have to import authn for it.
const CookieName = authn.SessionCookieName

// Lifetime is how long a newly minted session cookie is valid for. #138
// fixes this at 12 hours for both populations; #147 renews it on use,
// which this ticket does not implement. Exported so tests can assert a
// cookie's MaxAge against this constant instead of a repeated literal.
const Lifetime = 12 * time.Hour

// MsgInternalError is the body a caller sees for a failure that carries
// no more specific detail.
const MsgInternalError = "internal error"

// StatusResponse is the minimal success body both endpoints return --
// neither needs to tell the caller anything beyond "this worked".
type StatusResponse struct {
	OK bool `json:"ok"`
}

// CreateHandler accepts an ID token in the Authorization header --
// still Bearer at this seam, because no session cookie exists yet --
// verifies it, and sets the session cookie. This is the one place a
// Bearer ID token is still read once #138 lands elsewhere.
func CreateHandler(verifier authn.Verifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idToken, ok := authn.BearerToken(r)
		if !ok {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		if _, err := verifier.VerifyIDToken(r.Context(), idToken); err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		if err := SetCookie(r.Context(), w, verifier, idToken); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		writeStatus(w)
	})
}

// SetCookie mints a session cookie for idToken and sets it on w, with the
// same name, attributes, and lifetime CreateHandler uses.
func SetCookie(ctx context.Context, w http.ResponseWriter, verifier authn.Verifier, idToken string) error {
	cookie, err := BuildCookie(ctx, verifier, idToken)
	if err != nil {
		return err
	}
	http.SetCookie(w, cookie)
	return nil
}

// BuildCookie mints a session cookie for idToken and returns it, with the
// same name, attributes, and lifetime CreateHandler's cookie carries, but
// without writing it to a response. It is the entry point the bootstrap
// endpoints (#145: Staff signup, Staff invitation acceptance, Client
// portal invitation acceptance) use: each mints the cookie before
// committing its own transaction, so a mint failure rolls the transaction
// back instead of leaving committed rows behind a response that reports
// failure. A newly created or accepted person then lands signed in
// without a follow-up call to CreateHandler.
func BuildCookie(ctx context.Context, verifier authn.Verifier, idToken string) (*http.Cookie, error) {
	cookieValue, err := verifier.MintSessionCookie(ctx, idToken, Lifetime)
	if err != nil {
		return nil, fmt.Errorf("session: mint session cookie: %w", err)
	}

	return &http.Cookie{
		Name:     CookieName,
		Value:    cookieValue,
		Path:     "/",
		MaxAge:   int(Lifetime.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

// EndHandler clears the session cookie. It is idempotent: called with no
// cookie, an expired one, or a revoked one, it still clears the cookie
// and reports success -- the cookie is the browser's only credential, so
// nothing about that browser's state can make clearing it fail. It does
// not revoke Identity Platform refresh tokens; ending every session for
// a person is a separate administrative action (#154).
func EndHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     CookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		writeStatus(w)
	})
}

func writeStatus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(StatusResponse{OK: true}); err != nil {
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
	}
}
