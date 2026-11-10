// Package portalinvite issues and accepts a Client-portal invitation: the
// provisioning path 00006_client_portal_users.sql's RLS and
// clientauth.Middleware assumed but never got, per #90. Mirrors
// staffauth's invite/accept shape, applied to client_portal_users instead
// of staff.
package portalinvite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// msgAddressBlocked is what a Staff member sees when the Client's
// address is currently suppressed (ADR-0029). Named field-keyed so a
// caller with somewhere to point it (docs/api-design.md section 7)
// can, even though today's Engagement hub renders it as plain text
// where "An email has been sent to them" would have gone (#789).
//
// Points at the screen rather than promising a Clear: a Doula-role
// Staff member can send this invite (practice-tier, per #90) but
// cannot reach Clear (Owner/Admin-only, #744), and a complaint-caused
// suppression is never clearable at all (mailsuppress.ErrNotClearable)
// -- "clear it" would be wrong for either reader.
const msgAddressBlocked = "This email address is blocked. Blocked email addresses shows why and what can be done."

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
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, engagementID := c.Tx, c.PracticeID, c.EngagementID

		clientID, email, err := resolveEngagementClient(r.Context(), tx, engagementID, practiceID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// An erased record has no address and no portal link left
		// (ADR-0027), so an invitation would be a pending row nobody can
		// receive.
		if !clientstanding.ActInviteToPortal.Require(w, r, tx, clientID) {
			return
		}

		// A suppressed address (ADR-0029) is refused here rather than
		// queued: the outbox guard would only dead-letter it later,
		// after client_portal_users already carries a pending row and
		// the screen already says "an email has been sent." The Client
		// is this Practice's own, so there is nothing here #744's
		// AttachedToPractice check would add -- the address is already
		// this Practice's business.
		if email.Valid {
			suppressed, err := mailsuppress.Active(r.Context(), tx, email.String)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if suppressed {
				apierr.WriteFieldError(w, http.StatusConflict, apierr.CodeFailedPrecondition, "email", msgAddressBlocked)
				return
			}
		}

		resp, status, code, msg := invite(r.Context(), tx, clientID)
		if status != http.StatusOK && status != http.StatusCreated {
			apierr.Write(w, status, code, msg, nil)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionPortalInviteSent),
			Actor:       activity.StaffActor(c.StaffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.PortalInvite))

		apierr.WriteJSON(w, status, resp)
	}, staffauth.EngagementID())
}

// resolveEngagementClient confirms engagementID belongs to practiceID and
// returns its client_id and email (nullable, per ADR-0017), mirroring
// engagement.DetailHandler's join. Returns sql.ErrNoRows if the
// Engagement doesn't exist at this Practice.
func resolveEngagementClient(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) (string, sql.NullString, error) {
	var clientID string
	var email sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT c.id, c.email FROM engagements e JOIN clients c ON c.id = e.client_id
		 WHERE e.id = $1 AND e.practice_id = $2`,
		engagementID, practiceID,
	).Scan(&clientID, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.NullString{}, sql.ErrNoRows
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", sql.NullString{}, fmt.Errorf("portalinvite: resolve engagement client: %w", err)
	}
	return clientID, email, nil
}

// invite makes TransitionInvite (a new row) for clientID, or
// TransitionReinvite on her live invitation if she holds one. A Client
// whose access is accepted is a 409 Conflict.
//
// Since #813 a Client merged from two records can be reached by more
// than one Portal Account, so this reads one row deliberately rather
// than assuming there is only one, through herRowOrder -- the same row
// every Staff-facing read calls hers. It prefers an accepted row: a
// woman who can already sign in is refused a second invitation
// whichever of her logins the query happened to see first, which is the
// answer that cannot be wrong. A revoked or unlinked row is history, so
// a new invitation is a new row and that one keeps its record.
func invite(ctx context.Context, tx *sql.Tx, clientID string) (resp InviteResponse, status int, code apierr.Code, msg string) {
	var existingID string
	var state State
	err := tx.QueryRowContext(ctx,
		`SELECT id, state FROM client_portal_users
		  WHERE client_id = $1
		  `+herRowOrder+`
		  LIMIT 1`,
		clientID,
	).Scan(&existingID, &state)
	if errors.Is(err, sql.ErrNoRows) {
		err, state = nil, ""
	}
	expiresAt := clock.Now(ctx).Add(inviteTokenLifetime)

	switch {
	case err != nil:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError

	case state == StateAccepted:
		return InviteResponse{}, http.StatusConflict, apierr.CodeConflict, "this client already has portal access"

	case !TransitionReinvite.Allows(state):
		newID := uuid.NewString()
		inviteToken := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO client_portal_users (id, client_id, state, invite_token, invite_token_expires_at) VALUES ($1, $2, $3, $4, $5)`,
			newID, clientID, string(TransitionInvite.To), inviteToken, expiresAt,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError
		}
		if err := queueOutboxSend(ctx, tx, newID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: newID, InviteToken: inviteToken}, http.StatusCreated, "", ""

	default:
		inviteToken := uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`UPDATE client_portal_users
			    SET state = $1, invite_token = $2, invite_token_expires_at = $3, access_refused_at = NULL
			  WHERE id = $4 AND state = ANY($5::portal_access_state[])`,
			string(TransitionReinvite.To), inviteToken, expiresAt, existingID, TransitionReinvite.FromStates(),
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError
		}
		if err := queueOutboxSend(ctx, tx, existingID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return InviteResponse{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError
		}
		return InviteResponse{ClientPortalUserID: existingID, InviteToken: inviteToken}, http.StatusOK, "", ""
	}
}
