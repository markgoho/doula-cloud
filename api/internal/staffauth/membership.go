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

	"doula-cloud/api/internal/activity"
)

// MembershipEvent is one recorded change to a Membership, subject_kind
// 'membership' in the activity log (ADR-0022). Roles and EmploymentType
// are the state after the change; Previous* are the state before, empty
// on a 'joined' event, which has no before.
type MembershipEvent struct {
	PracticeID             string
	StaffID                string
	Type                   string // 'joined' | 'roles_changed' | 'employment_type_changed' | 'removed'
	PreviousRoles          string // Postgres array literal, e.g. "{owner,doula}"
	Roles                  string
	PreviousEmploymentType string
	EmploymentType         string
	ActorStaffID           string
}

// fromTo is one changed fact's before/after, the same shape client's
// diffRecords builds -- a membership event has exactly two facts that
// can change (roles, employment type), never more, so it is written by
// hand here rather than sharing that package's diffing machinery.
type fromTo struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// rolesLiteralToCSV strips a Postgres array literal's braces, e.g.
// "{owner,doula}" -> "owner,doula", leaving the diff's JSON free of
// Postgres-specific syntax. A MembershipEvent whose roles did not
// change carries "" (no braces at all, see RecordMembershipEvent's
// callers), which passes through unchanged.
func rolesLiteralToCSV(literal string) string {
	if literal == "" {
		return ""
	}
	return strings.Trim(literal, "{}")
}

// RecordMembershipEvent writes one row of a Membership's history. Every
// path that creates or changes a Membership calls it in its own
// transaction, so the record and the change either both land or neither
// does -- CLAUDE.md's audit-trail expectation, answered where the change
// happens rather than by a listener that can miss one.
func RecordMembershipEvent(ctx context.Context, tx *sql.Tx, e MembershipEvent) error {
	diff, err := json.Marshal(struct {
		Roles          fromTo `json:"roles"`
		EmploymentType fromTo `json:"employmentType"`
	}{
		Roles:          fromTo{From: rolesLiteralToCSV(e.PreviousRoles), To: rolesLiteralToCSV(e.Roles)},
		EmploymentType: fromTo{From: e.PreviousEmploymentType, To: e.EmploymentType},
	})
	if err != nil {
		// coverage:ignore reason: a struct of plain strings always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("staffauth: marshal membership event diff: %w", err)
	}

	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  e.PracticeID,
		SubjectKind: "membership",
		SubjectID:   e.StaffID,
		Action:      e.Type,
		Diff:        diff,
		Actor:       activity.StaffActor(e.ActorStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: record membership event: %w", err)
	}
	return nil
}

// membership is a Membership's two halves in the form the database wants
// them: roles as a practice_role[] literal, employment type as the enum's
// own spelling. parseMembership is the only way to build one, so a
// caller cannot reach the SQL below with an unvalidated role name.
type membership struct {
	roles          []string
	rolesLiteral   string
	employmentType string
}

// parseMembership validates the roles and employment type an invite or a
// membership edit carries -- the same rules on both, because they set the
// same two columns. It writes the 400 itself and returns ok=false, the
// way RequireOwner writes its own 403.
func parseMembership(w http.ResponseWriter, roles []string, employmentType string) (membership, bool) {
	if len(roles) == 0 {
		http.Error(w, "at least one role is required", http.StatusBadRequest)
		return membership{}, false
	}
	for _, role := range roles {
		if !validRoles[role] {
			http.Error(w, "unknown role: "+role, http.StatusBadRequest)
			return membership{}, false
		}
	}
	if !validEmploymentTypes[employmentType] {
		http.Error(w, "employmentType must be employee or contractor", http.StatusBadRequest)
		return membership{}, false
	}
	// Every role here is a known enum member, so this literal is safe to
	// build as text and let Postgres parse and cast -- no driver-level
	// array encoder is needed for a user-defined enum array type.
	return membership{
		roles:          roles,
		rolesLiteral:   "{" + strings.Join(roles, ",") + "}",
		employmentType: employmentType,
	}, true
}

