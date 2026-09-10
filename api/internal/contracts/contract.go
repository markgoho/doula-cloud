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
	"time"

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
	// AmountChangedAt is when amount_cents last moved after creation --
	// an Owner/Admin override (amount.go) or a rate-driven reprice
	// (#968, practicerate.PutRateHandler) -- so a reader can see the
	// price changed and when without hunting the activity ledger for
	// it. Nil while the Contract still carries its as-created amount.
	// Never set for an ambient contractor (priceForReader), the same
	// gate the "price" merge-field value itself is read through --
	// ADR-0008's money tier as amended by #282.
	AmountChangedAt *time.Time `json:"amountChangedAt,omitempty"`
	// VoidRequests is #971's own addition: every void request against
	// this Contract row, newest first, regardless of who asked or who is
	// reading -- unlike Values, a request's reason and outcome carry no
	// price, so nothing here needs ADR-0008's contractor redaction.
	// Populated by GetContractHandler and by every void-request write
	// handler's own response; omitted (not merely empty) for a Contract
	// nobody has ever asked to void, which is the common case.
	VoidRequests []VoidRequestSummary `json:"voidRequests,omitempty"`
	// HasSignedPDF reports that a Signed PDF exists for this Engagement
	// -- the one fact a screen needs to decide whether to offer the
	// download control, and the same fact the Signed-PDF routes key on
	// (signedPDFObjectPath). Never omitempty: a screen reading this
	// field has to be able to tell false from absent, and false is the
	// answer that hides a control. Not a status read: a voided Contract
	// whose PDF was preserved (#299) reports true, and a Draft or an
	// unsigned Sent Contract reports false. Filled by writeContract, not
	// by any handler for itself, so a route added later cannot forget it
	// and quietly hide a Client's own copy of what she signed (#1119).
	HasSignedPDF bool `json:"hasSignedPdf"`
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

