package plans

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
)

// Answers is a Plan Instance's filled-in values, keyed by field id. Value
// shape depends on the field's type: string for short_text/long_text/
// single_select (the chosen option), []any of string for multi_select
// (the chosen options), bool for checkbox. A section_header field never
// has an answer -- it's a display-only divider, not something to fill in.
type Answers map[string]any

// InstanceResponse is the body of the POST/GET/PUT Plan Instance
// responses: the field snapshot the instance was created against, plus
// whatever has been filled in so far.
type InstanceResponse struct {
	EngagementID string  `json:"engagementId"`
	PlanType     string  `json:"planType"`
	Fields       []Field `json:"fields"`
	Answers      Answers `json:"answers"`

	// ClientAcknowledgedAt is when the Client last confirmed she has read
	// this Plan Instance (#301, v1: Birth Plan only -- always nil for a
	// Care Plan, which never reaches a Client-portal write). Nil means
	// never acknowledged, or acknowledged before a Staff edit since
	// cleared it (PutInstanceHandler).
	ClientAcknowledgedAt *time.Time `json:"clientAcknowledgedAt,omitempty"`
}

// PutInstanceRequest is the body of a PUT Plan Instance request: a full
// replacement of Answers, the same "array/object is the whole state"
// convention PutTemplateHandler uses for Fields.
type PutInstanceRequest struct {
	Answers Answers `json:"answers"`
}

// nonEmpty normalizes a nil Answers to an empty (non-nil) map, so it
// marshals to `{}` rather than JSON null -- both into the NOT NULL
// answers column and into an HTTP response.
func (a Answers) nonEmpty() Answers {
	if a == nil {
		return Answers{}
	}
	return a
}

// PostInstanceHandler creates a Plan Instance for :engagementId +
// :planType, snapshotting the Practice's current Plan Template fields.
// Fails with 404 if the Practice has no template for :planType -- this
// shouldn't happen post-#63 (every Practice gets seeded defaults at
// signup), but a predictable 404 beats a crash or a silently-empty
// snapshot. Fails with 409 if a Plan Instance already exists for this
// Engagement + plan type (POST creates; PutInstanceHandler edits). Must
// be mounted behind staffauth.Middleware.
func PostInstanceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, planType, ok := resolveInstanceRequest(w, r)
		if !ok {
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())

		fields, found, err := fetchFields(r.Context(), tx, practiceID, planType)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			apierr.WriteError(w, "no plan template found for this practice and plan type", http.StatusNotFound)
			return
		}

		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			// coverage:ignore reason: Field always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO plan_instances (engagement_id, plan_type, fields) VALUES ($1, $2, $3)`,
			engagementID, planType, fieldsJSON,
		); err != nil {
			if pgerr.IsUniqueViolation(err) {
				apierr.WriteError(w, "a plan instance already exists for this engagement and plan type", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: Answers{}}
		apierr.WriteJSON(w, http.StatusCreated, out)
	})
}

// GetInstanceHandler views the Plan Instance for :engagementId +
// :planType, narrowed by ADR-0008's attachment rule for a contractor
// Doula, same as engagement.DetailHandler. Must be mounted behind
// staffauth.Middleware.
func GetInstanceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, planType, ok := resolveInstanceRequest(w, r)
		if !ok {
			return
		}

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}

		fields, answers, clientAcknowledgedAt, err := fetchInstance(r.Context(), tx, engagementID, planType)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: answers.nonEmpty(), ClientAcknowledgedAt: clientAcknowledgedAt}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// PutInstanceHandler replaces the full Answers map of the Plan Instance
