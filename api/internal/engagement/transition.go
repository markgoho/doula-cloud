package engagement

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/staffauth"
)

// doulaRole is the practice_role enum member (00002_practice_staff_tenancy.sql)
// this file cares about: an employee Doula reaches three of ADR-0015's
// four legal moves; reopening (completed -> active) is Owner/Admin only.
const doulaRole = "doula"

// The three engagement_status enum members (00005_client_engagement.sql)
// this file names, exported so a test in engagement_test can build its
// own move tables against the same literals this file switches on,
// rather than a second hand-copied set.
const (
	StatusIntake    = "intake"
	StatusActive    = "active"
	StatusCompleted = "completed"
)

// endingReasons is ADR-0015's fixed six-value vocabulary for
// TransitionRequest.EndingReason, checked here so a caller gets a clean
// 400 rather than the database's own enum-cast error.
var endingReasons = map[string]bool{
	"care_complete":    true,
	"client_withdrew":  true,
	"practice_ended":   true,
	"transferred":      true,
	"no_response":      true,
	"entered_in_error": true,
}

// legalMoves reports the target statuses reader may move an Engagement
// at current status to -- ADR-0015's six-move table narrowed by its role
// table. Shared by TransitionHandler (the write) and DetailHandler (the
// read, via Detail.StatusMoves), so the Engagement hub renders exactly
// the moves the API will accept and neither copy of the rule can drift
// from the other. A contractor Doula (IsAmbientContractor) gets none,
// regardless of status -- ADR-0015's cell is a flat outright refusal, not
// a per-move one. Never nil, matching Reader.Roles()'s own reasoning: a
// caller that JSON-encodes the result sends "[]", not "null".
func legalMoves(reader staffauth.Reader, current string) []string {
	none := []string{}
	if reader.IsAmbientContractor() {
		return none
	}
	canWork := reader.IsOwnerOrAdmin() || reader.Has(doulaRole)
	if !canWork {
		return none
	}
	switch current {
	case StatusIntake:
		return []string{StatusActive, StatusCompleted}
	case StatusActive:
		return []string{StatusCompleted}
	case StatusCompleted:
		if reader.IsOwnerOrAdmin() {
			return []string{StatusActive}
		}
	}
	return none
}

// TransitionRequest carries an Engagement's target status and, when that
// target is 'completed', the ending reason ADR-0015 requires (an
// optional free-text note beside it). EndingReason/EndingNote are
// ignored for any other target -- the six-move table is the contract,
// not a generic field PATCH a caller could half-apply.
type TransitionRequest struct {
	Status       string  `json:"status"`
	EndingReason *string `json:"endingReason,omitempty"`
	EndingNote   *string `json:"endingNote,omitempty"`
}

// TransitionResponse confirms an Engagement's new status and, so the
// Engagement hub can render its next set of controls without a second
// read, the moves the same caller may make from there.
type TransitionResponse struct {
	EngagementID string   `json:"engagementId"`
	Status       string   `json:"status"`
	StatusMoves  []string `json:"statusMoves"`
}

// engagementStatusRow is what TransitionHandler reads before writing --
// both the columns the move itself needs (status, ending_reason,
// ending_note) and enough to answer "was this a real move" and "what did
// the audit row's previous side hold".
type engagementStatusRow struct {
	status       string
	endingReason *string
	endingNote   *string
}

