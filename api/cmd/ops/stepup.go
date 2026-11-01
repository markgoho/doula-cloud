package main

import (
	"database/sql"
	"encoding/json"
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

// stepUpLimit is how long a step-up admits an act that needs one. The
// check page steps up and sends the act in one press, so five minutes
// is enough to correct a refused act and send it again, and short
// enough that a session left open does not keep the power (ADR-0062).
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
			details, _ := json.Marshal(stepUpRefusal{Reason: reason})
			if _, err := d.DB.ExecContext(r.Context(),
				`INSERT INTO operator_acts (operator_id, action, details, created_at) VALUES ($1, 'step_up_refused', $2, $3)`,
				op.ID, details, now); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, msgStepUpRefused, nil)
			return
		}
		if _, err := d.DB.ExecContext(r.Context(),
			`UPDATE operator_sessions SET step_up_at = $2 WHERE token_hash = $1`,
			tokenHash(op.token), now); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
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

// requireStepUp admits an act only on a session stepped up within
// stepUpLimit, checked on the act's own request. The act reads the
// stamp it was admitted on from signedIn(r).StepUpAt, for its
// operator_acts row's step_up_at. It reads what requireSession already
// read, so the guard costs no statement of its own (#2432).
func requireStepUp(db *sql.DB, next http.Handler) http.Handler {
	return requireSession(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		at := signedIn(r).StepUpAt
		if at == nil || !clock.Now(r.Context()).Before(at.Add(stepUpLimit)) {
			apierr.Write(w, http.StatusForbidden, apierr.CodeSecondFactorRequired, msgStepUpRequired, nil)
			return
		}
		next.ServeHTTP(w, r)
	}))
}
