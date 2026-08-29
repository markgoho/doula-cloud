// Package clientfieldtemplate holds the Staff-side BFF handlers for a
// Practice's Client Field Template -- the extra facts an Owner or Admin
// defines that a Client record holds beyond ADR-0017's twelve structural
// columns. See docs/adr/0017-twelve-columns-a-practice-defined-layer-and-
// an-engagement-that-is-asked-for.md, "The two layers, and the departure
// from ADR-0001". One row per Practice (client_field_templates), never
// deleted: removing a field archives it, and archived-but-held values
// stay live-read against this same table forever. All handlers rely on
// staffauth.Middleware having already resolved the caller's Staff/Practice
// ids and opened a request-scoped *sql.Tx with app.current_practice_id
// set, the same way plans.template.go does for its sibling screen.
package clientfieldtemplate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
)

// Field is one entry in a Client Field Template's field list. Order is
// not read from request bodies -- see normalizeFields -- but is always
// present on GET responses, reflecting each field's position in the
// array. Archived is the mechanism ADR-0017's departure from ADR-0001
// requires: a Client's values are read live, so a field can never
// disappear from the array (see normalizeFields), only stop being
// collected.
type Field struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Label    string   `json:"label"`
	Options  []string `json:"options,omitempty"`
	Order    int      `json:"order"`
	Archived bool     `json:"archived"`
}

// TemplateResponse is the body of both the GET response and the PUT
// request/response: a Practice's Client field list.
type TemplateResponse struct {
	Fields []Field `json:"fields"`
}

// GetHandler lets any Staff member at the current Practice read its
// Client Field Template. Unlike plans.GetTemplateHandler, a missing row
// is not a 404: empty-by-default is the designed state for a new
// Practice (ADR-0017), not a gap, so the settings screen's first visit
// gets an empty list rather than an error. Must be mounted behind
// staffauth.Middleware.
func GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		fields, err := FetchFields(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(TemplateResponse{Fields: fields}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// PutHandler lets an Owner or Admin replace the full field list of the
// current Practice's Client Field Template -- add, reorder, archive and
// unarchive, never a bare delete (normalizeFields refuses one). Every
// change that actually alters the list writes one activity row naming
// the actor, in the same transaction as the update. Must be mounted
// behind staffauth.Middleware.
func PutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		var req TemplateResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		previous, err := FetchFields(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		fields, errMsg := normalizeFields(req.Fields, previous)
		if errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		if err := saveFields(r.Context(), tx, practiceID, staffID, previous, fields); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(TemplateResponse{Fields: fields}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// FetchFields reads the field list stored for practiceID, returning an
// empty (never nil) slice if no row exists yet -- a new Practice starts
// with no client_field_templates row at all, and that is the designed
// empty-by-default state, not a missing one. Exported so client.resolveFields
// can resolve a Client's field_values against the same query, rather than
// the client package running its own copy of it.
func FetchFields(ctx context.Context, tx *sql.Tx, practiceID string) ([]Field, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx,
		`SELECT fields FROM client_field_templates WHERE practice_id = $1`,
		practiceID,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return []Field{}, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("clientfieldtemplate: fetch fields: %w", err)
	}

	var fields []Field
	if err := json.Unmarshal(raw, &fields); err != nil {
		// coverage:ignore reason: stored JSON is always written by normalizeFields, not exercised by unit tests
		return nil, fmt.Errorf("clientfieldtemplate: unmarshal fields: %w", err)
	}
	return fields, nil
}

// saveFields upserts practiceID's field list and, when fields actually
// differs from previous, writes one activity row (subject_kind
// 'client_field_template', ADR-0022) holding both sides -- skipped on a
// no-op PUT (e.g. re-saving an unchanged list) so the audit trail stays
// one row per real act.
func saveFields(ctx context.Context, tx *sql.Tx, practiceID, staffID string, previous, fields []Field) error {
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		// coverage:ignore reason: Field always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("clientfieldtemplate: marshal fields: %w", err)
	}

	var templateID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, $2)
		 ON CONFLICT (practice_id) DO UPDATE SET fields = EXCLUDED.fields, updated_at = now()
		 RETURNING id`,
		practiceID, fieldsJSON,
	).Scan(&templateID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientfieldtemplate: save fields: %w", err)
	}

	previousJSON, err := json.Marshal(previous)
	if err != nil {
		// coverage:ignore reason: Field always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("clientfieldtemplate: marshal previous fields: %w", err)
	}
	if string(previousJSON) == string(fieldsJSON) {
		return nil
	}

	diff, err := json.Marshal(map[string]json.RawMessage{"before": previousJSON, "after": fieldsJSON})
	if err != nil {
		// coverage:ignore reason: both sides always marshal cleanly, not exercised by unit tests
		return fmt.Errorf("clientfieldtemplate: marshal diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: "client_field_template",
		SubjectID:   templateID,
		Action:      "updated",
		Diff:        diff,
		Actor:       activity.StaffActor(staffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("clientfieldtemplate: record audit event: %w", err)
	}
	return nil
}
