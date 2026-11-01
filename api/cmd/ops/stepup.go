package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clock"
)

// The check page signs in again on the button press; a token cached from the session's own sign-in must not pass.
const freshSignInWindow = 5 * time.Minute

const stepUpLimit = 5 * time.Minute

const (
	msgStepUpRefused  = "Enter your password and the code your authenticator app shows now."
	msgStepUpRequired = "Enter the code your authenticator app shows now to do this."
)

const (
	refusedNoSignIn      = "no_sign_in"
	refusedNotVerified   = "not_verified"
	refusedOtherOperator = "other_operator"
	refusedNoCode        = "no_authenticator_code"
	refusedStaleSignIn   = "stale_sign_in"
)

type stepUpRefusal struct {
	Reason string `json:"reason"`
}

// A code Identity Platform refused leaves the browser with no token, so it arrives as none and is recorded as a refusal.
func stepUp(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		op := signedIn(r)
		now := clock.Now(r.Context())
		if reason := stepUpRefusalReason(r, d, op); reason != "" {
			if _, err := recordAct(r.Context(), d.DB, op, actStepUpRefused, nil, stepUpRefusal{Reason: reason}, nil, now); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				writeInternal(w)
				return
			}
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgStepUpRefused, nil)
			return
		}
		if _, err := d.DB.ExecContext(r.Context(),
			`UPDATE operator_sessions SET step_up_at = $2 WHERE token_hash = $1`,
			tokenHash(op.token), now); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func stepUpRefusalReason(r *http.Request, d Deps, op operator) string {
	idToken, ok := authn.BearerToken(r)
	if !ok {
		return refusedNoSignIn
	}
	identity, err := d.Verifier.Verify(r.Context(), idToken)
	if err != nil {
		return refusedNotVerified
	}
	if identity.UID != op.identityUID {
		return refusedOtherOperator
	}
	if !passedSecondFactor(identity.SecondFactor, d.AcceptEmulatorFactor) {
		return refusedNoCode
	}
	if time.Since(identity.AuthTime) > freshSignInWindow { //nolint:forbidigo // #773: auth_time is Identity Platform's real wall time, never a simulated clock
		return refusedStaleSignIn
	}
	return ""
}

// A retry under the act's Idempotency-Key replays and needs no new code.
func requireStepUp(db *sql.DB, action string, next http.Handler) http.Handler {
	return requireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := signedIn(r)
		if op.StepUpAt != nil && clock.Now(r.Context()).Before(op.StepUpAt.Add(stepUpLimit)) {
			next.ServeHTTP(w, r)
			return
		}
		if key := r.Header.Get(headerIdempotencyKey); key != "" {
			replay, err := isReplay(r.Context(), db, op, action, key)
			if err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				writeInternal(w)
				return
			}
			if replay {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeStepUpRequired(w)
	}))
}

var errStepUpSpent = errors.New("the step-up this act was admitted on is spent")

// Cleared only while still the admitting stamp, so two acts on one stamp cannot both pass: the second waits on the row lock and finds it gone.
func spendStepUp(ctx context.Context, tx *sql.Tx, op operator) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE operator_sessions SET step_up_at = NULL WHERE token_hash = $1 AND step_up_at = $2`,
		tokenHash(op.token), op.StepUpAt)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("spend step-up: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errStepUpSpent
	}
	return nil
}

func writeStepUpRequired(w http.ResponseWriter) {
	apierr.Write(w, http.StatusForbidden, apierr.CodeSecondFactorRequired, msgStepUpRequired, nil)
}
