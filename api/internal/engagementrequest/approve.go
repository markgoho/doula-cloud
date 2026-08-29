package engagementrequest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/billing"
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
const liveEngagementWarning = "this client already has a live engagement"

// ApproveHandler approves a pending Engagement Request: it is the only
// path in the codebase that creates an engagements row (approve, below),
// and it spends a Credit doing it. Owner or Admin only. Must be mounted
// behind staffauth.Middleware. db is needed only for the
// ErrNoCreditsRemaining path, which must queue the out-of-Credits
// Notification on a connection that survives this request's own
// rollback (mirrors the pre-#397 engagement.CreateHandler this replaces).
func ApproveHandler(db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}
		approverStaffID, _ := staffauth.StaffID(r.Context())

		requestID := r.PathValue("requestId")
		if !staffauth.ParseUUID(w, "request", requestID) {
			return
		}

		engagementID, warning, err := approve(r.Context(), tx, practiceID, requestID, approverStaffID)
		if err != nil {
			writeApproveErr(w, r, db, enq, tx, practiceID, err)
			return
		}

		resp := ApproveResponse{RequestID: requestID, EngagementID: engagementID, State: stateApproved}
		if warning {
			resp.Warning = liveEngagementWarning
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// writeApproveErr turns approve's error into the right response, queuing
// the out-of-Credits Notification and rolling the request's transaction
// back first when the balance was empty -- the one handler in this
// package that must roll back explicitly before responding, because
// Middleware's own deferred Commit would otherwise still persist the
// Engagement row approve() already inserted before ConsumeCredit failed.
func writeApproveErr(w http.ResponseWriter, r *http.Request, db *sql.DB, enq tasknudge.Enqueuer, tx *sql.Tx, practiceID string, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "engagement request not found", http.StatusNotFound)
	case errors.Is(err, errNotPending):
		http.Error(w, "that request is no longer pending", http.StatusConflict)
	case errors.Is(err, billing.ErrNoCreditsRemaining):
		// Read while the request tx (and its app.current_practice_id) is
		// still live -- credit_ledger is practice-tier RLS, and this is
		// the last point in the request that tx is usable for it.
		shouldNotify, notifyCheckErr := billing.ShouldQueueOutOfCreditsNotification(r.Context(), tx, practiceID)
		_ = tx.Rollback()
		if notifyCheckErr != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if shouldNotify {
			if err := billing.QueueOutOfCreditsNotification(r.Context(), db, practiceID, enq); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}
		http.Error(w, "no credits remaining, ask a practice owner or admin to buy more", http.StatusPaymentRequired)
	default:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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
// Returns sql.ErrNoRows when requestID does not exist at this Practice,
// errNotPending when it exists but is no longer pending, and
// billing.ErrNoCreditsRemaining unchanged when the Practice's balance is
// empty -- callers branch on all three.
func approve(ctx context.Context, tx *sql.Tx, practiceID, requestID, approverStaffID string) (engagementID string, warning bool, err error) {
	var clientID, kind, state string
	var dueDate sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT client_id, kind::text, due_date, state::text
		   FROM engagement_requests WHERE id = $1 FOR UPDATE`,
		requestID,
	).Scan(&clientID, &kind, &dueDate, &state)
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