// writeContract is how every Contract-shaped response leaves this
// package. It fills the one ContractResponse field no construction site
// computes for itself -- HasSignedPDF, read through signedPDFObjectPath,
// the same lookup the Signed-PDF routes stream from -- and then writes
// out under status.
//
// It exists so the fact a screen gates its download control on and the
// fact the route enforces are one fact rather than two that agree today
// (#1119). A handler builds the rest of the response and hands it over;
// forgetting to ask about the PDF is not a thing a caller can do,
// because asking is not a caller's job.
//
// out.EngagementID is the Engagement asked about: every construction
// site sets it, and it is the same id the caller's own tx is scoped to.
func writeContract(w http.ResponseWriter, r *http.Request, tx *sql.Tx, status int, out ContractResponse) {
	_, hasSignedPDF, err := signedPDFObjectPath(r.Context(), tx, out.EngagementID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	out.HasSignedPDF = hasSignedPDF
	apierr.WriteJSON(w, status, out)
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

		// #967: a Contract's amount is snapshotted from the Practice's
		// rate card at creation, never left for Staff to type in. No
		// rate for the Engagement's kind means no Contract -- refused
		// before any row is written, naming the kind so an Owner or
		// Admin knows exactly what to set.
		amountCents, kind, hasRate, err := resolveContractAmount(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !hasRate {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, noRateSetMsg(kind), nil)
			return
		}

		valuesJSON, err := json.Marshal(values)
		if err != nil {
			// coverage:ignore reason: MergeFieldValues always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO contracts (engagement_id, prose, merge_field_values, amount_cents) VALUES ($1, $2, $3, $4)`,
			engagementID, prose, valuesJSON, amountCents,
		); err != nil {
			if pgerr.IsUniqueViolation(err) {
				apierr.WriteError(w, "a contract already exists for this engagement", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// #972: ActionContractCreated carries no price -- it is no longer
		// in the money set (an employed Doula performs this act herself
		// under #282), so any diff here is unrestricted and readable by
		// a contractor too. The price this Contract was created with is
		// recorded separately below, as ActionContractPriced, which
		// stays in the money set.
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

		// ActionContractPriced: amountCentsBefore is always 0 here --
		// nothing existed to have carried a price before this Contract
		// did -- mirroring the shape PutContractAmountHandler's own
		// contractAmountDiff already uses for an override.
		pricedDiff, err := json.Marshal(contractAmountDiff{AmountCentsBefore: 0, AmountCentsAfter: amountCents})
		if err != nil {
			// coverage:ignore reason: marshal of a fixed, always-serializable struct never fails
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractPriced),
			Diff:        pricedDiff,
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
			Values:       withResolvedPrice(mergeFields, values, amountCents),
		}
		writeContract(w, r, tx, http.StatusCreated, out)
	})
}

// GetContractHandler views the Contract for :engagementId, for anyone
// who can reach the Engagement at all (narrowed by ADR-0008's attachment
// rule for a contractor Doula). #282 deleted the scope-vs-money split
// this handler used to enforce (ContractScope/ContractFull, ReadContract,
// the money_ merge-field-key convention): the premise both shared -- that
// a Client's money is hidden from a Doula -- was wrong for an employed
// Doula, and #282's named exit condition retires the split entirely
// rather than re-aiming it, since narrowing a contractor's read alone
// would need the same per-field classification back. #969 could not
// close that gap on its own -- with money no longer tagged at all, there
// was no single, reliable key left to gate a contractor's read on -- so
// a contractor on a granted attachment read every merge field value
// unfiltered in the interim, price included. #967's real amount_cents
// column gives price back exactly one reserved key, which is what
// priceForReader gates on here: an Owner, an Admin, and an employed
// Doula all read it resolved; a contractor never does, the same "no
// price key reachable at all" guarantee the deleted split used to give.
// Must be mounted behind staffauth.Middleware.
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

		id, prose, status, values, amountCents, amountChangedAt, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		mergeFields := extractMergeFields(prose)

		// An ambient contractor never reads the Contract's price
		// (priceForReader), so amountChangedAt is withheld from her the
		// same way -- it would otherwise leak "the price changed" even
		// with the value itself removed.
		if reader.IsAmbientContractor() {
			amountChangedAt = nil
		}

		voidRequests, err := listVoidRequests(r.Context(), tx, id)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		full := ContractResponse{
			EngagementID:    engagementID,
			Status:          status,
			Prose:           prose,
			MergeFields:     mergeFields,
			Values:          priceForReader(reader, mergeFields, values.nonEmpty(), amountCents),
			AmountChangedAt: amountChangedAt,
			VoidRequests:    voidRequests,
		}

		writeContract(w, r, tx, http.StatusOK, full)
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

		id, prose, status, _, amountCents, amountChangedAt, err := fetchContract(r.Context(), tx, engagementID)
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

		if _, present := req.Values[priceMergeKey]; present {
			apierr.WriteError(w, "price is resolved automatically from the practice's rate card and cannot be set directly", http.StatusBadRequest)
			return
		}

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
			EngagementID:    engagementID,
			Status:          status,
			Prose:           prose,
			MergeFields:     mergeFields,
			Values:          withResolvedPrice(mergeFields, req.Values, amountCents),
			AmountChangedAt: amountChangedAt,
		}
		writeContract(w, r, tx, http.StatusOK, out)
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
// fetchContract's amountCents return is the real column #967 added --
// every caller that builds a response or renders prose resolves the
// "price" merge field from it via withResolvedPrice, rather than trusting
// any stored copy (there is none: price is never written into
// merge_field_values).
// amountChangedAt is nil while the Contract still carries the amount it
// was created with, and set the moment amount_cents last moved for any
// reason after creation -- an Owner/Admin override (amount.go) or a
// rate-driven reprice (#968, practicerate.PutRateHandler).
func fetchContract(ctx context.Context, tx *sql.Tx, engagementID string) (id, prose, status string, values MergeFieldValues, amountCents int64, amountChangedAt *time.Time, err error) {
	var rawValues []byte
	err = tx.QueryRowContext(ctx,
		`SELECT id, prose, status, merge_field_values, amount_cents, amount_changed_at FROM contracts
		 WHERE engagement_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`,
		engagementID,
	).Scan(&id, &prose, &status, &rawValues, &amountCents, &amountChangedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", nil, 0, nil, fmt.Errorf("contracts: fetch contract: %w", err)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", "", "", nil, 0, nil, fmt.Errorf("contracts: fetch contract: %w", err)
	}

	if err := json.Unmarshal(rawValues, &values); err != nil {
		// coverage:ignore reason: stored JSON is always written by PostContractHandler/PutContractHandler, not exercised by unit tests
		return "", "", "", nil, 0, nil, fmt.Errorf("contracts: unmarshal merge field values: %w", err)
	}
	return id, prose, status, values, amountCents, amountChangedAt, nil
}

// clientNameMergeKey and practiceNameMergeKey are the merge fields this
// package resolves for the caller and stores in merge_field_values --
// per #258's brief, the Practice name and the Client's legal name
// (ADR-0017: client_name always resolves to the legal name, never the
// preferred one). Every other merge field -- engagement dates, scope of
// service, or any ad hoc token a Practice Owner wrote into its own
// prose -- has no column backing it and stays blank for Staff to fill
// in via PutContractHandler.
//
// #967 overturns this comment's original limit ("do not add columns to
// grow this set") for exactly one more key: priceMergeKey (price.go).
// The recorded reason a Contract's price is different from these two:
// client_name duplicates a fact that already lives on the Client, so
// growing this set with more of those would be duplication; a price has
// no other home in the system at all. It is not declared alongside these
// two because it does not work the same way -- it is resolved fresh at
// read/render time (withResolvedPrice) from a real column
// (contracts.amount_cents), never written into this row's
// merge_field_values, so there is no stored copy for PutContractHandler
// to let Staff overwrite the way it lets them overwrite these two.
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
