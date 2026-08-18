package portalinvite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/staffauth"
)

// InviteResponse identifies the pending client_portal_users row created (or
// re-invited) and hands back the one-time token the Client needs to accept
// -- there is no email-sending infrastructure yet (see
// 00004_staff_invitation.sql), so the Staff caller is expected to pass this
// along outside the app.
type InviteResponse struct {
	ClientPortalUserID string `json:"clientPortalUserId"`
	InviteToken        string `json:"inviteToken"`
}

// InviteHandler lets a Staff member scoped to a Practice invite the Client
// on the named Engagement to the Client portal. Per #90's scope decision,
// gating is practice-tier (any Staff member, not just an Owner, and not
// scoped to the specific Engagement) -- same tier as clients_insert in
// 00005_client_engagement.sql. Must be mounted behind staffauth.Middleware.
func InviteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		clientID, err := resolveEngagementClient(r.Context(), tx, engagementID, practiceID)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "NOT_FOUND", "engagement not found")
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			writeAPIError(w, http.StatusInternalServerError, codeInternalError, MsgInternalError)
			return
		}

		resp, status, code, msg := invite(r.Context(), tx, clientID)
		if status != http.StatusOK && status != http.StatusCreated {
			writeAPIError(w, status, code, msg)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			writeAPIError(w, http.StatusInternalServerError, codeInternalError, MsgInternalError)
		}
	})
}

// resolveEngagementClient confirms engagementID belongs to practiceID and
// returns its client_id, mirroring engagement.DetailHandler's join. Returns
// sql.ErrNoRows if the Engagement doesn't exist at this Practice.
func resolveEngagementClient(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) (string, error) {
	var clientID string
	err := tx.QueryRowContext(ctx,
		`SELECT c.id FROM engagements e JOIN clients c ON c.id = e.client_id
		 WHERE e.id = $1 AND e.practice_id = $2`,
		engagementID, practiceID,
	).Scan(&clientID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.ErrNoRows
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("portalinvite: resolve engagement client: %w", err)
	}
	return clientID, nil
}

// invite creates a pending client_portal_users row for clientID, or
// rotates the existing pending row's invite_token if one already exists.
// An already-accepted row (identity_uid set) is a 409 Conflict.
func invite(ctx context.Context, tx *sql.Tx, clientID string) (resp InviteResponse, status int, code, msg string) {
	var existingID string
	var identityUID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT id, identity_uid FROM client_portal_users WHERE client_id = $1`,
		clientID,
	).Scan(&existingID, &identityUID)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		newID := uuid.NewString()
		inviteToken := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO client_portal_users (id, client_id, invite_token) VALUES ($1, $2, $3)`,
			newID, clientID, inviteToken,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: newID, InviteToken: inviteToken}, http.StatusCreated, "", ""

	case err != nil:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError

	case identityUID.Valid:
		return InviteResponse{}, http.StatusConflict, "CONFLICT", "this client already has portal access"

	default:
		inviteToken := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_portal_users SET invite_token = $1 WHERE id = $2`,
			inviteToken, existingID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, codeInternalError, MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: existingID, InviteToken: inviteToken}, http.StatusOK, "", ""
	}
}
