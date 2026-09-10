package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// ScheduleRequest carries the Visit's new scheduled instant. ScheduledAt
// nil (an explicit `null` or the key absent) clears it -- a sibling of
// ReassignRequest's single-field shape, and the same tri-state
// parseScheduledAt already gives CreateHandler. A *string, not a
// *time.Time, for the same "keep parseScheduledAt's own 400 message"
// reason CreateRequest's own field is.
type ScheduleRequest struct {
	ScheduledAt *string `json:"scheduledAt"`
}

// ScheduleResponse confirms a Visit's scheduled instant after the write.
// ScheduledAt is *time.Time, matching list.Visit and CreateResponse.
type ScheduleResponse struct {
	VisitID     string     `json:"visitId"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
}

// ScheduleHandler sets, changes or clears a Visit's scheduled_at (#250) --
// a sibling of ReassignHandler, not an extension of it: the two edit
// unrelated fields, and forcing a caller who only wants to reschedule to
// also restate who the Visit is assigned to (ReassignRequest.StaffID is
// required) would be the wrong shape for this write. Must be mounted
// behind staffauth.Middleware.
//
// Setting a scheduled instant has one effect beyond the Visit row: it can
// move the Engagement itself from 'intake' to 'active' (#895). See the
// activation call at the end of this handler.
//
// No role gate of its own (#268). Setting the date of a Visit that
// already exists is the Admin's own job -- ADR-0006 grants her the Staff
// roster precisely because "booking a Visit means picking a Doula" -- so
// gating it on the Doula role refused the one person whose job the
// glossary says this is, while the screen offered her the control anyway.
// What is left is NotesHandler's rule, which is ADR-0008's read rule:
// staffauth.AttachingWrite has already refused any caller who may not
// reach this Engagement at all. Who a Visit is *for* is a different act
// with a stricter rule -- see assignee in roles.go.
func ScheduleHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := requireVisitWrite(w, r)
		// coverage:ignore reason: requireVisitWrite only reports false when staffauth.Middleware left no tx or no Reader on context, which cannot happen behind it
		if !ok {
			return
		}

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		visitID := r.PathValue("visitId")
		if !staffauth.ParseUUID(w, "visit", visitID) {
			return
		}
		if err := requireEngagementAtPractice(r.Context(), c.tx, engagementID, c.practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req ScheduleRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		scheduledAt, ok := parseScheduledAt(w, req.ScheduledAt)
		if !ok {
			return
		}

		// The Visit row is locked before it is read, so scheduledAtBefore
		// below is the value the UPDATE actually overwrites. The shared
		// transaction alone does not give that: under READ COMMITTED a
		// plain SELECT takes no row lock, so two concurrent reschedules
		// could both read the same original instant, and the second
		// entry would claim a "before" the first write had already
		// replaced (#922). SELECT ... FOR UPDATE blocks on a competing
		// writer and then re-reads the row it committed, so the pair
		// recorded is always the move that actually took place -- the
		// same mechanism ReassignHandler's own locking read uses (#887),
		// needed here whichever direction the write goes, since a set, a
		// change and a clear all edit the identical column.
		//
		// engagement_id is filtered explicitly here too, on top of the
		// RLS scoping staffauth.Middleware already set up on tx, matching
		// ReassignHandler's own reasoning: a Visit can't be rescheduled
		// via an engagementId/visitId pair that don't actually belong
		// together. Its NOT FOUND is this route's 404, same as
		// ReassignHandler's own missing-Visit case.
		var previous sql.NullTime
		if err := c.tx.QueryRowContext(r.Context(),
			`SELECT scheduled_at FROM visits WHERE id = $1 AND engagement_id = $2 FOR UPDATE`,
			visitID, engagementID,
		).Scan(&previous); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "visit not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		var previousAt *time.Time
		if previous.Valid {
			previousAt = &previous.Time
		}

		if _, err := c.tx.ExecContext(r.Context(),
			`UPDATE visits SET scheduled_at = $1 WHERE id = $2 AND engagement_id = $3`,
			scheduledAt, visitID, engagementID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		diff, err := json.Marshal(map[string]any{"scheduledAtBefore": previousAt, "scheduledAtAfter": scheduledAt})
		if err != nil {
			// coverage:ignore reason: marshaling two *time.Time values never fails
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), c.tx, activity.Entry{
			PracticeID:  c.practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitScheduled),
			Diff:        diff,
			Actor:       activity.StaffActor(c.staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Setting an instant is the move's trigger; clearing one is the
		// opposite act and activates nothing. Both answers are
		// activateOnScheduled's, shared with CreateHandler.
		if !activateOnScheduled(w, r, c, engagementID, scheduledAt) {
			// coverage:ignore reason: activateOnScheduled only reports false on a DB write failure, not exercised by unit tests
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ScheduleResponse{
			VisitID:     visitID,
			ScheduledAt: scheduledAt,
		})
	})
}
