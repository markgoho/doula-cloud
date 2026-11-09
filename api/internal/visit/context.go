// Package visit holds the Staff-side BFF handlers for Visit: list, create,
// reassign, schedule, and notes. All five rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// the engagement package's handlers do.
package visit

import (
	"database/sql"
	"encoding/json/v2"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
)

// parseScheduledAt turns a request's optional RFC3339 scheduledAt string
// into a *time.Time, shared by CreateHandler and ScheduleHandler so the
// two writes agree on the one format a caller may send. raw == nil (the
// field absent, explicitly null, or the whole body absent) means
// unscheduled and returns (nil, true) -- not an error. A raw value that
// fails to parse writes its own 400 and returns ok=false, the caller's
// signal to return without doing anything else.
func parseScheduledAt(w http.ResponseWriter, raw *string) (scheduledAt *time.Time, ok bool) {
	if raw == nil {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		apierr.WriteError(w, "scheduledAt must be an RFC3339 timestamp", http.StatusBadRequest)
		return nil, false
	}
	return &parsed, true
}

// activateOnScheduled applies ADR-0015's one automatic status move to a
// Visit write (#895): "`intake` -> `active` happens by itself, the first
// time a Visit is scheduled." Shared by CreateHandler and ScheduleHandler
// so the rule -- *a write that leaves a scheduled instant behind
// activates; one that leaves none does not* -- is stated once rather than
// once per endpoint, and so a third Visit write path could not quietly
// acquire half of it.
//
// scheduledAt is what the write leaves on the row, so clearing an instant
// (nil) activates nothing: that is the opposite act. The actor is the
// caller, c.StaffID, never the Visit's assignee -- see
// engagement.ActivateFromIntake, which also decides for itself that an
// Engagement past 'intake' is left alone.
//
// Runs after the caller's own row write and activity entry, so the ledger
// reads in the order the acts happened: the Visit was scheduled, and that
// changed the care phase. It writes the failure response itself and
// reports whether the request may continue, the same polarity
// the other Visit writes use.
func activateOnScheduled(w http.ResponseWriter, r *http.Request, c staffauth.Call, engagementID string, scheduledAt *time.Time) bool {
	if scheduledAt == nil {
		return true
	}
	if _, err := engagement.ActivateFromIntake(r.Context(), c.Tx, c.PracticeID, engagementID, c.StaffID); err != nil {
		// coverage:ignore reason: DB write failure inside the activation, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}

// lockVisit runs query, a locking read of one Visit by (id, engagement_id)
// in that placeholder order, and scans it into dest. A Visit that is not
// under that Engagement is this route's 404. It writes the failure
// response itself and reports whether the request may continue.
func lockVisit(w http.ResponseWriter, r *http.Request, tx *sql.Tx, query, visitID, engagementID string, dest ...any) bool {
	if err := tx.QueryRowContext(r.Context(), query, visitID, engagementID).Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "visit not found", http.StatusNotFound)
			return false
		}
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}

// recordVisitActivity writes the Engagement Activity entry a Visit write
// leaves, naming the caller as the actor and carrying diff as its Diff. It
// writes the failure response itself and reports whether the request may
// continue.
func recordVisitActivity(w http.ResponseWriter, r *http.Request, c staffauth.Call, engagementID string, action activity.EngagementAction, diff any) bool {
	encoded, err := json.Marshal(diff)
	if err != nil {
		// coverage:ignore reason: every diff passed here is a map of strings, bools and times, which always marshals cleanly
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if err := activity.Record(r.Context(), c.Tx, activity.Entry{
		PracticeID:  c.PracticeID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Diff:        encoded,
		Actor:       activity.StaffActor(c.StaffID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}
