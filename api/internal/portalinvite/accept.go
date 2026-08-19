package portalinvite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/authn"
)

// AcceptInviteRequest is the body of an accept-invite request: the
// one-time token from InviteResponse.
type AcceptInviteRequest struct {
	InviteToken string `json:"inviteToken"`
}

// AcceptInviteResponse identifies the Client the invite claimed. The
// frontend follows this up with GET /api/portal/session, same as after
// login, to decide where to land.
type AcceptInviteResponse struct {
	ClientID string `json:"clientId"`
}

// AcceptInviteHandler lets an invited Client claim their pending
// client_portal_users row: it verifies the caller's Identity Platform
// token, then sets identity_uid on the row matching the presented invite
// token. Like clientauth.SessionHandler, this runs before any Client is
// resolved, so it never sets app.current_client_id -- mirrors
// staffauth.AcceptInviteHandler's shape.
func AcceptInviteHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, ok := authn.BeginBootstrap(w, r, verifier, db)
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
			writeAPIError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid request body")
			return
		}
		req.InviteToken = strings.TrimSpace(req.InviteToken)
		if req.InviteToken == "" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "inviteToken is required")
			return
		}

		resp, status, code, msg := acceptInvite(r, tx, uid, req.InviteToken)
		if status != http.StatusOK {
			writeAPIError(w, status, code, msg)
			return
		}

		// Create the session before committing, so a failure rolls the
		// new rows back instead of leaving them committed behind a
		// response that reports failure (#145). uid is the identity
		// authn.Begin already verified.
		cookie, err := authn.MintSession(r.Context(), tx, uid, time.Now())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, codeInternalError, MsgInternalError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			writeAPIError(w, http.StatusInternalServerError, codeInternalError, MsgInternalError)
			return
		}
		committed = true

		http.SetCookie(w, cookie)

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			writeAPIError(w, http.StatusInternalServerError, codeInternalError, MsgInternalError)
		}
	})
}

func acceptInvite(r *http.Request, tx *sql.Tx, identityUID, inviteToken string) (resp AcceptInviteResponse, status int, code, msg string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.invite_token', $1, true)`, inviteToken); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE client_portal_users SET identity_uid = $1, invite_token = NULL WHERE invite_token = $2`,
		identityUID, inviteToken,
	)
	if isUniqueViolation(err) {
		return AcceptInviteResponse{}, http.StatusConflict, "CONFLICT", "a portal account already exists for this identity"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
	}
	rows, err := result.RowsAffected()
	if err != nil {
		// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
	}
	if rows == 0 {
		return AcceptInviteResponse{}, http.StatusNotFound, "NOT_FOUND", "invite not found or already accepted"
	}

	var clientID string
	if err := tx.QueryRowContext(ctx, `SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identityUID).Scan(&clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
	}

	return AcceptInviteResponse{ClientID: clientID}, http.StatusOK, "", ""
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), mirroring staffauth.isUniqueViolation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
