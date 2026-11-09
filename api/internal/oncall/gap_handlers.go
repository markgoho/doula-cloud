package oncall

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// CreateGapHandler records a coverage gap: a time one attached Doula
// cannot be reached, within the Engagement's window, optionally naming
// the colleague on the birth who covers it (#1093).
//
// Saving a gap with nobody covering it queues the content-free "a birth
// needs on-call cover" Notification to every current Owner and Admin in
// the same transaction (ADR-0009, ADR-0010, ADR-0011), and nudges the
// worker (ADR-0013). The act starts it; nothing fires on a clock
// (ADR-0038).
//
// Must be mounted behind staffauth.Middleware and AttachingWrite.
func CreateGapHandler(enq tasknudge.Enqueuer) http.Handler {
	return gapHandler(func(w http.ResponseWriter, r *http.Request, _ staffauth.Call, c gapWrite) {
		var req GapRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		facts, ok := c.accept(w, req)
		if !ok {
			return
		}

		var gapID string
		if err := c.tx.QueryRowContext(r.Context(),
			`INSERT INTO engagement_coverage_gaps
			     (engagement_id, staff_id, covering_staff_id, starts_at, ends_at, reason, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			c.engagementID, facts.StaffID, facts.CoveringStaffID, facts.StartsAt, facts.EndsAt, facts.reason, c.actorID,
		).Scan(&gapID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !c.finish(w, r, enq, gapID, activity.ActionCoverageGapCreated, gapDiff{GapID: gapID, After: &facts}, facts) {
			// coverage:ignore reason: finish only reports false on a DB failure, not exercised by unit tests
			return
		}
		writeGap(w, r, c.tx, gapID, http.StatusCreated)
	}, staffauth.EngagementID())
}

// UpdateGapHandler replaces a live gap's facts. Adding a covering Doula
// queues nothing; a save that leaves the gap uncovered queues the
// Notification again, because the hole it describes may have moved.
//
// Must be mounted behind staffauth.Middleware and AttachingWrite.
func UpdateGapHandler(enq tasknudge.Enqueuer) http.Handler {
	return gapHandler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call, c gapWrite) {
		gapID := call.ID("gapId")
		var req GapRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}

		before, ok := lockGapFor(w, r, c, gapID)
		if !ok {
			return
		}
		after, ok := c.accept(w, req)
		if !ok {
			return
		}

		if _, err := c.tx.ExecContext(r.Context(),
			`UPDATE engagement_coverage_gaps
			    SET staff_id = $2, covering_staff_id = $3, starts_at = $4, ends_at = $5, reason = $6,
			        updated_by = $7, updated_at = now()
			  WHERE id = $1`,
			gapID, after.StaffID, after.CoveringStaffID, after.StartsAt, after.EndsAt, after.reason, c.actorID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !c.finish(w, r, enq, gapID, activity.ActionCoverageGapUpdated, gapDiff{GapID: gapID, Before: &before, After: &after}, after) {
			// coverage:ignore reason: finish only reports false on a DB failure, not exercised by unit tests
			return
		}
		writeGap(w, r, c.tx, gapID, http.StatusOK)
	}, staffauth.EngagementID(), staffauth.PathID("gap", "gapId"))
}

// ClearGapHandler clears a live gap: the Doula can be reached again, or
// the gap was recorded in error. Cleared, never deleted, so the roster's
// history still answers "was she covered that night".
//
// Must be mounted behind staffauth.Middleware and AttachingWrite.
func ClearGapHandler() http.Handler {
	return gapHandler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call, c gapWrite) {
		gapID := call.ID("gapId")
		before, ok := lockGapFor(w, r, c, gapID)
		if !ok {
			return
		}

		if _, err := c.tx.ExecContext(r.Context(),
			`UPDATE engagement_coverage_gaps SET cleared_by = $2, cleared_at = now() WHERE id = $1`,
			gapID, c.actorID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordGap(r.Context(), c.tx, c.practiceID, c.engagementID, c.actorID,
			activity.ActionCoverageGapCleared, gapDiff{GapID: gapID, Before: &before}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}, staffauth.EngagementID(), staffauth.PathID("gap", "gapId"))
}

// gapHandler is staffauth.Handler for a route that writes a coverage gap:
// beginGapWrite has already resolved the Engagement's window and the
// caller's standing, and refused when there is none, before fn runs.
func gapHandler(fn func(w http.ResponseWriter, r *http.Request, call staffauth.Call, c gapWrite), opts ...staffauth.Option) http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call) {
		c, ok := beginGapWrite(w, r, call)
		if !ok {
			return
		}
		fn(w, r, call, c)
	}, opts...)
}

// lockGapFor locks a live gap on the request's Engagement and checks the
// caller may change it, writing the refusal itself when not.
func lockGapFor(w http.ResponseWriter, r *http.Request, c gapWrite, gapID string) (gapFacts, bool) {
	before, err := lockLiveGap(r.Context(), c.tx, c.engagementID, gapID)
	switch {
	case errors.Is(err, errGapNotFound):
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, MsgGapNotFound, nil)
		return gapFacts{}, false
	case err != nil:
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return gapFacts{}, false
	}
	if !c.mayWriteFor(before.StaffID) {
		apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden, MsgOnlyYourOwnGap, nil)
		return gapFacts{}, false
	}
	return before, true
}

// finish records the write and, where the saved gap has nobody covering
// it, queues the Notification and nudges the worker.
func (c gapWrite) finish(w http.ResponseWriter, r *http.Request, enq tasknudge.Enqueuer, gapID string, action activity.EngagementAction, diff gapDiff, saved gapFacts) bool {
	if err := recordGap(r.Context(), c.tx, c.practiceID, c.engagementID, c.actorID, action, diff); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if saved.CoveringStaffID != nil {
		return true
	}
	queued, err := queueGapNotice(r.Context(), c.tx, c.practiceID, gapID, c.actorID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if queued {
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.CoverageGap))
	}
	return true
}

// writeGap re-reads the saved gap and writes it as the response.
func writeGap(w http.ResponseWriter, r *http.Request, tx *sql.Tx, gapID string, status int) {
	g, err := readGap(r.Context(), tx, gapID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	apierr.WriteJSON(w, status, g)
}
