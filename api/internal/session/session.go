// Package session implements the two endpoints that bracket a browser
// session (#144): exchanging a verified ID token for an HttpOnly session
// cookie, and clearing that cookie on sign-out. Neither endpoint is
// scoped to a Practice or an Engagement -- both serve the Staff
// population and the Client population, so this package sits alongside
// staffauth and clientauth rather than inside either.
package session

import (
	"encoding/json"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
)

// CookieName is the session cookie's name. It must be exactly this
// value: since #139 the deployed app reaches the BFF through a Firebase
// Hosting rewrite of /api/** to Cloud Run, and Hosting strips every
// incoming Cookie header on that hop except one named exactly
// "__session". Any other name fails only in production -- local `vite
// dev` and the Playwright preview server both proxy /api without
// stripping anything.
const CookieName = "__session"

// lifetime is how long a newly minted session cookie is valid for. #138
// fixes this at 12 hours for both populations; #147 renews it on use,
// which this ticket does not implement.
const lifetime = 12 * time.Hour

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

		cookieValue, err := verifier.MintSessionCookie(r.Context(), idToken, lifetime)
		if err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     CookieName,
			Value:    cookieValue,
			Path:     "/",
			MaxAge:   int(lifetime.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
		writeStatus(w)
	})
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
