package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
)

// The json tags of the two name fields, which signup, Invitation
// acceptance and the account page's own correction all key their
// refusals under (#1537).
const (
	fieldFirstName = "firstName"
	fieldLastName  = "lastName"
)

const (
	// MsgFirstNameNeeded is the refusal for an empty first name, in
	// GOV.UK's wording rule for an empty field ("Enter your ..."). One
	// refusal per field, so a person who left both empty reads both and
	// fixes both in one pass (#488).
	MsgFirstNameNeeded = "Enter your first name"
	// MsgLastNameNeeded is the same refusal for an empty last name.
	MsgLastNameNeeded = "Enter your last name"

	// MsgNamesRequired is the summary line carried in APIError.Message
	// for a caller reading the API; the two sentences above are for the
	// person looking at the controls.
	MsgNamesRequired = "firstName and lastName are required"
)

// normalizeNames trims both fields and reports, per field, what is
// missing. details is nil when both are present. It is the server-side
// half of the requirement: the form marks both fields as needed, but a
// form is not an enforcement boundary.
func normalizeNames(first, last string) (string, string, map[string]string) {
	first, last = strings.TrimSpace(first), strings.TrimSpace(last)
	var details map[string]string
	if first == "" {
		details = map[string]string{fieldFirstName: MsgFirstNameNeeded}
	}
	if last == "" {
		if details == nil {
			details = map[string]string{}
		}
		details[fieldLastName] = MsgLastNameNeeded
	}
	return first, last, details
}

// RecordNameStated appends the onboarding row to the name audit trail:
// who said she has a name, and when. It holds no name -- see 00124. The
// sibling of RecordNameChange.
func RecordNameStated(ctx context.Context, tx *sql.Tx, staffID, actorStaffID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO staff_name_events (staff_id, kind, actor_staff_id)
		 VALUES ($1, 'stated', $2)`,
		staffID, actorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record first name: %w", err)
	}
	return nil
}

// RecordNameChange appends the row that says a name moved: who said so,
// and when. It holds no name -- see 00124.
func RecordNameChange(ctx context.Context, tx *sql.Tx, staffID, actorStaffID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO staff_name_events (staff_id, kind, actor_staff_id)
		 VALUES ($1, 'changed', $2)`,
		staffID, actorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record name change: %w", err)
	}
	return nil
}

// UpdateNameRequest is the whole body of a name correction. Like
// UpdateWorkStateRequest it carries no staff id: the route only ever
// writes the caller's own row.
type UpdateNameRequest struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// NameResponse is what a correction returns: the stored names.
type NameResponse struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// UpdateNameHandler lets a Staff member correct her own name (#1537).
// Self-edit only, by shape: the route carries no staff id and the row
// written is the one the session cookie's identity resolves to, with
// staff_self_update (00044) as the same rule at the boundary that can
// enforce it. Mounted outside the Practice-scoped middleware for the
// same reason UpdateWorkStateHandler is: a name is a fact about a
// person, not about a Membership.
//
// The change and its audit row share one transaction, so a correction
// that fails halfway leaves neither a changed name with no event nor an
// event describing a change that did not happen.
func UpdateNameHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db, authn.TierStaff)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req UpdateNameRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		first, last, details := normalizeNames(req.FirstName, req.LastName)
		if details != nil {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, MsgNamesRequired, details)
			return
		}

		self, ok := requireSelf(w, r, tx, uid)
		if !ok {
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE staff SET first_name = $1, last_name = $2 WHERE id = $3`,
			first, last, self.ID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := RecordNameChange(r.Context(), tx, self.ID, self.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		apierr.WriteJSON(w, http.StatusOK, NameResponse{FirstName: first, LastName: last})
	})
}
