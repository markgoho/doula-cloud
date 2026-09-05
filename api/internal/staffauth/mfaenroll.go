package staffauth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
)

// FinishEnrollmentHandler lets a signed-in Staff member exchange a
// just-enrolled ID token for a session that shows her new second factor,
// and records the enrolment in #615's audit table (staff_auth_events).
//
// #606 decision 4: firebase.sign_in_second_factor describes the
// sign-in event, so the session she is already holding keeps saying "no
// second factor" for its whole 12 hours no matter what she enrols
// mid-session -- only replacing the session fixes that. This is
// deliberately a separate endpoint from POST /api/session (an ordinary
// sign-in that happens to carry the claim), so enrolling is its own
// auditable act rather than indistinguishable from any other re-sign-in.
//
// Reachable from both entry points the AC requires: a refusal driving
// her into enrolment, and voluntary enrolment from account settings --
// both end here once the client-side TOTP enroll() call succeeds and the
// SDK hands back a fresh ID token. Self-only, same "no {practiceId}, no
// staff id" shape as UpdateWorkStateHandler: enrolment is per person
// (#606's brief), not per Practice.
func FinishEnrollmentHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, verified, ok := authn.BeginBootstrap(w, r, verifier, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if !verified.SecondFactor {
			// The token this call was handed does not show a second
			// factor -- enroll() did not actually finish, or the token
			// is stale. Decision 4's fallback is for the client to sign
			// her in again through the TOTP challenge instead; this
			// endpoint's job is only the case where it did work.
			http.Error(w, "that sign-in does not show a second factor", http.StatusBadRequest)
			return
		}

		staffID, found, err := setIdentityAndResolveStaff(r.Context(), tx, verified.UID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, MsgNoMatchingStaffAccount, http.StatusForbidden)
			return
		}

		// Replace, don't leave in place: if this browser already held a
		// pre-enrolment session, end it now rather than letting it sit
		// alongside the new one until it is swept. Not present on the
		// refusal-driven path (#606's app.ts routes there before any
		// session is even readable), so a missing cookie is not an
		// error.
		if oldCookie, err := r.Cookie(authn.SessionCookieName); err == nil {
			if err := authn.EndSession(r.Context(), tx, oldCookie.Value); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		cookie, err := authn.MintSession(r.Context(), tx, verified.UID, true, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordAuthEvent(r.Context(), tx, staffID, AuthEventEnrolled, staffID, ""); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		http.SetCookie(w, cookie)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			OK bool `json:"ok"`
		}{true})
	})
}
