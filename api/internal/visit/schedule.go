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

		// Read before writing, inside this same transaction: what the
		// audit-trail entry below needs to say more than ReassignHandler's
		// own activity row does -- a set, a change and a clear all edit
		// the identical column, and CLAUDE.md's audit-trail expectation
		// ("how did this come to be?") is not answered by a row that reads
		// the same for all three. engagement_id is filtered explicitly
		// here too, on top of the RLS scoping staffauth.Middleware already
		// set up on tx, matching ReassignHandler's own reasoning: a Visit
		// can't be rescheduled via an engagementId/visitId pair that don't
		// actually belong together. Its NOT FOUND is this route's 404,
		// same as ReassignHandler's own missing-Visit case.
		var previous sql.NullTime
		if err := tx.QueryRowContext(r.Context(),
			`SELECT scheduled_at FROM visits WHERE id = $1 AND engagement_id = $2`,
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

		if _, err := tx.ExecContext(r.Context(),
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
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitScheduled),
			Diff:        diff,
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ScheduleResponse{
			VisitID:     visitID,
			ScheduledAt: scheduledAt,
		})
	})
}