// sameRoles reports whether two role sets hold the same members, ignoring
// order. The order roles arrive in is the caller's, not a fact about the
// Membership, so a reordered but otherwise identical edit must not record
// a change that did not happen.
func sameRoles(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
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
		next, ok := parseMembership(w, req.Roles, req.EmploymentType)
		if !ok {
			return
		}

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
		lastOwner, err := removesLastOwner(r.Context(), tx, practiceID, targetStaffID, previousRoles, next.roles)
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
			next.rolesLiteral, next.employmentType, practiceID, targetStaffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// One event per axis that actually moved, so the history reads as
		// what changed rather than as a list of times someone opened the
		// form. A no-op edit records nothing.
		if !sameRoles(splitRoles(previousRoles), next.roles) {
			if err := RecordMembershipEvent(r.Context(), tx, MembershipEvent{
				PracticeID: practiceID, StaffID: targetStaffID, Type: "roles_changed",
				PreviousRoles: "{" + previousRoles + "}", Roles: next.rolesLiteral,
				ActorStaffID: actorStaffID,
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		if previousEmploymentType != next.employmentType {
			if err := RecordMembershipEvent(r.Context(), tx, MembershipEvent{
				PracticeID: practiceID, StaffID: targetStaffID, Type: "employment_type_changed",
				PreviousEmploymentType: previousEmploymentType, EmploymentType: next.employmentType,
				ActorStaffID: actorStaffID,
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		updated := UpdateMembershipResponse{StaffID: targetStaffID, Roles: next.roles, EmploymentType: next.employmentType}
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
func removesLastOwner(ctx context.Context, tx *sql.Tx, practiceID, targetStaffID, previousRoles string, nextRoles []string) (bool, error) {
	if !slices.Contains(splitRoles(previousRoles), "owner") || slices.Contains(nextRoles, "owner") {
		return false, nil
	}
	var otherOwners bool
	err := tx.QueryRowContext(ctx,
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

// RemoveMembershipHandler ends a Staff member's Membership at the current
// Practice -- the route LV-G8 (#291) found missing, without which a
// roster row nobody wants can never be taken off. The staff row survives:
// a person is not owned by one Practice, and she may hold a Membership
// elsewhere or be invited back here later. What she did while she was
// here (her Visits, her Messages) survives too -- removing a Membership
// ends her reach, it does not unwrite the record.
//
// The removal itself is a delete rather than an ended_at column, so the
// activity row this writes first is the only place who
// removed her and when survives. Must be mounted behind
// staffauth.Middleware.
func RemoveMembershipHandler() http.Handler {
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

		var roles, employmentType string
		err := tx.QueryRowContext(r.Context(),
			`SELECT array_to_string(roles, ','), employment_type::text
			   FROM practice_memberships
			  WHERE practice_id = $1 AND staff_id = $2
			  FOR UPDATE`,
			practiceID, targetStaffID,
		).Scan(&roles, &employmentType)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Removing a Membership removes every role on it, so the last-Owner
		// rule applies here for the same reason it applies to an edit.
		lastOwner, err := removesLastOwner(r.Context(), tx, practiceID, targetStaffID, roles, nil)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		if lastOwner {
			http.Error(w, "a practice must keep at least one Owner", http.StatusConflict)
			return
		}

		// The event goes first: it names the roles and employment type the
		// Membership held, which the next statement destroys.
		if err := RecordMembershipEvent(r.Context(), tx, MembershipEvent{
			PracticeID: practiceID, StaffID: targetStaffID, Type: "removed",
			PreviousRoles: "{" + roles + "}", PreviousEmploymentType: employmentType,
			ActorStaffID: actorStaffID,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`DELETE FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
			practiceID, targetStaffID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
