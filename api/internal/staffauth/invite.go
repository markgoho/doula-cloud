package staffauth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/staffinvite"
	"doula-cloud/api/internal/tasknudge"
)

// InvitationLifetime is how long a Staff Invitation stays acceptable
// after it is sent -- 7 days, the default #226 chose and ADR-0008
// records. A re-invite restarts it rather than extending the old one.
const InvitationLifetime = 7 * 24 * time.Hour

// InviteRequest is the body of a Staff invitation: the address to invite,
// and the Membership the invitee gets when she accepts. Roles and
// employment type ride the Invitation rather than arriving in a later
// PATCH, which is what abolishes the zero-role membership RA-G8 found
// (#266). There is no name field: ADR-0008's practice_invitations has no
// name column, and the invitee types her own name at accept -- the
// Practice does not get to name her.
type InviteRequest struct {
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
}

// InviteResponse identifies the Invitation. It deliberately does not
// carry the token: the token is mailed to the invited address and
// nowhere else (#316), so an Owner cannot hand herself a link that
// bypasses proving control of that mailbox.
type InviteResponse struct {
	InvitationID string `json:"invitationId"`
	ExpiresAt    string `json:"expiresAt"`
}

// validEmploymentTypes is the employment_type enum from
// 00030_employment_attachment_offer.sql, mirrored here so a request can
// be rejected before it reaches Postgres -- the same reason validRoles
// exists.
var validEmploymentTypes = map[string]bool{"employee": true, "contractor": true}

// InviteHandler lets a Practice Owner invite someone to join the Practice
// with a named Membership. It inserts a practice_invitations row and no
// staff row: no person exists here until the invitee proves control of
// the invited address by accepting (#226), which is what keeps a failed
// acceptance from leaving a member behind who can never sign in (#291).
// The token is minted here, stored as a digest, and handed to
// staffinvite.Queue in this same transaction so the only place it is
// readable in the clear is the outbox row the mailer reads.
//
// enq is ADR-0013's Cloud Tasks nudge, registered rather than fired for
// the reason portalinvite.InviteHandler gives: Middleware's commit --
// which decides whether the queued outbox row survives at all -- runs
// after this handler has returned. Must be mounted behind
// staffauth.Middleware.
func InviteHandler(enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := StaffID(r.Context())

		var req InviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		address := NormalizeAddress(req.Email)
		if address == "" {
			http.Error(w, "email is required", http.StatusBadRequest)
			return
		}
		if len(req.Roles) == 0 {
			http.Error(w, "at least one role is required", http.StatusBadRequest)
			return
		}
		for _, role := range req.Roles {
			if !validRoles[role] {
				http.Error(w, "unknown role: "+role, http.StatusBadRequest)
				return
			}
		}
		if !validEmploymentTypes[req.EmploymentType] {
			http.Error(w, "employmentType must be employee or contractor", http.StatusBadRequest)
			return
		}

		resp, status, msg := invite(r.Context(), tx, practiceID, actorStaffID, address, req)
		if status != http.StatusCreated && status != http.StatusOK {
			http.Error(w, msg, status)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.StaffInvite))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// invite creates the Invitation, or -- if this address already has a
// pending one at this Practice -- rotates it: a fresh token, a fresh
// expiry, and the roles/employment type the Owner just typed, which may
// differ from the first attempt. Rotating rather than inserting is what
// practice_invitations_one_pending (00039) enforces, so two concurrent
// invites to one address cannot both win.
func invite(ctx context.Context, tx *sql.Tx, practiceID, actorStaffID, address string, req InviteRequest) (InviteResponse, int, string) {
	alreadyMember, err := addressHoldsMembership(ctx, tx, practiceID, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	if alreadyMember {
		return InviteResponse{}, http.StatusConflict, "that address already holds a membership at this practice"
	}

	token := uuid.NewString()
	digest := TokenDigest(token)
	expiresAt := time.Now().Add(InvitationLifetime)
	// req.Roles is validated against validRoles by the caller, so this
	// literal can only ever contain known enum members -- the same
	// reasoning UpdateMembershipHandler's array literal rests on.
	rolesLiteral := "{" + strings.Join(req.Roles, ",") + "}"

	// Read whether a pending Invitation is already here purely to pick
	// the status code -- 200 for a rotation, 201 for a first send. The
	// write below is an upsert either way, so a row appearing between
	// this read and it costs an inaccurate status code, never a second
	// Invitation. Letting the INSERT fail and retrying as an UPDATE is
	// not the alternative it looks like: a constraint violation aborts
	// the whole transaction in Postgres, taking the retry with it.
	var rotating bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM practice_invitations
			WHERE practice_id = $1 AND lower(address) = $2 AND status = 'pending'
		)`,
		practiceID, address,
	).Scan(&rotating); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	var invitationID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type, $5, $6, $7)
		 ON CONFLICT (practice_id, lower(address)) WHERE status = 'pending'
		 DO UPDATE SET roles = $3::practice_role[], employment_type = $4::employment_type,
		               token_digest = $5, invited_by = $6, created_at = now(), expires_at = $7
		 RETURNING id`,
		practiceID, address, rolesLiteral, req.EmploymentType, digest, actorStaffID, expiresAt,
	).Scan(&invitationID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	status := http.StatusCreated
	if rotating {
		status = http.StatusOK
	}

	if err := staffinvite.Queue(ctx, tx, invitationID, token); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return InviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return InviteResponse{InvitationID: invitationID, ExpiresAt: expiresAt.UTC().Format(time.RFC3339)}, status, ""
}

// addressHoldsMembership reports whether any staff row carrying address
// already holds a membership at practiceID. It matches on the address
// rather than on a resolved staff id on purpose: staff.email is not
// unique, so the same person signing in through a second identity
// provider would otherwise be invited afresh, accept, and land a second
// membership row on one address -- the two-rows-on-one-email shape LV-G8
// (#291) found on the roster.
func addressHoldsMembership(ctx context.Context, tx *sql.Tx, practiceID, address string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM practice_memberships pm
			JOIN staff s ON s.id = pm.staff_id
			WHERE pm.practice_id = $1 AND lower(s.email) = $2
		)`,
		practiceID, address,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: check address membership: %w", err)
	}
	return exists, nil
}

// RevokeInvitationHandler lets a Practice Owner withdraw a pending
// Invitation. The row is not deleted -- who was invited and when is part
// of the record, the same reasoning 00030 applies to the table as a whole
// -- so this flips status and records the actor and the moment. Must be
// mounted behind staffauth.Middleware.
func RevokeInvitationHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := StaffID(r.Context())

		invitationID := r.PathValue("invitationId")
		if !ParseUUID(w, "invitation", invitationID) {
			return
		}

		result, err := tx.ExecContext(r.Context(),
			`UPDATE practice_invitations
			    SET status = 'revoked', revoked_by = $1, revoked_at = now()
			  WHERE id = $2 AND practice_id = $3 AND status = 'pending'`,
			actorStaffID, invitationID, practiceID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			http.Error(w, "no pending invitation found at this practice", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// NormalizeAddress is the one spelling of an email address this package
// stores and compares. Identity Platform does not promise to hand back
// the address in the case the Owner typed it, and accept has to decide
// whether the caller's verified address *is* the invited one -- so both
// ends go through here rather than comparing raw input.
func NormalizeAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// TokenDigest is the stored form of an Invitation token: SHA-256, hex,
// following 00028_sessions.sql's precedent. A leaked read of
// practice_invitations hands nobody a usable credential.
func TokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
