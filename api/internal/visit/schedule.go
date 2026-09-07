package visit

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// ScheduleRequest carries the Visit's new scheduled instant. ScheduledAt
// nil (an explicit `null` or the key absent) clears it -- a sibling of
// ReassignRequest's single-field shape, and the same tri-state
// parseScheduledAt already gives CreateHandler.
type ScheduleRequest struct {
	ScheduledAt *string `json:"scheduledAt"`
}

// ScheduleResponse confirms a Visit's scheduled instant after the write.
type ScheduleResponse struct {
	VisitID     string  `json:"visitId"`
	ScheduledAt *string `json:"scheduledAt,omitempty"`
}

// ScheduleHandler sets, changes or clears a Visit's scheduled_at (#250) --
// a sibling of ReassignHandler, not an extension of it: the two edit
// unrelated fields, and forcing a caller who only wants to reschedule to
// also restate who the Visit is assigned to (ReassignRequest.StaffID is
// required) would be the wrong shape for this write. Must be mounted
// behind staffauth.Middleware; the caller must hold the Doula role at the
// current Practice.
func ScheduleHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireDoula(w, r)
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
		if err := requireEngagementAtPractice(r.Context(), tx, engagementID, practiceID); err != nil {
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

		// engagement_id is filtered explicitly, on top of the RLS scoping
		// staffauth.Middleware already set up on tx, matching
		// ReassignHandler's own reasoning: a Visit can't be rescheduled
		// via an engagementId/visitId pair that don't actually belong
		// together.
		result, err := tx.ExecContext(r.Context(),
			`UPDATE visits SET scheduled_at = $1 WHERE id = $2 AND engagement_id = $3`,
			scheduledAt, visitID, engagementID,
		)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			// coverage:ignore reason: driver RowsAffected failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if rows == 0 {
			apierr.WriteError(w, "visit not found", http.StatusNotFound)
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitScheduled),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ScheduleResponse{
			VisitID:     visitID,
			ScheduledAt: formatScheduledAt(scheduledAt),
		})
	})
}
