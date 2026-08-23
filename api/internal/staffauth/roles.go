package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// validRoles is the practice_role enum from 00002_practice_staff_tenancy.sql,
// mirrored here so role-assignment requests can be validated before they
// ever reach Postgres.
var validRoles = map[string]bool{"owner": true, "admin": true, "doula": true}

// staffHasRole reports whether staffID's membership at practiceID includes
// role. Handlers that are Owner-only (invite, role assignment) call this
// themselves -- staffauth.Middleware only ever checks that a membership
// exists, not what roles it holds, so a zero-role invitee still lands on
// the Practice per the Staff-invitation ticket.
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
// empty slice for a zero-role (e.g. not-yet-assigned invitee) membership.
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

// AssignRolesRequest replaces a membership's full role set (not a diff) --
// the caller sends the roles it should hold after the call.
type AssignRolesRequest struct {
	Roles []string `json:"roles"`
}

// AssignRolesResponse confirms the roles a membership holds after the
// update.
type AssignRolesResponse struct {
	StaffID string   `json:"staffId"`
	Roles   []string `json:"roles"`
}

// AssignRolesHandler lets a Practice Owner set the roles held by another
// Staff member's membership at the same Practice -- including a zero-role
// invitee's first roles, which is what makes their invite usable. Must be
// mounted behind staffauth.Middleware.
func AssignRolesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		targetStaffID := r.PathValue("staffId")

		var req AssignRolesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		for _, role := range req.Roles {
			if !validRoles[role] {
				http.Error(w, "unknown role: "+role, http.StatusBadRequest)
				return
			}
		}

		// req.Roles is validated against validRoles above, so this literal
		// can only ever contain known enum members -- safe to build as text
		// and let Postgres parse+cast it, rather than needing a driver-level
		// array encoder for a user-defined enum array type.
		literal := "{" + strings.Join(req.Roles, ",") + "}"
		result, err := tx.ExecContext(r.Context(),
			`UPDATE practice_memberships SET roles = $1::practice_role[] WHERE practice_id = $2 AND staff_id = $3`,
			literal, practiceID, targetStaffID,
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
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(AssignRolesResponse{StaffID: targetStaffID, Roles: req.Roles}); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}
