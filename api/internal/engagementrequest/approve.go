package engagementrequest

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// errNotPending separates "no such Request here" from "that Request has
// already been decided", the same 404-vs-409 split offer.errEngagementCompleted
// draws.
var errNotPending = errors.New("engagementrequest: request is not pending")

// ApproveResponse reports the Engagement approval created, alongside the
// second-live-Engagement warning ADR-0017 shows at both request time and
// approval time.
type ApproveResponse struct {
	RequestID    string `json:"requestId"`
	EngagementID string `json:"engagementId"`
	State        string `json:"state"`
	Warning      string `json:"warning,omitempty"`
}

// liveEngagementWarning is the fixed copy ADR-0017's second-live-Engagement
// warning carries at both seats it appears in.
const liveEngagementWarning = "this client already has live work"

// ApproveHandler approves a pending Engagement Request: it is the only
// path in the codebase that creates an engagements row (approve, below),
// and it spends a Credit doing it. Owner or Admin only. Must be mounted
// behind staffauth.Middleware. db is needed only for the
// ErrNoCreditsRemaining path, which must queue the out-of-Credits
// Notification on a connection that survives this request's own
// rollback (mirrors the pre-#397 engagement.CreateHandler this replaces).
//
// Owner and Admin is declared at the mount, not checked here (#1016,
// following #970's and #990's own move): the handler no longer calls
// staffauth.RequireOwnerOrAdmin, because a Doula is refused by the gate
// before this runs. Widening or narrowing this route means editing its
// role list in mount.go.
func ApproveHandler(db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		tx, practiceID, approverStaffID, requestID := c.Tx, c.PracticeID, c.StaffID, c.ID("requestId")

		engagementID, warning, err := approve(r.Context(), tx, practiceID, requestID, approverStaffID)
		if err != nil {
			writeApproveErr(w, r, db, enq, tx, practiceID, err)
			return
		}

		resp := ApproveResponse{RequestID: requestID, EngagementID: engagementID, State: stateApproved}
		if warning {
			resp.Warning = liveEngagementWarning
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	}, staffauth.PathID("request", "requestId"))
}

