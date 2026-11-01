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

// freshSignInWindow is how recent the sign-in behind a step-up's token
// must be. The check page signs in again with the password and a code
// the moment the operator presses the act's button, so five minutes
// covers that round trip, as staffauth's recentAuthWindow does, while a
// token cached from the session's own sign-in cannot pass.
const freshSignInWindow = 5 * time.Minute

// stepUpLimit is how long a step-up waits to admit the one act it is
// for (spendStepUp). The check page steps up and sends the act in one
// press, so five minutes is ample, and short enough that a session left
// open does not keep the power (ADR-0062).
const stepUpLimit = 5 * time.Minute

const (
	msgStepUpRefused  = "Enter your password and the code your authenticator app shows now."
	msgStepUpRequired = "Enter the code your authenticator app shows now to do this."
)

// Why a step-up was refused, as its step_up_refused act records it.
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

// stepUp stamps the session's step_up_at once the operator proves a
// fresh authenticator-app code: a Bearer token from a sign-in just now,
// by the session's own operator, that passed a second factor. A code
// Identity Platform refused leaves the browser with no token, so it
// arrives here with none and is recorded like any other refusal. No
// idempotency key: a repeated stamp is the same stamp, and each refusal
// is its own try.
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
	// #773 exemption, as staffauth/reauth.go's: auth_time is Identity
	// Platform's real wall time, so it is read against real time, never
	// a simulated clock the sign-in never ran on.
	if time.Since(identity.AuthTime) > freshSignInWindow { //nolint:forbidigo // #773: see above
		return refusedStaleSignIn
	}
	return ""
}

// requireStepUp admits an act on a session stepped up within
// stepUpLimit, or a retry of an act this operator already took under the
// request's Idempotency-Key, which replays and needs no new code.
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

// errStepUpSpent is spendStepUp's answer when the stamp the act was
// admitted on is no longer the session's: another act spent it first, or
// a newer step-up replaced it.
var errStepUpSpent = errors.New("the step-up this act was admitted on is spent")

// spendStepUp clears the session's stamp inside the act's own
// transaction, only while it is still the one the act was admitted on,
// so each code admits one act (ADR-0062) and two acts sent on one stamp
// cannot both pass: the second waits on the first's row lock and finds
// the stamp gone. On errStepUpSpent the act answers writeStepUpRequired.
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

// writeStepUpRequired is the refusal of an act whose session has no
// unspent step-up within stepUpLimit.
func writeStepUpRequired(w http.ResponseWriter) {
	apierr.Write(w, http.StatusForbidden, apierr.CodeSecondFactorRequired, msgStepUpRequired, nil)
}