// TransitionHandler is the single Engagement status-transition endpoint
// ADR-0015 specifies (#253), replacing the old completion-only endpoint:
// pre-launch, keeping both would mean the same lifecycle logic living in
// two places, so this one is the whole write surface for
// engagements.status now.
//
// Every legal move writes an engagement_events row (ADR-0015's audit
// table, staff-only, never portal-readable -- see 00090's RLS policy).
// Two of the four moves also write the (portal-visible-by-default)
// activity ledger, matching what #476's vocabulary already reserves for
// them: intake -> active writes ActionCarePhaseChanged, and reaching
// 'completed' by any legal move writes ActionEngagementCompleted and
// runs ADR-0008's completion cascade (open Offers withdrawn, open
// attachments ended) in the same transaction CompleteHandler used to.
// Reopening (completed -> active) writes only the engagement_events row
// -- it is a correction, not itself a fact CONTEXT.md's Activity entry
// names for the ledger.
//
// Re-requesting the status an Engagement already holds is a no-op: no
// field write, no audit row, but the completion cascade still runs, so
// anything a partial earlier run left behind still closes. Must be
// mounted behind staffauth.Middleware.
func TransitionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}

		// The contractor refusal is outright (ADR-0015: "the contractor
		// cell is refused every one of these moves"), so it is checked
		// before the request body is even decoded -- the same ordering
		// CompleteHandler's RequireOwnerOrAdmin already used for its own
		// role gate.
		if reader.IsAmbientContractor() {
			apierr.WriteError(w, "a contractor Doula cannot change an Engagement's status", http.StatusForbidden)
			return
		}
		if !reader.IsOwnerOrAdmin() && !reader.Has(doulaRole) {
			apierr.WriteError(w, "only a Practice Owner, Admin or Doula can do that", http.StatusForbidden)
			return
		}

		var req TransitionRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if req.Status != StatusActive && req.Status != StatusCompleted {
			apierr.WriteError(w, "status must be 'active' or 'completed'", http.StatusBadRequest)
			return
		}
		if req.Status == StatusCompleted {
			if req.EndingReason == nil || !endingReasons[*req.EndingReason] {
				apierr.WriteError(w, "endingReason is required to complete an Engagement", http.StatusBadRequest)
				return
			}
		}

		var current engagementStatusRow
		err := tx.QueryRowContext(r.Context(),
			`SELECT status, ending_reason, ending_note FROM engagements WHERE id = $1`, engagementID,
		).Scan(&current.status, &current.endingReason, &current.endingNote)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "engagement not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		actorStaffID, _ := staffauth.StaffID(r.Context())
		isReopen := current.status == StatusCompleted && req.Status == StatusActive
		isNoOp := current.status == req.Status

		// Every (current, target) pair reaching this point is already
		// legal or a no-op: target is restricted to {active, completed}
		// above, current is one of engagement_status's three members, and
		// the six enumerated pairs that leaves are exactly the four legal
		// moves plus the two same-status no-ops -- ADR-0015's two refused
		// rows (active -> intake, completed -> intake) can never name
		// "intake" as a target at all, so they are already rejected by
		// the 400 above. What is left to gate here is role, not legality.
		if !isNoOp && isReopen && !reader.IsOwnerOrAdmin() {
			apierr.WriteError(w, "only a Practice Owner or Admin can reopen a completed Engagement", http.StatusForbidden)
			return
		}

		if !isNoOp {
			newEndingReason, newEndingNote := current.endingReason, current.endingNote
			switch {
			case req.Status == StatusCompleted:
				newEndingReason, newEndingNote = req.EndingReason, req.EndingNote
			case isReopen:
				newEndingReason, newEndingNote = nil, nil
			}

			result, err := tx.ExecContext(r.Context(),
				`UPDATE engagements SET status = $1, ending_reason = $2, ending_note = $3
				  WHERE id = $4 AND status = $5`,
				req.Status, newEndingReason, newEndingNote, engagementID, current.status,
			)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if n, err := result.RowsAffected(); err != nil || n != 1 {
				// coverage:ignore reason: driver RowsAffected failure or a concurrent writer already moved this Engagement, neither exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			if err := recordStatusEvent(r.Context(), tx, statusEvent{
				practiceID:           practiceID,
				engagementID:         engagementID,
				previousStatus:       current.status,
				status:               req.Status,
				previousEndingReason: current.endingReason,
				endingReason:         newEndingReason,
				previousEndingNote:   current.endingNote,
				endingNote:           newEndingNote,
				actorStaffID:         &actorStaffID,
			}); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}

			if current.status == StatusIntake && req.Status == StatusActive {
				diff, err := json.Marshal(map[string]any{"statusBefore": current.status, "statusAfter": req.Status})
				if err != nil {
					// coverage:ignore reason: marshaling two strings never fails
					apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
					return
				}
				if err := activity.Record(r.Context(), tx, activity.Entry{
					PracticeID:  practiceID,
					SubjectKind: activity.SubjectEngagement,
					SubjectID:   engagementID,
					Action:      string(activity.ActionCarePhaseChanged),
					Diff:        diff,
					Actor:       activity.StaffActor(actorStaffID),
				}); err != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
					return
				}
			}
			if req.Status == StatusCompleted {
				if err := activity.Record(r.Context(), tx, activity.Entry{
					PracticeID:  practiceID,
					SubjectKind: activity.SubjectEngagement,
					SubjectID:   engagementID,
					Action:      string(activity.ActionEngagementCompleted),
					Actor:       activity.StaffActor(actorStaffID),
				}); err != nil {
					// coverage:ignore reason: DB query failure, not exercised by unit tests
					apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
					return
				}
			}
		}

		// The completion cascade runs whenever the Engagement is
		// 'completed' after this request -- on a real intake/active ->
		// completed move above, and on a same-status re-request too, so
		// anything a partial earlier run left behind still closes
		// (CompleteHandler's own idempotence, carried over).
		if req.Status == StatusCompleted {
			if err := offer.CloseOnCompletion(r.Context(), tx, engagementID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if err := staffauth.EndAttachments(r.Context(), tx, engagementID, actorStaffID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, TransitionResponse{
			EngagementID: engagementID,
			Status:       req.Status,
			StatusMoves:  legalMoves(reader, req.Status),
		})
	})
}