// writeApproveErr turns approve's error into the right response, queuing
// the out-of-Credits Notification and rolling the request's transaction
// back first when the balance was empty -- the one handler in this
// package that must roll back explicitly before responding, because
// Middleware's own deferred Commit would otherwise still persist the
// Engagement row approve() already inserted before ConsumeCredit failed.
func writeApproveErr(w http.ResponseWriter, r *http.Request, db *sql.DB, enq tasknudge.Enqueuer, tx *sql.Tx, practiceID string, err error) {
	var refusal doulaNotAttachableError
	var notLive clientstanding.RefusalError
	switch {
	case errors.Is(err, sql.ErrNoRows):
		apierr.WriteError(w, "engagement request not found", http.StatusNotFound)
	case errors.Is(err, errNotPending):
		apierr.WriteError(w, "that request is no longer pending", http.StatusConflict)
	case errors.As(err, &notLive):
		// approve refuses before it writes, so the Request stays pending
		// and can still be refused or withdrawn.
		apierr.WriteError(w, notLive.Message, http.StatusConflict)
	case errors.As(err, &refusal):
		// The Doula the Request names can no longer be attached (#1596).
		// The reason names no control, because the approver amends nothing
		// (ADR-0017), so it is the summary's own sentence and carries no
		// Details. approve checks this before it writes, so on
		// ApproveHandler's path the Request is untouched and stays
		// pending. The rollback is for the collapsed path, where the
		// pending row was inserted in this same transaction: a refusal
		// there leaves nothing behind, the rule collapse states for an
		// empty balance.
		_ = tx.Rollback()
		apierr.WriteError(w, refusal.Error(), http.StatusConflict)
	case errors.Is(err, billing.ErrNoCreditsRemaining):
		// Read while the request tx (and its app.current_practice_id) is
		// still live -- credit_ledger is practice-tier RLS, and this is
		// the last point in the request that tx is usable for it.
		shouldNotify, notifyCheckErr := billing.ShouldQueueOutOfCreditsNotification(r.Context(), tx, practiceID)
		_ = tx.Rollback()
		if notifyCheckErr != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if shouldNotify {
			if err := billing.QueueOutOfCreditsNotification(r.Context(), db, practiceID, enq); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		apierr.WriteError(w, "no credits remaining, ask a practice owner or admin to buy more", http.StatusPaymentRequired)
	default:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
	}
}

// approve locks requestID FOR UPDATE, confirms it is pending, creates the
// Engagement with the Request's own kind and due date exactly as asked
// (ADR-0017: "the approver approves or refuses exactly what was
// described"), spends a Credit tagged to the new Engagement, and stamps
// the Request approved. Shared by ApproveHandler and RequestHandler's
// solo-Practice collapsed path -- one rule, not a special case for a
// Practice of one. warning reports ADR-0017's second-live-Engagement
// signal, read before the new Engagement exists so it reflects only
// what the Client already held.
//
// Where the Request names a Doula, the same transaction attaches her
// (attachment.Grant, #1596); where it says "No Doula yet", no Attachment
// is written and the Engagement exists with nobody on it.
//
// Returns sql.ErrNoRows when requestID does not exist at this Practice,
// errNotPending when it exists but is no longer pending,
// clientstanding.RefusalError when the Client record is no longer live and
// doulaNotAttachableError when the Doula it names can no longer be
// attached (nothing has been written at either point), and
// billing.ErrNoCreditsRemaining unchanged when the Practice's balance is
// empty -- callers branch on all five.
func approve(ctx context.Context, tx *sql.Tx, practiceID, requestID, approverStaffID string) (engagementID string, warning bool, err error) {
	var clientID, kind, state string
	var dueDate, doula sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT client_id, kind::text, due_date, state::text, doula_staff_id
		   FROM engagement_requests WHERE id = $1 FOR UPDATE`,
		requestID,
	).Scan(&clientID, &kind, &dueDate, &state, &doula)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, err //nolint:wrapcheck // sql.ErrNoRows is the sentinel callers errors.Is against, per this func's own doc comment
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("engagementrequest: lock request: %w", err)
	}
	if state != statePending {
		return "", false, errNotPending
	}
	// Asked here and not only at the ask: erasure leaves a pending
	// Request pending (it touches no engagement_requests row), so without
	// this an Engagement -- and a spent Credit -- could be made for a
	// Client whose details are already gone.
	if err := clientstanding.ActApproveRequest.RequireErr(ctx, tx, clientID); err != nil {
		return "", false, err //nolint:wrapcheck // clientstanding.RefusalError is what writeApproveErr errors.As against
	}
	// Asked again here, and before any write: days can pass between the
	// ask and the decision, and the named Doula's Membership can end or
	// her Employment type can change in them (ADR-0017's amendment).
	if doula.Valid {
		if err := requireStillAttachable(ctx, tx, practiceID, doula.String); err != nil {
			return "", false, err
		}
	}

	warning, err = hasLiveEngagement(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, err
	}

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO engagements (client_id, practice_id, kind, due_date)
		 VALUES ($1, $2, $3::engagement_kind, $4) RETURNING id`,
		clientID, practiceID, kind, dueDate,
	).Scan(&engagementID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", warning, fmt.Errorf("engagementrequest: insert engagement: %w", err)
	}
	if err := recordEngagementCreated(ctx, tx, practiceID, engagementID, kind, approverStaffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", warning, err
	}
	if doula.Valid {
		// attached_by is the approver, because approval is the act that
		// decides she is on it; the Request already records who asked.
		if err := attachment.Grant(ctx, tx, practiceID, engagementID, doula.String, approverStaffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return "", warning, fmt.Errorf("engagementrequest: attach named doula: %w", err)
		}
	}

	if err := billing.ConsumeCredit(ctx, tx, practiceID, engagementID); err != nil {
		return "", warning, err //nolint:wrapcheck // billing.ErrNoCreditsRemaining is the sentinel callers errors.Is against, per this func's own doc comment
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE engagement_requests
		    SET state = 'approved', decided_by = $1, decided_at = now(), engagement_id = $2
		  WHERE id = $3`,
		approverStaffID, engagementID, requestID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", warning, fmt.Errorf("engagementrequest: stamp approval: %w", err)
	}

	return engagementID, warning, nil
}

// recordEngagementCreated writes #476's activity row for the one path
// that creates an Engagement -- ADR-0022's ledger, subject_kind
// 'engagement' (activity.SubjectEngagement), subject_id the Engagement's
// own new id.
func recordEngagementCreated(ctx context.Context, tx *sql.Tx, practiceID, engagementID, kind, approverStaffID string) error {
	diff, err := json.Marshal(map[string]string{fieldKind: kind})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("engagementrequest: marshal engagement created diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionEngagementCreated),
		Diff:        diff,
		Actor:       activity.StaffActor(approverStaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: record engagement created: %w", err)
	}
	return nil
}
