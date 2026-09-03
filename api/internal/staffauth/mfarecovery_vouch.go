package staffauth

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/mfarecoverymail"
	"doula-cloud/api/internal/tasknudge"
)

// VouchHandler lets a Practice Owner vouch for any Staff member holding
// a Membership at her Practice (#605 §4.2.1.3, #615's AC): a single-use,
// 24-hour recovery code is minted for the target and delivered to the
// *Owner's own* address, never the target's -- she is a recovery
// contact, not someone clearing the factor herself. Owner-only (not
// Admin, matching #167's "the Owner throws the switch"), requires both
// RequireConfirmed's client-signalled confirmation and
// RequireRecentAuth's genuine step-up re-authentication, since this is
// the one recovery path an already-signed-in person can trigger for
// someone else. Must be mounted behind staffauth.Middleware.
func VouchHandler(verifier authn.Verifier, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		ownerStaffID, _ := StaffID(r.Context())
		if !RequireRecentAuth(w, r, verifier, tx, ownerStaffID) {
			return
		}
		if !RequireConfirmed(w, r) {
			return
		}

		targetStaffID := r.PathValue("staffId")
		if !ParseUUID(w, "staff", targetStaffID) {
			return
		}

		var targetIdentityUID string
		err := tx.QueryRowContext(r.Context(),
			`SELECT s.identity_uid FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1 AND pm.staff_id = $2`,
			practiceID, targetStaffID,
		).Scan(&targetIdentityUID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		code, err := authtoken.MintCode(r.Context(), tx, targetIdentityUID, authtoken.PurposeStaffMFARecovery, mfarecoverymail.CodeLifetime, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO staff_mfa_recovery_vouches (token_hash, staff_id, owner_staff_id) VALUES ($1, $2, $3)`,
			authtoken.Digest(code), targetStaffID, ownerStaffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		var ownerIdentityUID string
		if err := tx.QueryRowContext(r.Context(), `SELECT identity_uid FROM staff WHERE id = $1`, ownerStaffID).Scan(&ownerIdentityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := mfarecoverymail.QueueVouchedCodeMail(r.Context(), tx, ownerIdentityUID, targetStaffID, code); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.MFARecoveryCode))

		w.WriteHeader(http.StatusNoContent)
	})
}
