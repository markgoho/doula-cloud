package visit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// doulaRole is the practice_role enum member (00002_practice_staff_tenancy.sql)
// this package cares about -- only a Staff member with the Doula role may
// create or reassign a Visit.
const doulaRole = "doula"

// requireDoula resolves the caller's Staff/Practice ids and request-scoped
// tx from context (set by staffauth.Middleware) and confirms the caller
// holds the Doula role at that Practice, writing the appropriate error
// response itself if not. The caller is guaranteed a practice_memberships
// row by staffauth.Middleware, so staffauth.Roles's no-rows-is-an-error
// shape is safe to use here (unlike for an arbitrary reassignment target,
// see doulaMembership below).
func requireDoula(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, practiceID, ok = staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", false
	}
	staffID, _ := staffauth.StaffID(r.Context())

	roles, err := staffauth.Roles(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !slices.Contains(roles, doulaRole) {
		apierr.WriteError(w, "only a Staff member with the Doula role can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}

// doulaMembership reports whether staffID holds a practice_memberships row
// at practiceID, and if so whether that membership includes the Doula
// role and which employment type it carries. Unlike staffauth.Roles
// (built for the caller, whom staffauth.Middleware already guarantees a
// membership for), this treats "no membership" as a normal, expected
// outcome rather than an error -- staffID here is an arbitrary
// reassignment target the caller supplied, which may not be a Staff
// member at this Practice at all.
func doulaMembership(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (hasMembership, isDoula bool, employmentType string, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(roles), employment_type::text
		   FROM practice_memberships WHERE practice_id = $2 AND staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&isDoula, &employmentType)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "", nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, false, "", fmt.Errorf("visit: check doula membership: %w", err)
	}
	return true, isDoula, employmentType, nil
}

// employeeType is the employment_type a Practice may put on a birth
// directly. CONTEXT.md's Attachment entry draws the line: "An Admin may
// attach an employee directly -- naming her on a Visit is granted, not
// accrued, because she has done nothing... A contractor can only be
// attached by her own acceptance of an Offer: nobody can put an outsider
// on a Client's birth without her agreement" -- so a direct grant is for
// an employee and nobody else.
const employeeType = "employee"

// hasGrantedAttachment reports whether staffID holds an open, granted
// attachment to engagementID -- the record that she agreed to be on this
// birth, which is the only way a contractor gets onto one.
func hasGrantedAttachment(ctx context.Context, tx *sql.Tx, engagementID, staffID string) (bool, error) {
	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM engagement_attachments
			WHERE engagement_id = $1 AND staff_id = $2
			  AND origin = 'granted' AND ended_at IS NULL
		)`,
		engagementID, staffID,
	).Scan(&attached)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("visit: check granted attachment: %w", err)
	}
	return attached, nil
}

// callerEmploymentType reads the caller's own employment type at
// practiceID. Unlike doulaMembership this is for the caller, whom
// staffauth.Middleware already guarantees a membership for, so no rows is
// an error rather than an expected outcome.
func callerEmploymentType(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (string, error) {
	var employmentType string
	err := tx.QueryRowContext(ctx,
		`SELECT employment_type::text FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&employmentType)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("visit: read caller employment type: %w", err)
	}
	return employmentType, nil
}
