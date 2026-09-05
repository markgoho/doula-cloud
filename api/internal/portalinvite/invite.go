package portalinvite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// inviteTokenLifetime is how long a portal invitation stays acceptable --
// #616's AC, longer than a sign-in link (ADR-0026's 15 minutes) because
// she may not read mail for days. Re-sendable by the doula: invite
// rotates both the token and this expiry together.
const inviteTokenLifetime = 7 * 24 * time.Hour

// InviteResponse identifies the pending client_portal_users row created (or
// re-invited) and hands back the one-time token the Client needs to accept.
// invite() also queues a Practice-voice Notification email carrying the
// same link (#219, ADR-0010's outbox); InviteToken stays in the response
// too, so a Staff member can still copy/paste the link by hand as a
// fallback if the email never arrives.
type InviteResponse struct {
	ClientPortalUserID string `json:"clientPortalUserId"`
	InviteToken        string `json:"inviteToken"`
}

// InviteHandler lets a Staff member scoped to a Practice invite the Client
// on the named Engagement to the Client portal. Per #90's scope decision,
// gating is practice-tier (any Staff member, not just an Owner, and not
// scoped to the specific Engagement) -- same tier as clients_insert in
// 00005_client_engagement.sql. Must be mounted behind staffauth.Middleware.
// enq is ADR-0013's Cloud Tasks nudge: on a successful invite, this
// registers a nudge for the portal-invite outbox rather than firing it
// directly, since staffauth.Middleware's tx.Commit() -- which decides
// whether the queued outbox row actually survives -- runs after this
// handler (and idempotency.Wrap's response cache) has already returned.
func InviteHandler(enq tasknudge.Enqueuer) http.Handler {
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
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp, status, code, msg := invite(r.Context(), tx, clientID)
		if status != http.StatusOK && status != http.StatusCreated {
			apierr.Write(w, status, code, msg, nil)
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionPortalInviteSent),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.PortalInvite))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
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
func invite(ctx context.Context, tx *sql.Tx, clientID string) (resp InviteResponse, status int, code apierr.Code, msg string) {
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
			`INSERT INTO client_portal_users (id, client_id, invite_token, invite_token_expires_at) VALUES ($1, $2, $3, $4)`,
			newID, clientID, inviteToken, time.Now().Add(inviteTokenLifetime),
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		if err := queueOutboxSend(ctx, tx, newID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: newID, InviteToken: inviteToken}, http.StatusCreated, "", ""

	case err != nil:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError

	case identityUID.Valid:
		return InviteResponse{}, http.StatusConflict, apierr.CodeConflict, "this client already has portal access"

	default:
		inviteToken := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_portal_users SET invite_token = $1, invite_token_expires_at = $2 WHERE id = $3`,
			inviteToken, time.Now().Add(inviteTokenLifetime), existingID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		if err := queueOutboxSend(ctx, tx, existingID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: existingID, InviteToken: inviteToken}, http.StatusOK, "", ""
	}
}
