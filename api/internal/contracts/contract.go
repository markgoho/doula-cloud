package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"doula-cloud/api/internal/staffauth"
)

const statusDraft = "draft"

// MergeFieldValues is a Contract's filled-in merge field values, keyed by
// merge field key (e.g. "client_name"), mirroring plans.Answers but
// string-only -- every merge field in a Contract Template's prose is
// plain text (client name, price, engagement dates, scope of service),
// unlike a Plan Template's typed fields.
type MergeFieldValues map[string]string

// ContractResponse is the body of the POST/GET/PUT Contract responses:
// the Practice's Contract Template prose as it was snapshotted at
// creation, the merge field keys parsed out of that prose, and whatever
// has been filled in so far.
type ContractResponse struct {
	EngagementID string           `json:"engagementId"`
	Status       string           `json:"status"`
	Prose        string           `json:"prose"`
	MergeFields  []string         `json:"mergeFields"`
	Values       MergeFieldValues `json:"values"`
}

// PutContractRequest is the body of a PUT Contract request: a full
// replacement of Values, the same "object is the whole state" convention
// PutInstanceHandler uses for Answers. It deliberately carries no Status
// field -- a Contract's status is never client-settable through this
// endpoint, and Go's JSON decoder silently ignores an incoming "status"
// key that has nowhere to bind.
type PutContractRequest struct {
	Values MergeFieldValues `json:"values"`
}

// mergeFieldPattern matches a {{merge_field_key}} placeholder in Contract
// Template prose.
var mergeFieldPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

// extractMergeFields parses the merge field keys out of prose, in
// first-occurrence order with duplicates removed. Merge field keys are a
// pure function of prose, so this runs on every read/write rather than
// storing the list in a second column -- prose is itself frozen at
// Contract creation, so the result is stable for the row's lifetime.
func extractMergeFields(prose string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, match := range mergeFieldPattern.FindAllStringSubmatch(prose, -1) {
		key := match[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// nonEmpty normalizes a nil MergeFieldValues to an empty (non-nil) map,
// so it marshals to `{}` rather than JSON null -- both into the NOT NULL
// merge_field_values column and into an HTTP response.
func (v MergeFieldValues) nonEmpty() MergeFieldValues {
	if v == nil {
		return MergeFieldValues{}
	}
	return v
}

// PostContractHandler creates a Draft Contract for :engagementId,
// snapshotting the Practice's current Contract Template prose. Fails
// with 404 if the Practice has no Contract Template row -- this
// shouldn't happen post-#67 (every Practice gets one seeded at signup),
// but a predictable 404 beats a crash. Fails with 409 if a Contract
// already exists for this Engagement (POST creates; PutContractHandler
// edits). Must be mounted behind staffauth.Middleware.
func PostContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())

		prose, found, err := fetchProse(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "no contract template found for this practice", http.StatusNotFound)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO contracts (engagement_id, prose) VALUES ($1, $2)`,
			engagementID, prose,
		); err != nil {
			if isUniqueViolation(err) {
				http.Error(w, "a contract already exists for this engagement", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       statusDraft,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       MergeFieldValues{},
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// GetContractHandler views the Contract for :engagementId. Must be
// mounted behind staffauth.Middleware.
func GetContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		_, prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       status,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// PutContractHandler replaces the full Values map of the Contract for
// :engagementId -- the prose snapshot itself is fixed at creation and
// never editable via this endpoint. Only permitted while status =
// 'draft'; a Contract that has moved to sent/signed/voided 409s. Must be
// mounted behind staffauth.Middleware.
func PutContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, _, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if status != statusDraft {
			http.Error(w, "contract is no longer a draft", http.StatusConflict)
			return
		}

		var req PutContractRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Values = req.Values.nonEmpty()

		mergeFields := extractMergeFields(prose)
		if errMsg := validateMergeFieldValues(mergeFields, req.Values); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		valuesJSON, err := json.Marshal(req.Values)
		if err != nil {
			// coverage:ignore reason: MergeFieldValues always marshals cleanly, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET merge_field_values = $1 WHERE id = $2`,
			valuesJSON, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       status,
			Prose:        prose,
			MergeFields:  mergeFields,
			Values:       req.Values,
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// fetchContract reads the id, prose snapshot, status, and merge field
// values of engagementID's current Contract -- the most recently created
// row, per created_at DESC -- reporting sql.ErrNoRows (wrapped, so
// errors.Is still matches) if no Contract exists yet -- callers translate
// that into a 404. An Engagement can have more than one Contract row
// once Void-then-recreate (#72) is in play: at most one non-voided row
// at a time (contracts_engagement_id_active_key,
// 00020_contracts_recreate_after_void.sql), plus any number of superseded
// voided rows. "Most recent" is always the right one to surface here --
// either the in-flight Draft/Sent/Signed Contract, or, in the window
// right after a Void before Staff creates the next Draft, the just-voided
// row, so its terminal state still displays. Callers that mutate the row
// (Put/Send/Sign/Void) target the returned id directly, rather than
// re-deriving "the current row" via engagement_id in their own UPDATE,
// so a concurrent create-after-void can never make an UPDATE land on the
// wrong row.
func fetchContract(ctx context.Context, tx *sql.Tx, engagementID string) (id, prose, status string, values MergeFieldValues, err error) {
	var rawValues []byte
	err = tx.QueryRowContext(ctx,
		`SELECT id, prose, status, merge_field_values FROM contracts
		 WHERE engagement_id = $1 ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&id, &prose, &status, &rawValues)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", nil, fmt.Errorf("contracts: fetch contract: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", "", "", nil, fmt.Errorf("contracts: fetch contract: %w", err)
	}

	if err := json.Unmarshal(rawValues, &values); err != nil {
		// coverage:ignore reason: stored JSON is always written by PostContractHandler/PutContractHandler, not exercised by unit tests
		return "", "", "", nil, fmt.Errorf("contracts: unmarshal merge field values: %w", err)
	}
	return id, prose, status, values, nil
}

// validateMergeFieldValues checks each entry in values against
// mergeFields, mirroring plans.validateAnswers's rigor. Returns a
// non-empty error message on the first invalid entry.
func validateMergeFieldValues(mergeFields []string, values MergeFieldValues) string {
	known := make(map[string]bool, len(mergeFields))
	for _, key := range mergeFields {
		known[key] = true
	}

	for key := range values {
		if !known[key] {
			return "unknown merge field key: " + key
		}
	}
	return ""
}