// for :engagementId + :planType -- the Fields snapshot itself is fixed at
// creation and never editable via this endpoint. Must be mounted behind
// staffauth.Middleware.
func PutInstanceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, planType, ok := resolveInstanceRequest(w, r)
		if !ok {
			return
		}

		fields, _, _, err := fetchInstance(r.Context(), tx, engagementID, planType)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req PutInstanceRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.Answers = req.Answers.nonEmpty()

		if errMsg := validateAnswers(fields, req.Answers); errMsg != "" {
			apierr.WriteError(w, errMsg, http.StatusBadRequest)
			return
		}

		answersJSON, err := json.Marshal(req.Answers)
		if err != nil {
			// coverage:ignore reason: Answers always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// A Staff edit that actually changes the stored answers resets
		// the Client's acknowledgement (#301's decision): the CASE
		// compares the row's pre-update `answers` -- every SET expression
		// in one UPDATE statement sees the same pre-update row -- against
		// the incoming value, atomically, rather than a separate read-then-
		// write that could race a concurrent PUT or acknowledge.
		var clientAcknowledgedAt sql.NullTime
		if err := tx.QueryRowContext(r.Context(),
			`UPDATE plan_instances
			 SET answers = $1::jsonb,
			     client_acknowledged_at = CASE WHEN answers IS DISTINCT FROM $1::jsonb THEN NULL ELSE client_acknowledged_at END
			 WHERE engagement_id = $2 AND plan_type = $3
			 RETURNING client_acknowledged_at`,
			answersJSON, engagementID, planType,
		).Scan(&clientAcknowledgedAt); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())
		staffID, _ := staffauth.StaffID(r.Context())
		diff, err := json.Marshal(map[string]string{"planType": planType})
		if err != nil {
			// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionPlanInstanceEdited),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: req.Answers, ClientAcknowledgedAt: nullTimePtr(clientAcknowledgedAt)}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// fetchInstance reads the field snapshot, answers, and Client
// acknowledgement (#301) stored for engagementID + planType, reporting
// sql.ErrNoRows (wrapped, so errors.Is still matches) if no Plan Instance
// exists yet -- callers
// translate that into a 404.
func fetchInstance(ctx context.Context, tx *sql.Tx, engagementID, planType string) ([]Field, Answers, *time.Time, error) {
	var rawFields, rawAnswers []byte
	var clientAcknowledgedAt sql.NullTime
	err := tx.QueryRowContext(ctx,
		`SELECT fields, answers, client_acknowledged_at FROM plan_instances WHERE engagement_id = $1 AND plan_type = $2`,
		engagementID, planType,
	).Scan(&rawFields, &rawAnswers, &clientAcknowledgedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, fmt.Errorf("plans: fetch instance: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, nil, nil, fmt.Errorf("plans: fetch instance: %w", err)
	}

	var fields []Field
	if err := json.Unmarshal(rawFields, &fields); err != nil {
		// coverage:ignore reason: stored JSON is always written by PostInstanceHandler, not exercised by unit tests
		return nil, nil, nil, fmt.Errorf("plans: unmarshal instance fields: %w", err)
	}
	var answers Answers
	if err := json.Unmarshal(rawAnswers, &answers); err != nil {
		// coverage:ignore reason: stored JSON is always written by PutInstanceHandler, not exercised by unit tests
		return nil, nil, nil, fmt.Errorf("plans: unmarshal instance answers: %w", err)
	}
	return fields, answers, nullTimePtr(clientAcknowledgedAt), nil
}

// nullTimePtr converts a scanned sql.NullTime to a *time.Time, the shape
// InstanceResponse.ClientAcknowledgedAt marshals -- nil rather than a
// zero time.Time when the column is NULL.
func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// validateAnswers checks each entry in answers against the field it
// claims to answer in fields, mirroring normalizeFields's rigor for
// Plan Templates. Returns a non-empty error message on the first invalid
// entry, matching normalizeFields's return shape.
func validateAnswers(fields []Field, answers Answers) string {
	fieldByID := make(map[string]Field, len(fields))
	for _, f := range fields {
		fieldByID[f.ID] = f
	}

	for id, val := range answers {
		f, ok := fieldByID[id]
		if !ok {
			return "unknown field id: " + id
		}

		switch f.Type {
		case "section_header":
			return "field " + id + " of type section_header cannot have an answer"
		case "checkbox":
			if _, ok := val.(bool); !ok {
				return "field " + id + " requires a boolean answer"
			}
		case fieldTypeSingleSelect:
			s, ok := val.(string)
			if !ok || !slices.Contains(f.Options, s) {
				return "field " + id + " requires one of its options"
			}
		case fieldTypeMultiSelect:
			values, ok := val.([]any)
			if !ok {
				return "field " + id + " requires an array of its options"
			}
			for _, v := range values {
				s, ok := v.(string)
				if !ok || !slices.Contains(f.Options, s) {
					return "field " + id + " requires an array of its options"
				}
			}
		default: // short_text, long_text
			if _, ok := val.(string); !ok {
				return "field " + id + " requires a string answer"
			}
		}
	}
	return ""
}
