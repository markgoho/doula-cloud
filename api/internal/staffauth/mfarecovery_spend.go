package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
)

// msgMFARecoveryInvalid is the one message #615's spend endpoint ever
// returns for a failure -- an unknown address, a wrong code, or a code
// that was never issued to that address are the same outcome, per
// #168's account-enumeration rule.
const msgMFARecoveryInvalid = "this code is invalid or has expired"

// SpendMFARecoveryRequest is the body of a recovery-code spend: the
// account address and the code, decimal or opaque, either an
// Owner-vouched issued code or a sole Owner's own saved code.
type SpendMFARecoveryRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// SpendMFARecoveryHandler is #615's one unauthenticated endpoint for all
// three recovery paths' actual spend: it clears the named identity's
// TOTP enrolment and mints no session (#605's sequence -- Identity
// Platform challenges for the second factor on every sign-in once one
// exists, so a locked-out person cannot sign in first and spend a code
// afterward). The code alone is the credential, the same way
// SpendResetHandler's link token is: email exists only so an unknown
// address answers identically to a wrong code (#168), and so the
// per-account rate limit (#602) has something to key on before the code
// itself is ever looked at -- it is not cross-checked against which
// identity the code actually names, the same way a password-reset link
// is honoured for whoever holds it, not for whoever the request happened
// to come from.
//
// Tries the Owner-vouched issued code first (authtoken, purpose
// staff_mfa_recovery), then a sole Owner's own saved code -- a caller
// need not say which kind she holds.
func SpendMFARecoveryHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SpendMFARecoveryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := NormalizeAddress(req.Email)
		if address == "" || req.Code == "" {
			http.Error(w, "email and code are required", http.StatusBadRequest)
			return
		}

		// Resolved and discarded past this point on purpose -- see the
		// handler doc comment. Its only job is deciding whether an
		// unknown address short-circuits here, before any DB work the
		// code check would otherwise do, mirroring RequestResetHandler.
		if _, err := accounts.GetAccountByEmail(r.Context(), address); errors.Is(err, authn.ErrAccountNotFound) {
			http.Error(w, msgMFARecoveryInvalid, http.StatusBadRequest)
			return
		} else if err != nil {
			// coverage:ignore reason: Admin SDK failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// This request carries no session and never will (spending a
		// recovery code mints none) -- neither app.current_practice_id nor
		// app.current_identity_uid is ever set here, so staff_practice_visibility
		// and staff_self_visibility (00002, 00006) admit nothing. Reusing
		// 00033's notification_worker_trusted flag rather than inventing a
		// same-shaped policy: this endpoint is exactly that shape of
		// caller -- a trusted backend process with no session of its own --
		// even though it isn't a mail worker.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, identityUID, reason, actorStaffID, ok, err := spendAnyMFACode(r.Context(), tx, req.Code, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, msgMFARecoveryInvalid, http.StatusBadRequest)
			return
		}

		if err := clearEnrolmentAndRecord(r.Context(), tx, accounts, staffID, identityUID, reason, actorStaffID, ""); err != nil {
			// coverage:ignore reason: DB/Admin SDK failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.WriteHeader(http.StatusNoContent)
	})
}

// spendAnyMFACode tries code as an issued (Owner-vouched) recovery code
// first, then as a sole Owner's saved code, returning who it belongs to
// and which AuthEventReason applies. ok is false for a code that matches
// neither -- the caller's cue to answer with msgMFARecoveryInvalid.
func spendAnyMFACode(ctx context.Context, tx *sql.Tx, code string, now time.Time) (staffID, identityUID string, reason AuthEventReason, actorStaffID string, ok bool, err error) {
	identityUID, err = authtoken.Spend(ctx, tx, code, authtoken.PurposeStaffMFARecovery, now)
	if err == nil {
		if scanErr := tx.QueryRowContext(ctx, `SELECT id FROM staff WHERE identity_uid = $1`, identityUID).Scan(&staffID); scanErr != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", "", "", "", false, fmt.Errorf("staffauth: resolve issued-code identity: %w", scanErr)
		}
		owner, voucherErr := vouchingOwner(ctx, tx, authtoken.Digest(code))
		if voucherErr != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", "", "", "", false, voucherErr
		}
		return staffID, identityUID, AuthEventOwnerVouched, owner, true, nil
	}
	if !errors.Is(err, authtoken.ErrInvalid) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", "", "", false, fmt.Errorf("staffauth: spend issued code: %w", err)
	}

	staffID, ok, err = spendSavedCode(ctx, tx, code)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", "", "", false, err
	}
	if !ok {
		return "", "", "", "", false, nil
	}
	if scanErr := tx.QueryRowContext(ctx, `SELECT identity_uid FROM staff WHERE id = $1`, staffID).Scan(&identityUID); scanErr != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", "", "", false, fmt.Errorf("staffauth: resolve saved-code identity: %w", scanErr)
	}
	// Self-service: the sole Owner spending her own saved code is her
	// own actor, matching staff_auth_events_actor_shape.
	return staffID, identityUID, AuthEventSelfService, staffID, true, nil
}

// vouchingOwner reads which Owner vouched for tokenHash's issued code
// (staff_mfa_recovery_vouches, 00062) -- the actor staff_auth_events'
// eventual 'owner_vouched' row needs, carried from mint to spend since
// authtoken.Spend itself knows nothing beyond identity and purpose.
func vouchingOwner(ctx context.Context, tx *sql.Tx, tokenHash string) (string, error) {
	var ownerStaffID string
	err := tx.QueryRowContext(ctx, `SELECT owner_staff_id FROM staff_mfa_recovery_vouches WHERE token_hash = $1`, tokenHash).Scan(&ownerStaffID)
	if err != nil {
		// coverage:ignore reason: every issued code this package mints has a matching vouches row; a missing one is a DB-consistency failure, not exercised by unit tests
		return "", fmt.Errorf("staffauth: read vouching owner: %w", err)
	}
	return ownerStaffID, nil
}

// spendSavedCode marks one live saved code spent and mints its
// replacement (#605's §4.2.1.1: "a replacement issued whenever one is
// spent") in the same call. code_hash is globally UNIQUE, so this needs
// no staff_id to scope the lookup by -- the code itself already names
// exactly one person.
func spendSavedCode(ctx context.Context, tx *sql.Tx, code string) (staffID string, ok bool, err error) {
	err = tx.QueryRowContext(ctx,
		`UPDATE staff_mfa_recovery_codes SET used_at = now()
		 WHERE code_hash = $1 AND used_at IS NULL AND revoked_at IS NULL
		 RETURNING staff_id`,
		authtoken.Digest(code),
	).Scan(&staffID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("staffauth: spend saved code: %w", err)
	}
	if _, err := mintOneSavedCode(ctx, tx, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, err
	}
	return staffID, true, nil
}
