package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
)

// msgVoidRequestReasonNeeded and msgVoidDeclineReasonNeeded are the two
// reason fields #971 asks for -- imperative, no "please"/"required"/
// "valid" (formErrors.usage.spec.ts, CLAUDE.md's content rule), matching
// engagement/fielderrors.go's own wording.
const (
	msgVoidRequestReasonNeeded   = "Enter why this contract needs to be voided"
	msgVoidDeclineReasonNeeded   = "Enter why this request is being declined"
	msgVoidRequestAlreadyOpen    = "you already have a void request open for this contract"
	msgVoidRequestNotFoundOrGone = "no open void request found with this id for this contract"
)

// VoidRequestSummary is one row of a Contract's void-request history --
// #971's own record of who asked to void a Signed Contract, when, why,
// and (once decided) whether an Owner or an Admin voided it or declined
// with a reason of their own. Carries no price: unlike Values, nothing
// here needs ADR-0008's contractor redaction.
type VoidRequestSummary struct {
	ID              string     `json:"id"`
	RequestedBy     string     `json:"requestedBy"`
	RequestedByName string     `json:"requestedByName"`
	Reason          string     `json:"reason"`
	Status          string     `json:"status"`
	DeclineReason   string     `json:"declineReason,omitempty"`
	DecidedBy       string     `json:"decidedBy,omitempty"`
	DecidedAt       *time.Time `json:"decidedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// postVoidRequestBody and postVoidRequestDeclineBody are the two write
// bodies #971 needs -- each carries exactly one field, the reason a
// person typed, mirroring engagement's own ending-reason shape.
type postVoidRequestBody struct {
	Reason string `json:"reason"`
}

type postVoidRequestDeclineBody struct {
	Reason string `json:"reason"`
}

// listVoidRequests reads every void request against contractID, newest
// first -- the full history, not just what is still open, so a decline
// stays visible beside whatever request replaced it. Shared by
// GetContractHandler, PostVoidRequestHandler, the decline handler, and
// PostVoidContractHandler, all of which embed it in the ContractResponse
// they return.
func listVoidRequests(ctx context.Context, tx *sql.Tx, contractID string) ([]VoidRequestSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT cvr.id, cvr.requested_by, s.name, cvr.reason, cvr.status::text,
		        COALESCE(cvr.decline_reason, ''), COALESCE(cvr.decided_by::text, ''),
		        cvr.decided_at, cvr.created_at
		   FROM contract_void_requests cvr
		   JOIN staff s ON s.id = cvr.requested_by
		  WHERE cvr.contract_id = $1
		  ORDER BY cvr.created_at DESC, cvr.id DESC`,
		contractID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list void requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := []VoidRequestSummary{}
	for rows.Next() {
		var item VoidRequestSummary
		var decidedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.RequestedBy, &item.RequestedByName, &item.Reason, &item.Status,
			&item.DeclineReason, &item.DecidedBy, &decidedAt, &item.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan void request: %w", err)
		}
		if decidedAt.Valid {
			item.DecidedAt = &decidedAt.Time
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate void requests: %w", err)
	}
	return list, nil
}

