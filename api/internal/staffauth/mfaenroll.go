package staffauth

import (
	"context"
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionmint"
	"doula-cloud/api/internal/tasknudge"
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
func FinishEnrollmentHandler(verifier authn.Verifier, db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
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
			apierr.WriteError(w, "that sign-in does not show a second factor", http.StatusBadRequest)
			return
		}

		step := func(ctx context.Context, tx *sql.Tx) (sessionmint.Result, error) {
			staffID, found, err := setIdentityAndResolveStaff(ctx, tx, verified.UID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return sessionmint.Result{}, err
			}
			if !found {
				return sessionmint.Result{Refusal: &sessionmint.Refusal{
					Status: http.StatusForbidden, Message: MsgNoMatchingStaffAccount,
				}}, nil
			}
			if err := recordAuthEvent(ctx, tx, staffID, AuthEventEnrolled, staffID, ""); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return sessionmint.Result{}, err
			}
			return sessionmint.Result{IdentityUID: verified.UID, Body: struct {
				OK bool `json:"ok"`
			}{true}}, nil
		}

		// #816's trap: this seam always minted over its own
		// pre-enrolment session (a live cookie of the same tier, since
		// enrolment never crosses populations), which sessionmint.Issue
		// now ends silently for every seam rather than only here -- see
		// its own doc comment. A live *portal* session in this browser
		// is not that case, and is asked about exactly like every other
		// cross-population mint, replacing the unconditional EndSession
		// this handler used to run regardless of tier (#816's own AC).
		committed = sessionmint.Issue(w, r, tx, enq, sessionmint.Staff(verified), step, nil)
	})
}
