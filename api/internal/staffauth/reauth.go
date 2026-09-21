package staffauth

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// recentAuthWindow is how fresh a re-authentication must be for
// RequireRecentAuth to accept it. #605's Owner-vouch AC asks for
// re-authentication before an Owner may vouch; five minutes is enough
// for the client-side reauthenticateWithCredential() round trip plus
// filling in a password, without being so wide that a stale cached ID
// token from earlier in the session would pass.
const recentAuthWindow = 5 * time.Minute

// RequireRecentAuth guards a sensitive action (#615's Owner-vouch AC)
// with a genuine step-up check, distinct from RequireConfirmed's
// client-signaled confirmation. The caller must present a *fresh*
// Bearer ID token -- obtained by the client calling Identity Platform's
// own reauthenticateWithCredential() immediately before this request --
// alongside the session cookie Middleware already verified.
//
// Three things must all hold: the Bearer token verifies at all; its UID
// matches the already-authenticated session's own identity (so a caller
// cannot step up as anyone but herself); and its AuthTime is within
// recentAuthWindow of now (so a token cached from sign-in an hour ago
// cannot be replayed as "recent"). Any failure writes 401 and returns
// ok=false; the caller must not proceed.
//
// Reading a Bearer token here is a deliberate, narrow exception to
// authn.Begin's "no route behind Begin accepts a Bearer ID token" rule
// (#151): that rule retired the Bearer path as an alternative way to
// authenticate a whole request, which this is not -- the session cookie
// still does that. This reads a second, additional credential proving
// recency, for one specific action, the same way a bank asks for a
// re-entered password before a wire transfer despite an already-valid
// session.
func RequireRecentAuth(w http.ResponseWriter, r *http.Request, verifier authn.Verifier, tx *sql.Tx, staffID string) (ok bool) {
	idToken, present := authn.BearerToken(r)
	if !present {
		apierr.WriteError(w, "this action requires a fresh sign-in", http.StatusUnauthorized)
		return false
	}

	verified, err := verifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		apierr.WriteError(w, "this action requires a fresh sign-in", http.StatusUnauthorized)
		return false
	}

	var sessionIdentityUID string
	if err := tx.QueryRowContext(r.Context(), `SELECT identity_uid FROM staff WHERE id = $1`, staffID).Scan(&sessionIdentityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if verified.UID != sessionIdentityUID {
		apierr.WriteError(w, "this action requires a fresh sign-in", http.StatusUnauthorized)
		return false
	}

	// #773 exemption: verified.AuthTime is a real Identity Platform token
	// claim, the same real wall-clock fact FirebaseVerifier.VerifyIDToken
	// exempts (see its own doc comment, citing #762). Comparing it against
	// clock.Now instead of time.Now would reproduce that exact bug: under
	// a simulated jump, a token minted seconds ago in the real world would
	// read as arbitrarily stale (or fresh) against a clock the real sign-in
	// never ran on. time.Since stays real for the same reason the token
	// verification two lines up does.
	if time.Since(verified.AuthTime) > recentAuthWindow {
		apierr.WriteError(w, "this action requires a fresh sign-in", http.StatusUnauthorized)
		return false
	}

	return true
}
