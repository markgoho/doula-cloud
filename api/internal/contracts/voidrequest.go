package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/staffnotice"
)

// MaxVoidRequestReasonRunes is the longest a void request's reason, and the
// reason for declining one, may be, in runes as the form counts it: three
// lines of a card on the Contracts screen at 1280px, so three requests are
// read on its first screen. A decline reason is read in the same places and
// the same width, so it shares the number (#2966).
const MaxVoidRequestReasonRunes = 120

var msgVoidRequestReasonTooLong = fmt.Sprintf("Use %d characters or fewer for the reason", MaxVoidRequestReasonRunes)

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
// writes it under status, as viewContract does.
func respondWithContract(w http.ResponseWriter, r *http.Request, c staffauth.Call, status int) {
	ct, err := fetchContract(r.Context(), c.Tx, c.EngagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the caller just wrote this row
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	viewContract(w, r, c, status, ct)
}

// viewContract writes ct and every void request against it as the whole
// ContractResponse under status -- the shape the Contract read and every
// void-request write return. priceForReader gates Values (never
// withResolvedPrice unconditionally): the request endpoint is reachable by
// a contractor on a granted attachment, so this must never hand her the
// Practice's price the way a caller blindly copying void.go's own
// response-building would. An ambient contractor never reads
// amount_changed_at either (#968): it would otherwise leak "the price
// changed" with the value itself removed.
func viewContract(w http.ResponseWriter, r *http.Request, c staffauth.Call, status int, ct contractRow) {
	mergeFields := extractMergeFields(ct.Prose)
	voidRequests, err := listVoidRequests(r.Context(), c.Tx, ct.ID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	if c.Reader.IsAmbientContractor() {
		ct.AmountChangedAt = nil
	}
	writeContract(w, r, c, status, ContractResponse{
		EngagementID:    c.EngagementID,
		Status:          ct.Status,
		Prose:           ct.Prose,
		MergeFields:     mergeFields,
		Values:          priceForReader(c.Reader, mergeFields, ct.Values.nonEmpty(), ct.AmountCents),
		AmountChangedAt: ct.AmountChangedAt,
		VoidRequests:    voidRequests,
	})
}

// queueVoidRequestedNotices tells each Owner and Admin of the Practice,
// other than the person who asked, that a request waits (#2953).
func queueVoidRequestedNotices(ctx context.Context, c staffauth.Call, requestID string) error {
	recipients, err := otherOwnersAndAdmins(ctx, c)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	for _, id := range recipients {
		if err := staffnotice.Queue(ctx, c.Tx, staffnotice.Notice{
			PracticeID:       c.PracticeID,
			RecipientStaffID: id,
			Kind:             staffnotice.KindVoidRequested,
			SubjectID:        requestID,
			TriggeredBy:      c.StaffID,
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("contracts: queue void request notice: %w", err)
		}
	}
	return nil
}

func otherOwnersAndAdmins(ctx context.Context, c staffauth.Call) ([]string, error) {
	rows, err := c.Tx.QueryContext(ctx,
		`SELECT staff_id FROM practice_memberships
		  WHERE practice_id = $1 AND staff_id <> $2
		    AND ('owner' = ANY(roles) OR 'admin' = ANY(roles))`,
		c.PracticeID, c.StaffID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: list void request recipients: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("contracts: scan void request recipient: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: iterate void request recipients: %w", err)
	}
	return ids, nil
}

// queueVoidAnswer tells asker what became of her request, unless she is the
// one who decided it (#2953).
func queueVoidAnswer(ctx context.Context, c staffauth.Call, kind staffnotice.Kind, requestID, asker string) error {
	if asker == c.StaffID {
		return nil
	}
	if err := staffnotice.Queue(ctx, c.Tx, staffnotice.Notice{
		PracticeID:       c.PracticeID,
		RecipientStaffID: asker,
		Kind:             kind,
		SubjectID:        requestID,
		TriggeredBy:      c.StaffID,
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("contracts: queue void answer: %w", err)
	}
	return nil
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
	return contractHandler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow) {
		if Status(ct.Status) != StatusSigned {
			apierr.WriteError(w,
				statusClause(Status(ct.Status))+", so you cannot ask for it to be voided. Only a signed Contract can be voided.",
				http.StatusConflict)
			return
		}

		var body postVoidRequestBody
		if !apierr.DecodeJSON(w, r, &body) {
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "reason", msgVoidRequestReasonNeeded)
			return
		}
		if utf8.RuneCountInString(reason) > MaxVoidRequestReasonRunes {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "reason", msgVoidRequestReasonTooLong)
			return
		}

		var requestID string
		if err := c.Tx.QueryRowContext(r.Context(),
			`INSERT INTO contract_void_requests (contract_id, engagement_id, practice_id, requested_by, reason)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id`,
			ct.ID, c.EngagementID, c.PracticeID, c.StaffID, reason,
		).Scan(&requestID); err != nil {
			if pgerr.IsUniqueViolation(err) {
				apierr.WriteError(w, msgVoidRequestAlreadyOpen, http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordEngagementActivity(r.Context(), c, activity.ActionContractVoidRequested, nil); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := queueVoidRequestedNotices(r.Context(), c, requestID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		respondWithContract(w, r, c, http.StatusCreated)
	}, staffauth.Engagement())
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
	return contractHandler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow) {
		var body postVoidRequestDeclineBody
		if !apierr.DecodeJSON(w, r, &body) {
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "reason", msgVoidDeclineReasonNeeded)
			return
		}
		if utf8.RuneCountInString(reason) > MaxVoidRequestReasonRunes {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "reason", msgVoidRequestReasonTooLong)
			return
		}

		var asker string
		err := c.Tx.QueryRowContext(r.Context(),
			`UPDATE contract_void_requests
			    SET status = 'declined', decided_by = $1, decided_at = $5, decline_reason = $2
			  WHERE id = $3 AND contract_id = $4 AND status = 'open'
			  RETURNING requested_by`,
			c.StaffID, reason, c.ID("requestId"), ct.ID, clock.Now(r.Context()),
		).Scan(&asker)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, msgVoidRequestNotFoundOrGone, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := queueVoidAnswer(r.Context(), c, staffnotice.KindVoidDeclined, c.ID("requestId"), asker); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := recordEngagementActivity(r.Context(), c, activity.ActionContractVoidDeclined, nil); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		respondWithContract(w, r, c, http.StatusOK)
	}, staffauth.Engagement(), staffauth.PathID("void request", "requestId"))
}
