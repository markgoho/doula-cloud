package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// ReassignRequest names the Doula a Visit should be reassigned to.
type ReassignRequest struct {
	StaffID string `json:"staffId"`
}

// ReassignResponse confirms who a Visit is now assigned to.
type ReassignResponse struct {
	VisitID string `json:"visitId"`
	StaffID string `json:"staffId"`
}

// ReassignHandler reassigns a Visit's staff_id to a different Doula at the
// same Practice -- coverage/handoff is just editing that field, no
// separate coverage entity. Must be mounted behind staffauth.Middleware.
//
// Naming somebody else here is the same act CreateHandler performs at
// creation time, decided by the same seam: `resolveAssignee` settles
// whether this caller may name that person (an Owner or an Admin may; a
// plain Doula may only ever name herself), whether that person may be
// named, and whether she is granted an attachment for it.
func ReassignHandler() http.Handler {
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

		var req ReassignRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		if !staffauth.ParseUUID(w, "staff", req.StaffID) {
			return
		}
		staffID, isEmployee, ok := resolveAssignee(w, r, c, engagementID, &req.StaffID)
		if !ok {
			return
		}

		// engagement_id is filtered explicitly, on top of the RLS scoping
		// staffauth.Middleware already set up on tx, so a Visit can't be
		// reassigned via an engagementId/visitId pair that don't actually
		// belong together.
		result, err := c.tx.ExecContext(r.Context(),
			`UPDATE visits SET staff_id = $1 WHERE id = $2 AND engagement_id = $3`,
			staffID, visitID, engagementID,
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
		// See CreateHandler's own diff comment: who the Visit was put on
		// rides every entry, so "who acted, who it went to, and when" is
		// readable off one row.
		diff, err := json.Marshal(map[string]string{"assignedStaffId": staffID})
		if err != nil {
			// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), c.tx, activity.Entry{
			PracticeID:  c.practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitReassigned),
			Diff:        diff,
			Actor:       activity.StaffActor(c.staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The attachment the employee handed this Visit gets is
		// grantAssignee's rule, shared with the create path. It runs
		// only here, below the rows == 0 404, so a reassign that matched
		// no Visit attaches nobody.
		if !grantAssignee(w, r, c, engagementID, staffID, isEmployee) {
			// coverage:ignore reason: grantAssignee only reports false on a DB write failure, not exercised by unit tests
			return
		}

		apierr.WriteJSON(w, http.StatusOK, ReassignResponse{VisitID: visitID, StaffID: staffID})
	})
}
