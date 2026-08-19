package staffauth

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/session"
)

// AcceptInviteRequest is the body of an accept-invite request: the
// one-time token from InviteResponse.
type AcceptInviteRequest struct {
	InviteToken string `json:"inviteToken"`
}

// AcceptInviteResponse identifies the Staff row the invite claimed. The
// frontend follows this up with GET /api/staff/session, same as after
// login, to decide where to land.
type AcceptInviteResponse struct {
	StaffID string `json:"staffId"`
}

// AcceptInviteHandler lets an invited person claim their pending Staff
// row: it verifies the caller's Identity Platform token, then sets
// identity_uid on the staff row matching the presented invite token.
// Like SignupHandler and SessionHandler, this runs before any Practice is
// chosen, so it never sets app.current_practice_id.
func AcceptInviteHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.Begin(w, r, verifier, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req AcceptInviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.InviteToken = strings.TrimSpace(req.InviteToken)
		if req.InviteToken == "" {
			http.Error(w, "inviteToken is required", http.StatusBadRequest)
			return
		}

		resp, status, msg := acceptInvite(r, tx, uid, req.InviteToken)
		if status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}

		// Create the session before committing, so a failure rolls the
		// new rows back instead of leaving them committed behind a
		// response that reports failure (#145). uid is the identity
		// authn.Begin already verified.
		cookie, err := session.BuildCookie(r.Context(), tx, uid)
		if err != nil {
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
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func acceptInvite(r *http.Request, tx *sql.Tx, identityUID, inviteToken string) (AcceptInviteResponse, int, string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.invite_token', $1, true)`, inviteToken); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE staff SET identity_uid = $1, invite_token = NULL WHERE invite_token = $2`,
		identityUID, inviteToken,
	)
	if isUniqueViolation(err) {
		return AcceptInviteResponse{}, http.StatusConflict, "a staff account already exists for this identity"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	if rows == 0 {
		return AcceptInviteResponse{}, http.StatusNotFound, "invite not found or already accepted"
	}

	var staffID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM staff WHERE identity_uid = $1`, identityUID).Scan(&staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return AcceptInviteResponse{StaffID: staffID}, http.StatusOK, ""
}
