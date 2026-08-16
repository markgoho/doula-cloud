// Package plans holds the Staff-side BFF handlers for a Practice's Plan
// Templates: the field list a Practice defines for its Care Plan and
// Birth Plan (see docs/adr/0001-practice-defined-plan-templates.md). Both
// handlers rely on staffauth.Middleware having already resolved the
// caller's Staff/Practice ids and opened a request-scoped *sql.Tx with
// app.current_practice_id set, the same way engagement and visit do.
package plans

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"doula-cloud/api/internal/staffauth"
)

// validPlanTypes is the plan_type enum from 00011_plan_templates.sql,
// mirrored here so the :planType path segment can be validated before it
// ever reaches Postgres.
var validPlanTypes = map[string]bool{"care_plan": true, "birth_plan": true}

// validFieldTypes is the field-type palette from ADR-0001: the only kinds
// of field a Plan Template may contain.
var validFieldTypes = map[string]bool{
	"short_text":     true,
	"long_text":      true,
	"single_select":  true,
	"multi_select":   true,
	"checkbox":       true,
	"section_header": true,
}

var selectFieldTypes = map[string]bool{"single_select": true, "multi_select": true}

// Field is one entry in a Plan Template's field list. Order is not read
// from request bodies -- see normalizeFields -- but is always present on
// GET responses, reflecting each field's position in the array.
type Field struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Label   string   `json:"label"`
	Options []string `json:"options,omitempty"`
	Order   int      `json:"order"`
}

// TemplateResponse is the body of both the GET response and the PUT
// request/response: a Practice's field list for one plan type.
type TemplateResponse struct {
	PlanType string  `json:"planType"`
	Fields   []Field `json:"fields"`
}

// planTypeFromPath validates the :planType path segment, writing a 400
// response itself if it isn't a known plan_type enum member.
func planTypeFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	planType := r.PathValue("planType")
	if !validPlanTypes[planType] {
		http.Error(w, "unknown plan type: "+planType, http.StatusBadRequest)
		return "", false
	}
	return planType, true
}

// GetTemplateHandler lets any Staff member at the current Practice read
// its Plan Template for :planType. Must be mounted behind
// staffauth.Middleware.
func GetTemplateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		planType, ok := planTypeFromPath(w, r)
		if !ok {
			return
		}

		fields, found, err := fetchFields(r.Context(), tx, practiceID, planType)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "no plan template found for this practice and plan type", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(TemplateResponse{PlanType: planType, Fields: fields}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// PutTemplateHandler lets a Practice Owner replace the full field list of
// its Plan Template for :planType. Must be mounted behind
// staffauth.Middleware.
func PutTemplateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		roles, err := staffauth.Roles(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !hasOwnerRole(roles) {
			http.Error(w, "only a Practice Owner can do that", http.StatusForbidden)
			return
		}

		planType, ok := planTypeFromPath(w, r)
		if !ok {
			return
		}

		var req TemplateResponse
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		fields, errMsg := normalizeFields(req.Fields)
		if errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			// coverage:ignore reason: Field always marshals cleanly, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, $2, $3)
			 ON CONFLICT (practice_id, plan_type) DO UPDATE SET fields = EXCLUDED.fields`,
			practiceID, planType, fieldsJSON,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(TemplateResponse{PlanType: planType, Fields: fields}); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// normalizeFields validates fields against the ADR-0001 palette and
// returns them with Order rewritten to each field's array position -- the
// array order is the single source of truth for field order, so a
// client-sent Order value is never trusted. Returns a non-empty error
// message, and a nil slice, on the first invalid field.
func normalizeFields(fields []Field) ([]Field, string) {
	seenIDs := make(map[string]bool, len(fields))
	out := make([]Field, len(fields))
	for i, f := range fields {
		if f.ID == "" {
			return nil, "field id is required"
		}
		if seenIDs[f.ID] {
			return nil, "duplicate field id: " + f.ID
		}
		seenIDs[f.ID] = true

		if !validFieldTypes[f.Type] {
			return nil, "unknown field type: " + f.Type
		}
		if f.Label == "" {
			return nil, "field label is required"
		}

		if selectFieldTypes[f.Type] {
			if len(f.Options) == 0 {
				return nil, "field " + f.ID + " requires at least one option"
			}
			if slices.Contains(f.Options, "") {
				return nil, "field " + f.ID + " has a blank option"
			}
		} else if len(f.Options) > 0 {
			return nil, "field " + f.ID + " of type " + f.Type + " may not have options"
		}

		out[i] = Field{ID: f.ID, Type: f.Type, Label: f.Label, Options: f.Options, Order: i}
	}
	return out, ""
}

// hasOwnerRole reports whether roles (as returned by staffauth.Roles)
// includes "owner".
func hasOwnerRole(roles []string) bool {
	return slices.Contains(roles, "owner")
}

// fetchFields reads the field list stored for practiceID and planType,
// reporting found=false if no row exists (e.g. a Practice created before
// this feature landed -- seeding is not backfilled).
func fetchFields(ctx context.Context, tx *sql.Tx, practiceID, planType string) ([]Field, bool, error) {
	var raw []byte
	err := tx.QueryRowContext(ctx,
		`SELECT fields FROM plan_templates WHERE practice_id = $1 AND plan_type = $2`,
		practiceID, planType,
	).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("plans: fetch fields: %w", err)
	}

	var fields []Field
	if err := json.Unmarshal(raw, &fields); err != nil {
		// coverage:ignore reason: stored JSON is always written by normalizeFields, not exercised by unit tests
		return nil, false, fmt.Errorf("plans: unmarshal fields: %w", err)
	}
	return fields, true, nil
}
