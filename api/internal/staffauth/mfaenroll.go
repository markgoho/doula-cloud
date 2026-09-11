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
			// The one route in the family that cannot call requireSelf:
			// it runs inside sessionmint's step, which owns the
			// response and takes a Refusal value rather than an
			// http.ResponseWriter. It reaches resolveSelf -- the same
			// owner requireSelf itself reaches -- and restates that
			// function's recorded status, 404, which this route
			// answered as 403 until #1182. selfResolvingRoutes drives
			// this route with a stranded credential like every other,
			// so the restatement cannot drift back.
			self, found, err := resolveSelf(ctx, tx, verified.UID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return sessionmint.Result{}, err
			}
			if !found {
				return sessionmint.Result{Refusal: &sessionmint.Refusal{
					Status: http.StatusNotFound, Message: MsgNoMatchingStaffAccount,
				}}, nil
			}
			staffID := self.ID
			if err := recordAuthEvent(ctx, tx, staffID, AuthEventEnrolled, staffID, ""); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				return sessionmint.Result{}, err
			}
			return sessionmint.Result{IdentityUID: verified.UID, Body: struct {
				OK bool `json:"ok"`
			}{true}}, nil
		}

		// #816's own AC: this seam used to mint over its own
		// pre-enrolment session with an unconditional EndSession,
		// regardless of tier -- so a live *portal* session in this
		// browser was silently deleted outright. ReplaceSameTier keeps
		// the silent replacement for this seam's own same-tier cookie
		// (always the same identity re-authenticating -- see
		// sessionmint.Issue's own doc comment for why this is the one
		// caller entitled to it) while routing a cross-tier cookie
		// through the same ask-first path every other seam uses.
		adapter := sessionmint.Staff(verified)
		adapter.ReplaceSameTier = true
		committed = sessionmint.Issue(w, r, tx, enq, adapter, step, nil)
	})
}