// respondWithContract re-reads engagementID's current Contract row and
// every void request against it, and writes the whole ContractResponse
// under status -- the shape every Contract write handler returns.
// priceForReader gates Values the same way GetContractHandler does
// (never withResolvedPrice unconditionally): unlike void/amount, the
// request endpoint is reachable by a contractor on a granted attachment,
// so this must never hand her the Practice's price the way a caller
// blindly copying void.go's own response-building would.
func respondWithContract(w http.ResponseWriter, r *http.Request, tx *sql.Tx, engagementID string, status int) {
	id, prose, contractStatus, values, amountCents, amountChangedAt, err := fetchContract(r.Context(), tx, engagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the caller just wrote this row
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	mergeFields := extractMergeFields(prose)
	voidRequests, err := listVoidRequests(r.Context(), tx, id)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	reader, has := staffauth.ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	// #968: an ambient contractor never reads amount_changed_at either,
	// the same withholding GetContractHandler applies -- it would
	// otherwise leak "the price changed" with the value itself removed.
	if reader.IsAmbientContractor() {
		amountChangedAt = nil
	}
	writeContract(w, r, tx, status, ContractResponse{
		EngagementID:    engagementID,
		Status:          contractStatus,
		Prose:           prose,
		MergeFields:     mergeFields,
		Values:          priceForReader(reader, mergeFields, values.nonEmpty(), amountCents),
		AmountChangedAt: amountChangedAt,
		VoidRequests:    voidRequests,
	})
}

// PostVoidRequestHandler is #971's own gap-filler: a Doula who reaches
// :engagementId's Signed Contract at all -- an employee anywhere at the
// Practice, a contractor on a granted attachment (AttachingWrite's reach
// test, the same one PostContractHandler's own AnyStaff declaration
// relies on) -- asks for it to be voided, naming why. Owner and Admin
// reach this route too (they reach every Engagement), though they have
// no reason to ask rather than just voiding directly.
//
// Only a Signed Contract may have a void requested -- the same
// precondition TransitionVoid itself carries, checked directly rather
// than through Transitions/Check since this route never writes the
// contracts row (lifecycle_test.go's guardrail only demands a Transition
// for a route that does). Refuses a second open request from the same
// person (contract_void_requests_one_open_per_requester) with a 409,
// rather than silently letting her ask twice.
//
// Declared staffauth.AnyStaff at the mount (#970's own convention: every
// Contract write, gated or not, goes through ir.ExemptGated), attaching
// so a contractor's reach is checked before the handler runs at all.
// Must be mounted through idempotency.Router.ExemptGated.
func PostVoidRequestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, _, status, _, _, _, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if Status(status) != StatusSigned {
			apierr.WriteError(w,
				fmt.Sprintf("a void may only be requested for a signed contract: it is %s", status),
				http.StatusConflict)
			return
		}

		var body postVoidRequestBody
		if !apierr.DecodeJSON(w, r, &body) {
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgVoidRequestReasonNeeded,
				map[string]string{"reason": msgVoidRequestReasonNeeded})
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		practiceID, _ := staffauth.PracticeID(r.Context())

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO contract_void_requests (contract_id, engagement_id, practice_id, requested_by, reason)
			 VALUES ($1, $2, $3, $4, $5)`,
			id, engagementID, practiceID, staffID, reason,
		); err != nil {
			if pgerr.IsUniqueViolation(err) {
				apierr.WriteError(w, msgVoidRequestAlreadyOpen, http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractVoidRequested),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		respondWithContract(w, r, tx, engagementID, http.StatusCreated)
	})
}

// PostVoidRequestDeclineHandler is the other half of #971's ask: an
// Owner or an Admin refuses one specific open request, naming why --
// distinct from voiding (PostVoidContractHandler), which is how she
// grants one instead. requestId is a path segment of its own rather than
// "the" request, because more than one Doula may hold an open request on
// the same Contract at once (the ticket's own AC: only duplicates from
// the *same* person are refused) -- declining one must never touch
// another still open.
//
// Owner and Admin only, declared at the mount the same way Void and the
// amount override are (#970, #967) -- attaching=false: an Owner or Admin
// bypasses Attachment entirely, the same reasoning amount.go's own mount
// comment gives, so nothing here needs AttachingWrite's reach test.
// Refuses with 404 if requestId does not name a currently-open request
// against this Contract -- already decided, or naming some other
// Contract's request entirely. Must be mounted through
// idempotency.Router.ExemptGated.
func PostVoidRequestDeclineHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}
		requestID := r.PathValue("requestId")
		if !staffauth.ParseUUID(w, "void request", requestID) {
			return
		}

		id, _, _, _, _, _, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var body postVoidRequestDeclineBody
		if !apierr.DecodeJSON(w, r, &body) {
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgVoidDeclineReasonNeeded,
				map[string]string{"reason": msgVoidDeclineReasonNeeded})
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())

		res, err := tx.ExecContext(r.Context(),
			`UPDATE contract_void_requests
			    SET status = 'declined', decided_by = $1, decided_at = now(), decline_reason = $2
			  WHERE id = $3 AND contract_id = $4 AND status = 'open'`,
			staffID, reason, requestID, id,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		affected, err := res.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver always reports rows affected for an UPDATE, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if affected == 0 {
			apierr.WriteError(w, msgVoidRequestNotFoundOrGone, http.StatusNotFound)
			return
		}

		practiceID, _ := staffauth.PracticeID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractVoidDeclined),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		respondWithContract(w, r, tx, engagementID, http.StatusOK)
	})
}
