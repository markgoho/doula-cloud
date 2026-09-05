package staffauth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
)

// savedCodeSetSize is how many saved recovery codes a sole Owner holds
// at once. #605's resolution cites GitHub, Slack, Okta and Google
// Workspace as the precedent for backup codes at all (Jakob's Law); ten
// is the count all four of those settled on, and nothing in #615's AC
// asks for a different number.
const savedCodeSetSize = 10

// hasLiveSavedCodes reports whether staffID holds any unspent,
// unrevoked saved code right now.
func hasLiveSavedCodes(ctx context.Context, tx *sql.Tx, staffID string) (bool, error) {
	var has bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM staff_mfa_recovery_codes WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL)`,
		staffID,
	).Scan(&has)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("staffauth: check live saved codes: %w", err)
	}
	return has, nil
}

// reconcileSavedCodes mints or revokes staffID's saved-code set to match
// her current sole-Owner status -- #615's AC: minted on the Membership
// event that makes her a Practice's sole Owner, revoked when a second
// Owner's Membership arrives. Every call site that can change who is a
// sole Owner (signup, invitation acceptance, a Membership edit, a
// Membership removal) calls this for the staff member whose Membership
// changed and, since one person's change can make or unmake someone
// else's sole ownership too, for every other Owner at the same Practice
// -- see reconcileOwnersAtPractice.
//
// Idempotent: calling it when nothing needs to change is a no-op past
// the two existence checks, so every call site can call it
// unconditionally rather than working out first whether it applies.
func reconcileSavedCodes(ctx context.Context, tx *sql.Tx, staffID string) error {
	sole, err := isSoleOwnerAnywhere(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	live, err := hasLiveSavedCodes(ctx, tx, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}

	switch {
	case sole && !live:
		return mintSavedCodeSet(ctx, tx, staffID)
	case !sole && live:
		if _, err := tx.ExecContext(ctx,
			`UPDATE staff_mfa_recovery_codes SET revoked_at = now() WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
			staffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("staffauth: revoke saved codes: %w", err)
		}
	}
	return nil
}

// reconcileOwnersAtPractice calls reconcileSavedCodes for targetStaffID
// and for every current Owner at practiceID -- the set of people whose
// sole-Owner status could have just changed. A Membership write site
// calls this once, after its own INSERT/UPDATE/DELETE has committed
// within the same tx, rather than working out by hand which of the two
// AC triggers (mint on becoming sole, revoke on a second Owner arriving)
// applies.
func reconcileOwnersAtPractice(ctx context.Context, tx *sql.Tx, practiceID, targetStaffID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT staff_id FROM practice_memberships WHERE practice_id = $1 AND 'owner' = ANY(roles)`,
		practiceID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: list practice owners: %w", err)
	}
	defer func() { _ = rows.Close() }()

	staffIDs := map[string]bool{targetStaffID: true}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("staffauth: scan practice owner: %w", err)
		}
		staffIDs[id] = true
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: list practice owners: %w", err)
	}

	for id := range staffIDs {
		if err := reconcileSavedCodes(ctx, tx, id); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}
	return nil
}

// mintSavedCodeSet mints a fresh set of savedCodeSetSize codes for
// staffID, discarding each plaintext immediately -- called only from
// reconcileSavedCodes, which runs at a Membership-event write site with
// no secure channel back to the person it concerns. RotateSavedCodesHandler
// is the only place a plaintext set is ever returned.
func mintSavedCodeSet(ctx context.Context, tx *sql.Tx, staffID string) error {
	for range savedCodeSetSize {
		if _, err := mintOneSavedCode(ctx, tx, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return err
		}
	}
	return nil
}

// mintOneSavedCode inserts one fresh saved code for staffID and returns
// its plaintext. rand.Text() gives 128+ bits of randomness -- comfortably
// past #605's §4.2.1.1 floor of 64 bits, and long enough that a global
// UNIQUE constraint on code_hash needs no identity-scoped lookup the way
// authtoken.MintCode's short decimal codes do; retried the same way
// regardless, since any collision, however unlikely, must not surface as
// a raw constraint-violation error.
func mintOneSavedCode(ctx context.Context, tx *sql.Tx, staffID string) (string, error) {
	const maxAttempts = 5
	for range maxAttempts {
		code := rand.Text()
		res, err := tx.ExecContext(ctx,
			`INSERT INTO staff_mfa_recovery_codes (staff_id, code_hash) VALUES ($1, $2) ON CONFLICT (code_hash) DO NOTHING`,
			staffID, authtoken.Digest(code),
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", fmt.Errorf("staffauth: insert saved code: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return code, nil
		}
	}
	// coverage:ignore reason: requires forcing a rand.Text() collision across maxAttempts draws, not exercised by unit tests
	return "", fmt.Errorf("staffauth: could not mint a unique saved code after %d attempts", maxAttempts)
}

// RotateSavedCodesResponse is the plaintext set RotateSavedCodesHandler
// returns -- the one moment any saved code's plaintext exists outside
// the person's own memory.
type RotateSavedCodesResponse struct {
	Codes []string `json:"codes"`
}

// RotateSavedCodesHandler lets a Practice's current sole Owner see her
// saved codes: it revokes whatever set she holds (spent, unspent, or
// none at all) and mints a fresh full set, returning every plaintext
// once. Self-only, the same "no {practiceId}, no staff id" shape
// PUT /api/staff/work-state uses (00043) -- who is sole Owner is a fact
// about the person, not a Membership -- and the one path by which she
// ever actually sees a saved code's plaintext: reconcileSavedCodes never
// surfaces one, including the replacement it mints when she spends one
// during recovery, so calling this again afterward is how she recovers
// visibility into her current set. 403s anyone who is not currently a
// sole Owner, matching the AC that only that population ever holds
// these.
func RotateSavedCodesHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// Self-only, like PUT /api/staff/work-state: no app.current_practice_id
		// is ever set here, and staff_self_visibility (00006) only admits a
		// row matching app.current_identity_uid, which authn.Begin does not
		// set either (#151 -- session ownership moved off Identity Platform
		// entirely). isSoleOwnerAnywhere also needs to read practice_memberships
		// across every Practice this person belongs to, which no per-Practice
		// policy can admit by construction. Same reuse of 00033's trust flag
		// as the two unauthenticated/internal MFA-recovery handlers.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		var staffID string
		err := tx.QueryRowContext(r.Context(), `SELECT id FROM staff WHERE identity_uid = $1`, uid).Scan(&staffID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, MsgNoMatchingStaffAccount, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		sole, err := isSoleOwnerAnywhere(r.Context(), tx, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !sole {
			apierr.WriteError(w, "saved recovery codes are only issued to a practice's sole owner", http.StatusForbidden)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE staff_mfa_recovery_codes SET revoked_at = now() WHERE staff_id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
			staffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		codes := make([]string, 0, savedCodeSetSize)
		for range savedCodeSetSize {
			code, err := mintOneSavedCode(r.Context(), tx, staffID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
			codes = append(codes, code)
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(RotateSavedCodesResponse{Codes: codes}); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}
