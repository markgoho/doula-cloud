package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// validRoles is the practice_role enum from 00002_practice_staff_tenancy.sql,
// mirrored here so role-assignment requests can be validated before they
// ever reach Postgres.
var validRoles = map[string]bool{"owner": true, "admin": true, "doula": true}

// staffHasRole reports whether staffID's membership at practiceID includes
// role. Handlers that are Owner-only (invite, membership editing) call
// this themselves -- staffauth.Middleware only ever checks that a
// membership exists, not what roles it holds.
func staffHasRole(ctx context.Context, tx *sql.Tx, practiceID, staffID, role string) (bool, error) {
	var has bool
	err := tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(roles) FROM practice_memberships WHERE practice_id = $2 AND staff_id = $3`,
		role, practiceID, staffID,
	).Scan(&has)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: check role: %w", err)
	}
	return has, nil
}

// Roles returns the roles staffID's membership holds at practiceID -- an
// empty slice for a membership holding none, which since #316 is only
// reachable by an Owner emptying one, not by joining.
// Exported so callers outside this package (the practice-landing handler
// in main.go) can gate Owner-only UI affordances, like the invite link,
// on the caller's actual roles instead of just their membership.
func Roles(ctx context.Context, tx *sql.Tx, practiceID, staffID string) ([]string, error) {
	var roles string
	err := tx.QueryRowContext(ctx,
		`SELECT array_to_string(roles, ',') FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&roles)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("staffauth: list roles: %w", err)
	}
	if roles == "" {
		return []string{}, nil
	}
	return strings.Split(roles, ","), nil
}

// RequireOwner resolves the caller's Staff/Practice ids and request-scoped
// tx from context (set by staffauth.Middleware) and confirms the caller
// holds the 'owner' role at that Practice, writing the appropriate error
// response itself if not. Shared by Owner-only handlers across packages
// (invite, role assignment, here, and billing.PostPurchaseHandler) the
// same way RequireTx is -- exported so billing doesn't need its own copy
// of the owner check.
func RequireOwner(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	staffID, _ := StaffID(r.Context())
	practiceID, _ = PracticeID(r.Context())

	isOwner, err := staffHasRole(r.Context(), tx, practiceID, staffID, "owner")
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !isOwner {
		http.Error(w, "only a Practice Owner can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}

// RequireOwnerOrAdmin is RequireOwner widened by one role, for the writes
// ADR-0008 puts in an Admin's hands as well as an Owner's -- making an
// Offer, withdrawing one, completing an Engagement. Owner-only stays the
// default for anything that changes who is at the Practice at all
// (inviting, editing a Membership); this is for running the work.
func RequireOwnerOrAdmin(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID string, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	staffID, _ := StaffID(r.Context())
	practiceID, _ = PracticeID(r.Context())

	roles, err := Roles(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, MsgInternalError, http.StatusInternalServerError)
		return nil, "", false
	}
	if !slices.Contains(roles, "owner") && !slices.Contains(roles, "admin") {
		http.Error(w, "only a Practice Owner or Admin can do that", http.StatusForbidden)
		return nil, "", false
	}
	return tx, practiceID, true
}
