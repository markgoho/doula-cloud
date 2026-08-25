package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// MembershipEvent is one recorded change to a Membership --
// practice_membership_events (00039). Roles and EmploymentType are the
// state after the change; Previous* are the state before, empty on a
// 'joined' event, which has no before.
type MembershipEvent struct {
	PracticeID             string
	StaffID                string
	Type                   string // 'joined' | 'roles_changed' | 'employment_type_changed'
	PreviousRoles          string // Postgres array literal, e.g. "{owner,doula}"
	Roles                  string
	PreviousEmploymentType string
	EmploymentType         string
	ActorStaffID           string
}

// RecordMembershipEvent writes one row of a Membership's history. Every
// path that creates or changes a Membership calls it in its own
// transaction, so the record and the change either both land or neither
// does -- CLAUDE.md's audit-trail expectation, answered where the change
// happens rather than by a listener that can miss one.
func RecordMembershipEvent(ctx context.Context, tx *sql.Tx, e MembershipEvent) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO practice_membership_events
		     (practice_id, staff_id, event_type, previous_roles, roles,
		      previous_employment_type, employment_type, actor_staff_id)
		 VALUES ($1, $2, $3::membership_event_type,
		         NULLIF($4, '')::practice_role[], NULLIF($5, '')::practice_role[],
		         NULLIF($6, '')::employment_type, NULLIF($7, '')::employment_type, $8)`,
		e.PracticeID, e.StaffID, e.Type, e.PreviousRoles, e.Roles,
		e.PreviousEmploymentType, e.EmploymentType, e.ActorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record membership event: %w", err)
	}
	return nil
}

// UpdateMembershipRequest replaces a Membership's roles and employment
// type together (not a diff) -- the caller sends the state it should hold
// after the call. They arrive on one request because they are edited on
// one form (RA-G2, #261): ADR-0008 makes them the two halves of what a
// person is at a Practice, and splitting them into two round trips only
// invents an intermediate state nobody asked for.
type UpdateMembershipRequest struct {
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
}

// UpdateMembershipResponse confirms the Membership after the update.
type UpdateMembershipResponse struct {
	StaffID        string   `json:"staffId"`
	Roles          []string `json:"roles"`
	EmploymentType string   `json:"employmentType"`
}

// UpdateMembershipHandler lets a Practice Owner edit another Staff
// member's Membership at the same Practice. The change takes effect at
// once, with no grandfathering: ADR-0008 makes employment type the gate
// on ambient reach, and a gate that keeps honouring the old answer is
// not a gate. Both halves of the change are recorded with actor and
// timestamp. Must be mounted behind staffauth.Middleware.
func UpdateMembershipHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		actorStaffID, _ := StaffID(r.Context())
		targetStaffID := r.PathValue("staffId")
		if !ParseUUID(w, "staff", targetStaffID) {
			return
		}

		var req UpdateMembershipRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
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

		// req.Roles is validated against validRoles above, so this literal
		// can only ever contain known enum members -- safe to build as
		// text and let Postgres parse and cast it, rather than needing a
		// driver-level array encoder for a user-defined enum array type.
		rolesLiteral := "{" + strings.Join(req.Roles, ",") + "}"

		var previousRoles, previousEmploymentType string
		err := tx.QueryRowContext(r.Context(),
			`SELECT array_to_string(roles, ','), employment_type::text
			   FROM practice_memberships
			  WHERE practice_id = $1 AND staff_id = $2
			  FOR UPDATE`,
			practiceID, targetStaffID,
		).Scan(&previousRoles, &previousEmploymentType)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// A Practice must keep at least one Owner. Nothing else in the
		// API can grant the role back, so an Owner demoting the last one
		// -- herself, most likely -- would leave a Practice nobody can
		// invite to, edit a Membership at, or buy Credits for.
		lastOwner, err := removesLastOwner(r, tx, practiceID, targetStaffID, previousRoles, req.Roles)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if lastOwner {
			http.Error(w, "a practice must keep at least one Owner", http.StatusConflict)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE practice_memberships
			    SET roles = $1::practice_role[], employment_type = $2::employment_type
			  WHERE practice_id = $3 AND staff_id = $4`,
			rolesLiteral, req.EmploymentType, practiceID, targetStaffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// One event per axis that actually moved, so the history reads as
		// what changed rather than as a list of times someone opened the
		// form. A no-op edit records nothing.
		if "{"+previousRoles+"}" != rolesLiteral {
			if err := RecordMembershipEvent(r.Context(), tx, MembershipEvent{
				PracticeID: practiceID, StaffID: targetStaffID, Type: "roles_changed",
				PreviousRoles: "{" + previousRoles + "}", Roles: rolesLiteral,
				ActorStaffID: actorStaffID,
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		if previousEmploymentType != req.EmploymentType {
			if err := RecordMembershipEvent(r.Context(), tx, MembershipEvent{
				PracticeID: practiceID, StaffID: targetStaffID, Type: "employment_type_changed",
				PreviousEmploymentType: previousEmploymentType, EmploymentType: req.EmploymentType,
				ActorStaffID: actorStaffID,
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		updated := UpdateMembershipResponse{StaffID: targetStaffID, Roles: req.Roles, EmploymentType: req.EmploymentType}
		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(updated); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// removesLastOwner reports whether applying nextRoles to targetStaffID's
// Membership would leave practiceID with no Owner at all. It is only ever
// true when the target currently holds 'owner', is losing it, and nobody
// else at the Practice holds it.
func removesLastOwner(r *http.Request, tx *sql.Tx, practiceID, targetStaffID, previousRoles string, nextRoles []string) (bool, error) {
	if !slices.Contains(strings.Split(previousRoles, ","), "owner") || slices.Contains(nextRoles, "owner") {
		return false, nil
	}
	var otherOwners bool
	err := tx.QueryRowContext(r.Context(),
		`SELECT EXISTS(
			SELECT 1 FROM practice_memberships
			WHERE practice_id = $1 AND staff_id <> $2 AND 'owner' = ANY(roles)
		)`,
		practiceID, targetStaffID,
	).Scan(&otherOwners)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("staffauth: count remaining owners: %w", err)
	}
	return !otherOwners, nil
}
