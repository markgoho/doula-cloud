// Package session implements the two endpoints that bracket a browser
// session (#144): exchanging a verified ID token for an HttpOnly session
// cookie, and clearing that cookie on sign-out. Neither endpoint is
// scoped to a Practice or an Engagement -- both serve the Staff
// population and the Client population, so this package sits alongside
// staffauth and clientauth rather than inside either. The session itself
// -- token, row, expiry -- belongs to authn (ADR-0004); this package is
// only its HTTP surface.
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionmint"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/tasknudge"
)

// CookieName is the session cookie's name -- see authn.SessionCookieName
// for why it must be exactly this value. Aliased here so callers that
// only need the cookie's name, not the Verifier interface or
// BearerToken, don't have to import authn for it.
const CookieName = authn.SessionCookieName

// Lifetime is how long a newly created or renewed Staff session is valid
// for -- see authn.SessionLifetime, which owns the value. CreateHandler
// below is Staff-only (Client sign-in mints through portalinvite and
// clientauth instead, at authn.PortalSessionLifetime -- #618, ADR-0026),
// so this alias is sound for it. Exported so tests can assert a cookie's
// MaxAge against this constant instead of a repeated literal.
const Lifetime = authn.SessionLifetime

// MsgInternalError is the body a caller sees for a failure that carries
// no more specific detail.
const MsgInternalError = "internal error"

// StatusResponse is the minimal success body both endpoints return --
// neither needs to tell the caller anything beyond "this worked".
type StatusResponse struct {
	OK bool `json:"ok"`
}

// CreateHandler accepts an ID token in the Authorization header --
// still Bearer at this seam, because no session exists yet -- verifies
// it with the identity provider, creates the session, and sets the
// cookie. Since #151 removed the Bearer path from authn.Begin, the only
// other places an ID token is read are the three bootstrap endpoints,
// via authn.BeginBootstrap.
//
// This is one of the seven seams #837's sessionmint.Issue guards: a
// browser holds exactly one session, so signing in here with a live
// *portal* session overwrites it. That is refused once with a warning
// and only done on the retry that confirms it -- see authn/eviction.go.
// The eviction, the notice it queues and the new session all ride one
// transaction, which is why this no longer mints straight on the pool:
// an evicted row and an unminted replacement would leave the browser
// holding a cookie that verifies as nothing.
func CreateHandler(verifier authn.Verifier, db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idToken, ok := authn.BearerToken(r)
		if !ok {
			apierr.WriteError(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		verified, err := verifier.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			apierr.WriteError(w, "invalid token", http.StatusUnauthorized)
			return
		}

		step := func(context.Context, *sql.Tx) (sessionmint.Result, error) {
			return sessionmint.Result{IdentityUID: verified.UID, Body: StatusResponse{OK: true}}, nil
		}
		if !sessionmint.IssueFromDB(w, r, db, enq, sessionmint.Staff(*verified), step, nil) {
			return
		}

		// Best-effort, same as EndHandler's swallowed EndSession below: a
		// failed notice queue must never turn a legitimate sign-in into a
		// 500 (#345).
		_ = sessionnotice.QueueNewSignInIfDue(r.Context(), db, verified.UID, time.Now(), enq)
	})
}

// EndHandler deletes the caller's session row and clears the cookie. It
// is idempotent: called with no cookie, or one naming a session that has
// already ended or expired, it still clears the cookie and reports
// success. It ends only the session in this browser -- the same person's
// sessions elsewhere are separate rows and are untouched; ending every
// session for a person is a distinct action, whether an Owner's
// administrative revoke (#154, staffauth.EndSessionsHandler) or a
// Client's own self-service (#618, clientauth.EndAllSessionsHandler).
func EndHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(CookieName); err == nil {
			// A failed DELETE is not reported: the browser is losing its
			// only copy of the token either way, and the row expires
			// within SessionLifetime regardless. Failing sign-out would
			// leave the caller signed in with no way out.
			_ = authn.EndSession(r.Context(), db, cookie.Value)
		}

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
		apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
	}
}
