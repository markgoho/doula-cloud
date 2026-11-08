// Package visit holds the Staff-side BFF handlers for Visit: list, create,
// reassign, schedule, and notes. All five rely on staffauth.Middleware
// having already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// the engagement package's handlers do.
package visit

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
)

// requireVisitPath parses the path's Engagement and Visit ids and
// confirms the Engagement belongs to practiceID, so a Visit can never be
// written under an Engagement at a different Practice. It writes the 400
// or 404 itself.
func requireVisitPath(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID string) (engagementID, visitID string, ok bool) {
	engagementID = r.PathValue("engagementId")
	if !staffauth.ParseUUID(w, "engagement", engagementID) {
		return "", "", false
	}
	visitID = r.PathValue("visitId")
	if !staffauth.ParseUUID(w, "visit", visitID) {
		return "", "", false
	}
	if !staffauth.RequireEngagementAtPractice(w, r, tx, engagementID, practiceID) {
		return "", "", false
	}
	return engagementID, visitID, true
}

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
// caller, c.staffID, never the Visit's assignee -- see
// engagement.ActivateFromIntake, which also decides for itself that an
// Engagement past 'intake' is left alone.
//
// Runs after the caller's own row write and activity entry, so the ledger
// reads in the order the acts happened: the Visit was scheduled, and that
// changed the care phase. It writes the failure response itself and
// reports whether the request may continue, the same polarity
// requireVisitWrite uses.
func activateOnScheduled(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID string, scheduledAt *time.Time) bool {
	if scheduledAt == nil {
		return true
	}
	if _, err := engagement.ActivateFromIntake(r.Context(), c.tx, c.practiceID, engagementID, c.staffID); err != nil {
		// coverage:ignore reason: DB write failure inside the activation, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}
