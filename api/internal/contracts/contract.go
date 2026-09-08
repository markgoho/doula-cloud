package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
)

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
// edits). Declared staffauth.AnyStaff at the mount (#282, #970): whoever
// reaches the Engagement at all -- Owner, Admin, an employed Doula, or a
// contractor on a granted attachment -- may create. Must be mounted
// through idempotency.Router.ExemptGated.
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
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			apierr.WriteError(w, "no contract template found for this practice", http.StatusNotFound)
			return
		}

		mergeFields := extractMergeFields(prose)
		values, err := resolveMergeFieldValues(r.Context(), tx, engagementID, mergeFields)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		valuesJSON, err := json.Marshal(values)
		if err != nil {
			// coverage:ignore reason: MergeFieldValues always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO contracts (engagement_id, prose, merge_field_values) VALUES ($1, $2, $3)`,
			engagementID, prose, valuesJSON,
		); err != nil {
			if pgerr.IsUniqueViolation(err) {
				apierr.WriteError(w, "a contract already exists for this engagement", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractCreated),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := ContractResponse{
			EngagementID: engagementID,
			Status:       string(StatusDraft),
			Prose:        prose,
			MergeFields:  mergeFields,
			Values:       values,
		}
		apierr.WriteJSON(w, http.StatusCreated, out)
	})
}

// GetContractHandler views the Contract for :engagementId, in full, for
// anyone who can reach the Engagement at all (narrowed by ADR-0008's
// attachment rule for a contractor Doula). #282 deleted the scope-vs-
// money split this handler used to enforce (ContractScope/ContractFull,
// ReadContract, the money_ merge-field-key convention): the premise both
// shared -- that a Client's money is hidden from a Doula -- was wrong for
// an employed Doula, and #282's named exit condition retires the split
// entirely rather than re-aiming it, since narrowing a contractor's read
// alone would need the same per-field classification back. Until #967
// gives a Contract a real amount column, a contractor on a granted
// attachment reads this Contract's merge field values unfiltered,
// including a money-tagged one if the Practice's Template used the old
// convention -- the interim cost #969 accepts and #967 closes. Must be
// mounted behind staffauth.Middleware.
func GetContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
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
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}

		_, prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		full := ContractResponse{
			EngagementID: engagementID,
			Status:       status,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}

		apierr.WriteJSON(w, http.StatusOK, full)
	})
}

// PutContractHandler replaces the full Values map of the Contract for
// :engagementId -- the prose snapshot itself is fixed at creation and
// never editable via this endpoint. Only permitted while status =
// 'draft'; a Contract that has moved to sent/signed/voided 409s. Declared
// staffauth.AnyStaff at the mount (#282, #970), the same reach-only rule
// PostContractHandler carries -- until #967 gives a Contract a real
// amount column, "set values" still means filling in merge fields, which
// #282's write table treats as scope regardless of what a Practice's own
// Template prose names. Must be mounted through
// idempotency.Router.ExemptGated.
func PutContractHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, _, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if ok, refusal := TransitionEdit.Check(Status(status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		var req PutContractRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.Values = req.Values.nonEmpty()

		mergeFields := extractMergeFields(prose)
		if errMsg := validateMergeFieldValues(mergeFields, req.Values); errMsg != "" {
			apierr.WriteError(w, errMsg, http.StatusBadRequest)
			return
		}

		valuesJSON, err := json.Marshal(req.Values)
		if err != nil {
			// coverage:ignore reason: MergeFieldValues always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET merge_field_values = $1 WHERE id = $2`,
			valuesJSON, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := ContractResponse{
			EngagementID: engagementID,
			Status:       status,
			Prose:        prose,
			MergeFields:  mergeFields,
			Values:       req.Values,
		}
		apierr.WriteJSON(w, http.StatusOK, out)
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
		 WHERE engagement_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
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

// clientNameMergeKey and practiceNameMergeKey are the merge fields this
// package resolves for the caller rather than leaving blank for Staff to
// type -- per #258's brief, the resolvable set today is exactly these
// two: the Practice name and the Client's legal name (ADR-0017:
// client_name always resolves to the legal name, never the preferred
// one). Every other merge field -- engagement dates, price, scope of
// service, or any ad hoc token a Practice Owner wrote into its own
// prose -- has no column backing it and stays blank for Staff to fill
// in via PutContractHandler. Do not add columns to grow this set; #258
// deliberately scoped it to data the product already holds.
const (
	clientNameMergeKey   = "client_name"
	practiceNameMergeKey = "practice_name"
)

// resolveMergeFieldValues returns Draft Values with every merge field in
// the resolvable set (clientNameMergeKey, practiceNameMergeKey) already
// filled in, for whichever of those two the Template's prose actually
// asks for -- one query against the Engagement, its Client and its
// Practice, rather than one query per resolvable field. Every other
// merge field is left for Staff to fill in via PutContractHandler,
// exactly as before.
func resolveMergeFieldValues(ctx context.Context, tx *sql.Tx, engagementID string, mergeFields []string) (MergeFieldValues, error) {
	wantsClientName := slices.Contains(mergeFields, clientNameMergeKey)
	wantsPracticeName := slices.Contains(mergeFields, practiceNameMergeKey)
	if !wantsClientName && !wantsPracticeName {
		return MergeFieldValues{}, nil
	}

	var givenName, practiceName string
	var familyName sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT c.given_name, c.family_name, p.name
		 FROM engagements e
		 JOIN clients c ON c.id = e.client_id
		 JOIN practices p ON p.id = e.practice_id
		 WHERE e.id = $1`,
		engagementID,
	).Scan(&givenName, &familyName, &practiceName); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- resolveContractRequest already proved the Engagement (and therefore its Client and Practice) exists
		return nil, fmt.Errorf("contracts: resolve merge field values: %w", err)
	}

	values := MergeFieldValues{}
	if wantsClientName {
		values[clientNameMergeKey] = client.LegalName(givenName, familyName.String)
	}
	if wantsPracticeName {
		values[practiceNameMergeKey] = practiceName
	}
	return values, nil
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
