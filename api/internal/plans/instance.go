package plans

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"doula-cloud/api/internal/activity"
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
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "no plan template found for this practice and plan type", http.StatusNotFound)
			return
		}

		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			// coverage:ignore reason: Field always marshals cleanly, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO plan_instances (engagement_id, plan_type, fields) VALUES ($1, $2, $3)`,
			engagementID, planType, fieldsJSON,
		); err != nil {
			if pgerr.IsUniqueViolation(err) {
				http.Error(w, "a plan instance already exists for this engagement and plan type", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: Answers{}}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
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

		practiceID, _ := staffauth.PracticeID(r.Context())
		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}

		fields, answers, err := fetchInstance(r.Context(), tx, engagementID, planType)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: answers.nonEmpty()}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
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

		fields, _, err := fetchInstance(r.Context(), tx, engagementID, planType)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req PutInstanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Answers = req.Answers.nonEmpty()

		if errMsg := validateAnswers(fields, req.Answers); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		answersJSON, err := json.Marshal(req.Answers)
		if err != nil {
			// coverage:ignore reason: Answers always marshals cleanly, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE plan_instances SET answers = $1 WHERE engagement_id = $2 AND plan_type = $3`,
			answersJSON, engagementID, planType,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())
		staffID, _ := staffauth.StaffID(r.Context())
		diff, err := json.Marshal(map[string]string{"planType": planType})
		if err != nil {
			// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := InstanceResponse{EngagementID: engagementID, PlanType: planType, Fields: fields, Answers: req.Answers}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// fetchInstance reads the field snapshot and answers stored for
// engagementID + planType, reporting sql.ErrNoRows (wrapped, so
// errors.Is still matches) if no Plan Instance exists yet -- callers
// translate that into a 404.
func fetchInstance(ctx context.Context, tx *sql.Tx, engagementID, planType string) ([]Field, Answers, error) {
	var rawFields, rawAnswers []byte
	err := tx.QueryRowContext(ctx,
		`SELECT fields, answers FROM plan_instances WHERE engagement_id = $1 AND plan_type = $2`,
		engagementID, planType,
	).Scan(&rawFields, &rawAnswers)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("plans: fetch instance: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, nil, fmt.Errorf("plans: fetch instance: %w", err)
	}

	var fields []Field
	if err := json.Unmarshal(rawFields, &fields); err != nil {
		// coverage:ignore reason: stored JSON is always written by PostInstanceHandler, not exercised by unit tests
		return nil, nil, fmt.Errorf("plans: unmarshal instance fields: %w", err)
	}
	var answers Answers
	if err := json.Unmarshal(rawAnswers, &answers); err != nil {
		// coverage:ignore reason: stored JSON is always written by PutInstanceHandler, not exercised by unit tests
		return nil, nil, fmt.Errorf("plans: unmarshal instance answers: %w", err)
	}
	return fields, answers, nil
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
