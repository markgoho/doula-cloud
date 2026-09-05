package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// AcceptInviteRequest is the body of a Staff invitation acceptance: the
// token from the emailed link, and the name the invitee gives for
// herself. The name is asked for here rather than typed by the Owner at
// invite time because ADR-0008's practice_invitations carries no name
// column -- a person names herself. It is ignored when the caller
// already has a staff row, whose name is hers to change elsewhere.
type AcceptInviteRequest struct {
	InviteToken string `json:"inviteToken"`
	Name        string `json:"name"`
	// WorkState is the US state this person works from, as a USPS
	// two-letter abbreviation (#415). Like Name, it is asked for on the
	// form and ignored when the caller already has a staff row: a work
	// state is a fact about the person, so the Practice she joined first
	// already recorded it and this one inherits it. That inherited value
	// is not silent -- the roster prints it as self-reported, with the
	// date she last asserted it.
	WorkState string `json:"workState"`
}

// AcceptInviteResponse identifies the Practice the caller just joined and
// the staff row the acceptance resolved to -- new or pre-existing.
type AcceptInviteResponse struct {
	StaffID    string `json:"staffId"`
	PracticeID string `json:"practiceId"`
}

// AcceptInviteHandler turns a pending Invitation into a Membership. It
// runs before any session exists, so it reads a Bearer ID token through
// authn.BeginBootstrap, and the verified address on that token is the
// whole security of the flow: the Invitation is only acceptable by
// someone who can sign in as the address it was mailed to (#226). The
// caller's identity resolves to an existing staff row or creates one,
// and the Membership is written with the Invitation's own roles and
// employment type -- no zero-role membership, and no staff row created
// before someone proves she can sign in as it (#266, #291).
//
// accounts.SetEmailVerified is called on every successful acceptance
// (#613/#169): holding the invite token already proves control of the
// invited address -- inv.address must match the caller's verified email,
// checked below -- so re-proving it with a mailed verification link
// would be a second round-trip proving something already proven. No
// verification mail is ever queued on this path.
func AcceptInviteHandler(verifier authn.Verifier, accounts authn.AccountManager, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, verified, ok := authn.BeginBootstrap(w, r, verifier, db)
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
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.InviteToken = strings.TrimSpace(req.InviteToken)
		req.Name = strings.TrimSpace(req.Name)
		req.WorkState = strings.TrimSpace(req.WorkState)
		if req.InviteToken == "" {
			apierr.WriteError(w, "inviteToken is required", http.StatusBadRequest)
			return
		}

		resp, status, msg := acceptInvite(r.Context(), tx, verified, req)
		if status != http.StatusOK {
			// 410 is the one failure that wrote something: acceptInvite
			// marks an Invitation it finds past its expiry, and that
			// discovery is a fact worth keeping rather than rolling back
			// with the acceptance that didn't happen.
			if status == http.StatusGone {
				if err := tx.Commit(); err != nil {
					// coverage:ignore reason: DB commit failure, not exercised by unit tests
					apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
					return
				}
				committed = true
			}
			apierr.WriteError(w, msg, status)
			return
		}

		// Confirmed above: holding this Invitation's token already proves
		// mailbox control. Done before MintSession, on the same "external
		// side effect that fails rolls everything back" reasoning as the
		// session mint below -- a joined Membership sitting behind an
		// account Identity Platform still calls unverified would leave
		// #606's MFA gate with nothing to enroll against.
		if err := accounts.SetEmailVerified(r.Context(), verified.UID); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Mint the session before committing, so a failure rolls the new
		// Membership back rather than leaving it behind a response that
		// reports failure -- signup.go's reasoning (#145).
		cookie, err := authn.MintSession(r.Context(), tx, verified.UID, verified.SecondFactor, time.Now())
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		http.SetCookie(w, cookie)
		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// invitation is one pending practice_invitations row, as the accept path
// reads it.
type invitation struct {
	id             string
	practiceID     string
	address        string
	roles          string
	employmentType string
	expiresAt      time.Time
}

// acceptInvite does the work in the order the RLS policies demand: look
// the Invitation up through practice_invitations_accept_lookup (00039)
// while no Practice context is set, resolve or create the staff row
// through staff_self_visibility/staff_self_insert (which only apply
// while app.current_practice_id is unset -- signup.go:116-121 documents
// the same ordering constraint), and only then set the Practice context
// the Membership insert and the Invitation update need.
func acceptInvite(ctx context.Context, tx *sql.Tx, verified authn.VerifiedToken, req AcceptInviteRequest) (AcceptInviteResponse, int, string) {
	address := NormalizeAddress(verified.Email)
	if address == "" {
		return AcceptInviteResponse{}, http.StatusForbidden, "your account has no verified email address, so it cannot accept an invitation"
	}

	digest := TokenDigest(req.InviteToken)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.invite_token_digest', $1, true)`, digest); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, verified.UID); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	var inv invitation
	err := tx.QueryRowContext(ctx,
		`SELECT id, practice_id, address, array_to_string(roles, ','), employment_type::text, expires_at
		 FROM practice_invitations
		 WHERE token_digest = $1 AND status = 'pending'`,
		digest,
	).Scan(&inv.id, &inv.practiceID, &inv.address, &inv.roles, &inv.employmentType, &inv.expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AcceptInviteResponse{}, http.StatusNotFound, "invitation not found, already accepted, or revoked"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// An address mismatch is deliberately the same 403 whether the caller
	// signed in as the wrong person or is fishing with someone else's
	// link -- either way she is not the invitee.
	if NormalizeAddress(inv.address) != address {
		return AcceptInviteResponse{}, http.StatusForbidden, "this invitation was sent to a different email address"
	}

	if !inv.expiresAt.After(time.Now()) {
		// Flip the column on the way past rather than waiting for a
		// sweep: the person holding the link is the one who found out it
		// is stale, and the Owner's Staff screen should say so too. The
		// Practice context is set first because
		// practice_invitations_accept_lookup (00039) is SELECT-only on
		// purpose -- a write to this row goes through the practice-tier
		// policy, and nothing else in this branch needs the pre-Practice
		// window.
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, inv.practiceID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE practice_invitations SET status = 'expired' WHERE id = $1`, inv.id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
		}
		return AcceptInviteResponse{}, http.StatusGone, "this invitation has expired -- ask for a new one"
	}

	staffID, newWorkState, status, msg := resolveStaff(ctx, tx, verified, address, req.Name, req.WorkState)
	if status != http.StatusOK {
		return AcceptInviteResponse{}, status, msg
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, inv.practiceID); err != nil {
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// A membership already at this Practice is a 409, not the 500 an
	// unhandled unique violation used to produce (#316). It is checked on
	// the address rather than on staffID alone because staff.email is not
	// unique: the same person arriving through a second identity provider
	// gets a second staff row, and only the address catches that.
	alreadyMember, err := AddressHoldsMembership(ctx, tx, inv.practiceID, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}
	if alreadyMember {
		return AcceptInviteResponse{}, http.StatusConflict, "you already hold a membership at this practice"
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type)`,
		inv.practiceID, staffID, "{"+inv.roles+"}", inv.employmentType,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if err := RecordMembershipEvent(ctx, tx, MembershipEvent{
		PracticeID:     inv.practiceID,
		StaffID:        staffID,
		Type:           "joined",
		Roles:          "{" + inv.roles + "}",
		EmploymentType: inv.employmentType,
		ActorStaffID:   staffID,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// #615's AC: a fresh Membership can make the new Staff member a sole
	// Owner (an invited co-founder), or can end an existing sole Owner's
	// status (a second Owner just arrived) -- reconcileOwnersAtPractice
	// covers both without this handler having to decide which applies.
	if err := reconcileOwnersAtPractice(ctx, tx, inv.practiceID, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// Only a person this acceptance created gets a work-state event: for
	// anyone else the fact predates this Practice, and re-recording it
	// here would read as her having asserted it again when she did not
	// (#415). Written after the Membership because
	// staff_work_state_events_practice_visibility (00043) admits a row
	// only for someone holding one at the current Practice.
	if newWorkState != "" {
		if err := RecordFirstWorkStateAssertion(ctx, tx, staffID, newWorkState, staffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE practice_invitations
		    SET status = 'accepted', accepted_staff_id = $1, accepted_at = now()
		  WHERE id = $2`,
		staffID, inv.id,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// An Offer mailed to this address (#317) named the Invitation, not a
	// staff row, because no staff row existed to name. Now one does, so
	// the Offer gets the id it was always going to have -- ADR-0008's
	// "staff_id stays NULL until the Invitation is accepted and the
	// accept handler back-fills it". Every Offer on the Invitation is
	// back-filled, not just an open one: a withdrawn or expired Offer is
	// part of the history she reads, and it should name her too.
	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_offers SET staff_id = $1 WHERE invitation_id = $2 AND staff_id IS NULL`,
		staffID, inv.id,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return AcceptInviteResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return AcceptInviteResponse{StaffID: staffID, PracticeID: inv.practiceID}, http.StatusOK, ""
}

// resolveStaff returns the staff row for the caller's verified identity,
// creating one if this is her first Practice. The second return is the
// normalized work state of a row it created, and empty for a row that
// already existed -- which is what tells the caller whether there is a
// first work-state assertion to record, and spares it re-normalizing
// what this function already validated. Must run before
// app.current_practice_id is set: both policies it depends on
// (staff_self_visibility, staff_self_insert) are scoped to that window.
func resolveStaff(ctx context.Context, tx *sql.Tx, verified authn.VerifiedToken, address, name, workState string) (string, string, int, string) {
	var staffID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM staff WHERE identity_uid = $1`, verified.UID).Scan(&staffID)
	if err == nil {
		return staffID, "", http.StatusOK, ""
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", http.StatusInternalServerError, MsgInternalError
	}

	if name == "" {
		return "", "", http.StatusBadRequest, "name is required to create your account"
	}
	// Validated here rather than at the top of the handler because it is
	// required only on the branch that creates a person: someone already
	// Staff elsewhere keeps the work state she already asserted, and
	// rejecting her invitation over a field her form did not need would
	// be a wall in the middle of a flow.
	normalized, ok := NormalizeWorkState(workState)
	if !ok {
		return "", "", http.StatusBadRequest, MsgWorkStateRequired
	}
	// staff.email is the verified address, not anything the caller typed
	// -- it is what a later invitation to this Practice is matched
	// against, so it may not be self-asserted.
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, $4) RETURNING id`,
		verified.UID, name, address, normalized,
	).Scan(&staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", http.StatusInternalServerError, MsgInternalError
	}
	return staffID, normalized, http.StatusOK, ""
}
