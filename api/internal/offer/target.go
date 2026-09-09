package offer

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/staffauth"
)

// offerTarget is who an Offer is addressed to, in the shape
// engagement_offers stores it: exactly one of staffID and invitationID is
// set (00030's offer_target_named), and employmentType is snapshotted
// onto the row rather than read live off a Membership that may not exist
// yet.
//
// inviteToken and accessCode are the two plaintext credentials the email
// path mints. They exist only between here and the outbox row that mails
// them -- engagement_offers keeps a digest of the code, and
// practice_invitations a digest of the token.
type offerTarget struct {
	staffID          sql.NullString
	invitationID     sql.NullString
	employmentType   string
	inviteToken      string
	accessCode       string
	accessCodeDigest sql.NullString
}

// doulaRole is the practice_role an Offer's target must hold: ADR-0008's
// attachment is Doula-only, so offering work to an Admin who does not
// also do the work would mint an attachment nothing can use.
const doulaRole = "doula"

// contractorType is the employment_type whose fee the CHECK constraint
// requires, and the one an emailed Invitation joins someone as.
const contractorType = "contractor"

// blockedDetails is the field-keyed refusal both target shapes below
// share for a currently-suppressed address (ADR-0029):
// staffauth.MsgAddressBlocked rather than a package-local copy of it,
// because offer already imports staffauth for everything else this file
// does, and mailsuppress.Mount takes no dependency on offer that a direct
// mailsuppress import here could cycle with (unlike staffauth.InviteHandler's
// own SuppressionChecker indirection, needed only because
// mailsuppress.Mount takes a *staffauth.GatedRouter).
func blockedDetails() map[string]string {
	return map[string]string{"email": staffauth.MsgAddressBlocked}
}

// resolveTarget turns the request's staffId-or-email into an offerTarget,
// minting an Invitation for the email path. Exactly one of the two must
// be named -- an Offer with both would leave 00030's offer_target_named
// satisfied but the read paths ambiguous about which target decided.
func resolveTarget(ctx context.Context, tx *sql.Tx, practiceID, actorStaffID string, req CreateRequest) (offerTarget, int, apierr.Code, string, map[string]string) {
	address := staffauth.NormalizeAddress(req.Email)
	switch {
	case req.StaffID != "" && address != "":
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, "name either a staff member or an email address, not both", nil
	case req.StaffID != "":
		return resolveStaffTarget(ctx, tx, practiceID, req.StaffID)
	case address != "":
		return resolveEmailTarget(ctx, tx, practiceID, actorStaffID, address)
	default:
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, "an offer needs a staffId or an email address", nil
	}
}

// resolveStaffTarget reads the target's employment type and address off
// her own Membership and staff row rather than the request body -- a
// request that could assert "employee" for a contractor would skip the
// fee the CHECK constraint exists to require, and an address the request
// asserted rather than read off her own row could not be trusted for the
// suppression check below.
func resolveStaffTarget(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (offerTarget, int, apierr.Code, string, map[string]string) {
	if _, err := uuid.Parse(staffID); err != nil {
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, "invalid staff id", nil
	}

	var isDoula bool
	var employmentType, address string
	err := tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(pm.roles), pm.employment_type::text, s.email
		   FROM practice_memberships pm
		   JOIN staff s ON s.id = pm.staff_id
		  WHERE pm.practice_id = $2 AND pm.staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&isDoula, &employmentType, &address)
	if errors.Is(err, sql.ErrNoRows) {
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, "staff member not found at this practice", nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}
	if !isDoula {
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, "staff member does not hold the Doula role at this practice", nil
	}

	// A suppressed address (ADR-0029) is refused here, before any row is
	// written, the same claim #789 proves for the Client portal invite.
	// This address comes off her own stored staff row rather than the
	// request body, the same reason #789 answers a 409 rather than
	// #861's 400 field error for its own stored-record case.
	blocked, err := mailsuppress.Active(ctx, tx, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}
	if blocked {
		return offerTarget{}, http.StatusConflict, apierr.CodeFailedPrecondition, staffauth.MsgAddressBlocked, blockedDetails()
	}

	return offerTarget{
		staffID:        sql.NullString{String: staffID, Valid: true},
		employmentType: employmentType,
	}, http.StatusOK, "", "", nil
}

