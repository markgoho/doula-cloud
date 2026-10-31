package staffauth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clock"
)

// LookupInvitationRequest carries the token from the emailed link, in the
// body rather than the path so it never lands in an access log.
type LookupInvitationRequest struct {
	InviteToken string `json:"inviteToken"`
}

// LookupInvitationResponse is what the accept screen shows before she
// types anything (#2238): the Practice, the address the Invitation was
// sent to, what she joins as, and whether that address already signs in
// to DoulaCloud, which decides whether the screen asks her to create a
// password or to type the one she has.
type LookupInvitationResponse struct {
	PracticeName   string   `json:"practiceName"`
	Address        string   `json:"address"`
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
	HasAccount     bool     `json:"hasAccount"`
}

// LookupInvitationHandler reads a pending Invitation by its token. The
// token is the whole credential, as it is for accepting: whoever holds it
// was mailed it, so the address and whether it has an account are hers to
// read. It writes nothing -- an expired Invitation answers 410 here and is
// marked expired only by the accept path, which runs in a Practice
// context this read never sets.
func LookupInvitationHandler(accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req LookupInvitationRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.InviteToken = strings.TrimSpace(req.InviteToken)
		if req.InviteToken == "" {
			apierr.WriteError(w, "inviteToken is required", http.StatusBadRequest)
			return
		}

		resp, status, msg := lookupInvitation(r.Context(), db, req.InviteToken)
		if status != http.StatusOK {
			apierr.WriteError(w, msg, status)
			return
		}

		_, err := accounts.GetAccountByEmail(r.Context(), resp.Address)
		switch {
		case err == nil:
			resp.HasAccount = true
		case errors.Is(err, authn.ErrAccountNotFound):
			resp.HasAccount = false
		default:
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

func lookupInvitation(ctx context.Context, db *sql.DB, token string) (LookupInvitationResponse, int, string) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		return LookupInvitationResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	defer func() { _ = tx.Rollback() }()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.invite_token_digest', $1, true)`, TokenDigest(token)); err != nil {
		return LookupInvitationResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}

	var resp LookupInvitationResponse
	var roles string
	var expired bool
	err = tx.QueryRowContext(ctx,
		`SELECT p.name, i.address, array_to_string(i.roles, ','), i.employment_type::text, i.expires_at <= $2
		 FROM practice_invitations i
		 JOIN practices p ON p.id = i.practice_id
		 WHERE i.token_digest = $1 AND i.status = 'pending'`,
		TokenDigest(token), clock.Now(ctx),
	).Scan(&resp.PracticeName, &resp.Address, &roles, &resp.EmploymentType, &expired)
	if errors.Is(err, sql.ErrNoRows) {
		return LookupInvitationResponse{}, http.StatusNotFound, "invitation not found, already accepted, or revoked"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return LookupInvitationResponse{}, http.StatusInternalServerError, apierr.MsgInternalError
	}
	if expired {
		return LookupInvitationResponse{}, http.StatusGone, "this invitation has expired -- ask for a new one"
	}
	resp.Address = NormalizeAddress(resp.Address)
	resp.Roles = splitRoles(roles)
	return resp, http.StatusOK, ""
}
