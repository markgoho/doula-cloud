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
// creation time, held to the same two rules: `assignee` decides whether
// this caller may name that person (an Owner or an Admin may; a plain
// Doula may only ever name herself), and `requireEligibleAssignee`
// decides whether that person may be named.
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
		staffID, isSelf, ok := assignee(w, c, &req.StaffID)
		if !ok {
			return
		}
		isEmployee := !c.reader.IsContractor()
		if !isSelf {
			employmentType, eligible := requireEligibleAssignee(w, r, c, engagementID, staffID)
			if !eligible {
				return
			}
			isEmployee = employmentType == employeeType
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

		// The employee the Visit was handed to is now on this birth, so
		// she gets a granted attachment even though she is not the actor
		// -- ADR-0008's "an Admin scheduling her onto a Visit ... that is
		// a granted attachment, written explicitly". attached_by is the
		// person who did the handing, not the person handed to. A
		// contractor needs none: the check above already proved she holds
		// the one her own acceptance opened.
		if isEmployee {
			if err := staffauth.Grant(r.Context(), c.tx, engagementID, staffID, c.staffID, nil, nil); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, ReassignResponse{VisitID: visitID, StaffID: staffID})
	})
}