// resolveEmailTarget mints the Invitation the Offer rides on -- ADR-0008's
// "one link joins her to the Practice and puts the job in front of her at
// once" -- plus the six-digit code the pre-account read asks for.
//
// The Invitation always carries the Doula role and contractor employment
// type: CONTEXT.md's Offer entry says this link "joins her to the
// Practice as a contractor Doula", and it is the only shape that makes
// sense here -- an employee is inside the business, which is not
// something an emailed link makes anyone. Someone who should be an
// employee is invited through the Staff screen and offered work through
// her Membership afterwards.
func resolveEmailTarget(ctx context.Context, tx *sql.Tx, practiceID, actorStaffID, address string) (offerTarget, int, apierr.Code, string, map[string]string) {
	const employmentType = contractorType

	// Someone who is already at this Practice is offered work through her
	// Membership, not through a second front door: a fresh Invitation for
	// an address that already holds one would be refused at accept
	// anyway, and the Offer would be unacceptable forever.
	alreadyMember, err := staffauth.AddressHoldsMembership(ctx, tx, practiceID, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}
	if alreadyMember {
		return offerTarget{}, http.StatusConflict, apierr.CodeConflict, "that address already holds a membership at this practice -- offer the work to that staff member instead", nil
	}

	// A suppressed address (ADR-0029) is refused here, before
	// MintInvitation below writes a practice_invitations row -- the same
	// claim #861 proves for the Staff invitation. This address arrives
	// fresh in the request body rather than off a stored record, so it
	// takes #861's 400 field-error shape rather than resolveStaffTarget's
	// 409 above.
	blocked, err := mailsuppress.Active(ctx, tx, address)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}
	if blocked {
		return offerTarget{}, http.StatusBadRequest, apierr.CodeInvalidArgument, staffauth.MsgAddressBlocked, blockedDetails()
	}

	invitationID, token, _, rotated, err := staffauth.MintInvitation(ctx, tx, practiceID, actorStaffID, address, "{"+doulaRole+"}", employmentType)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}
	// Minting rotates the token, which silently breaks the link in every
	// email already sent against this Invitation -- including another
	// Engagement's still-open Offer to the same address. Those Offers are
	// re-issued rather than left holding a dead link: a fresh code each,
	// and a fresh email carrying the new token.
	if rotated {
		if status, msg := reissueOpenOffers(ctx, tx, invitationID, token); status != http.StatusOK {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return offerTarget{}, status, apierr.CodeForStatus(status), msg, nil
		}
	}

	code, err := newAccessCode()
	if err != nil {
		// coverage:ignore reason: crypto/rand failure, not exercised by unit tests
		return offerTarget{}, http.StatusInternalServerError, apierr.CodeInternal, apierr.MsgInternalError, nil
	}

	return offerTarget{
		invitationID:     sql.NullString{String: invitationID, Valid: true},
		employmentType:   employmentType,
		inviteToken:      token,
		accessCode:       code,
		accessCodeDigest: sql.NullString{String: staffauth.TokenDigest(code), Valid: true},
	}, http.StatusOK, "", "", nil
}

// reissueOpenOffers gives every Offer still open on invitationID a fresh
// access code and queues a fresh email carrying token -- the answer to
// the token this Invitation just rotated away from under them. One email
// per open Offer, which is what re-issuing two Offers means; the code
// has to be new because engagement_offers keeps only its digest, so the
// old one cannot be re-sent.
func reissueOpenOffers(ctx context.Context, tx *sql.Tx, invitationID, token string) (int, string) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM engagement_offers
		  WHERE invitation_id = $1 AND state = 'offered' AND expires_at > now()`,
		invitationID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return http.StatusInternalServerError, apierr.MsgInternalError
	}
	defer func() { _ = rows.Close() }()

	var offerIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: DB scan failure, not exercised by unit tests
			return http.StatusInternalServerError, apierr.MsgInternalError
		}
		offerIDs = append(offerIDs, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB row iteration failure, not exercised by unit tests
		return http.StatusInternalServerError, apierr.MsgInternalError
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: DB row close failure, not exercised by unit tests
		return http.StatusInternalServerError, apierr.MsgInternalError
	}

	for _, offerID := range offerIDs {
		code, err := newAccessCode()
		if err != nil {
			// coverage:ignore reason: crypto/rand failure, not exercised by unit tests
			return http.StatusInternalServerError, apierr.MsgInternalError
		}
		// access_code_attempts resets with the code: the guesses spent
		// against a code nobody can use any more are not held against the
		// one replacing it.
		if _, err := tx.ExecContext(ctx,
			`UPDATE engagement_offers
			    SET access_code_digest = $1, access_code_sent_at = NULL, access_code_attempts = 0
			  WHERE id = $2`,
			staffauth.TokenDigest(code), offerID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return http.StatusInternalServerError, apierr.MsgInternalError
		}
		if err := queue(ctx, tx, offerID, token, code); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return http.StatusInternalServerError, apierr.MsgInternalError
		}
	}
	return http.StatusOK, ""
}
