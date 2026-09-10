package visit

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
)

// CreateRequest carries who the Visit is for and when it happens, both
// optional. StaffID names a colleague (#268) -- absent, null, or the
// caller's own id all mean "me", which is what a Doula logging her own
// Visit sends and is exactly what this route did before the field
// existed. It is named `staffId`, the same as ReassignRequest.StaffID, so
// the two moments of the one act take the one word.
//
// ScheduledAt is a pointer for the same absent/null/no-body reason (see
// apierr.DecodeJSONOptional). A *string here, not a *time.Time: a
// malformed value (json.Unmarshal failing straight into a time.Time)
// would otherwise surface as DecodeJSONOptional's generic "invalid
// request body" 400 rather than parseScheduledAt's own "scheduledAt must
// be an RFC3339 timestamp" -- deliberately narrower than
// CreateResponse/list.Visit's *time.Time below, which have no such
// format-message to lose.
type CreateRequest struct {
	StaffID     *string `json:"staffId"`
	ScheduledAt *string `json:"scheduledAt"`
}

// CreateResponse identifies the Visit row created. ScheduledAt is
// *time.Time, matching list.Visit's own field -- encoding/json already
// marshals it to RFC3339(Nano), so what this write echoes back is the
// exact value list.ListHandler would read for the same row, with no
// second hand-formatted copy to drift out of sync with it.
type CreateResponse struct {
	VisitID     string     `json:"visitId"`
	StaffID     string     `json:"staffId"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
}

// CreateHandler creates a Visit under an Engagement, assigned to the
// Staff member the body names or, with no name in it, to the caller
// herself. Must be mounted behind staffauth.Middleware.
//
// A Visit created with a scheduledAt can move the Engagement itself from
// 'intake' to 'active' (#895); one created without leaves it alone. See
// the activation call at the end of this handler.
//
// Who may do which of those two, whether a named Staff member may be
// named at all, and whether she is granted an attachment for it are all
// `resolveAssignee`'s -- the one seam the reassign path uses too, so the
// two moments of the act can never drift apart.
//
// The role decision therefore has to come *after* the body is decoded,
// which is why this handler no longer opens with a role gate. A caller
// who cannot reach the Engagement at all is already refused upstream by
// staffauth.AttachingWrite, before any of this runs.
func CreateHandler() http.Handler {
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
		if err := requireEngagementAtPractice(r.Context(), c.tx, engagementID, c.practiceID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var req CreateRequest
		if !apierr.DecodeJSONOptional(w, r, &req) {
			return
		}
		if req.StaffID != nil && !staffauth.ParseUUID(w, "staff", *req.StaffID) {
			return
		}
		scheduledAt, ok := parseScheduledAt(w, req.ScheduledAt)
		if !ok {
			return
		}
		staffID, isEmployee, ok := resolveAssignee(w, r, c, engagementID, req.StaffID)
		if !ok {
			return
		}

		visitID := uuid.NewString()
		if _, err := c.tx.ExecContext(r.Context(),
			`INSERT INTO visits (id, engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3, $4)`,
			visitID, engagementID, staffID, scheduledAt,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		// assignedStaffId is on every entry, not only the naming-a-colleague
		// ones: "who was this Visit put on, and by whom" is one question, and
		// an entry that answers it only sometimes cannot be read back as an
		// answer at all. The actor is the Entry's own Actor, so the two
		// together say who did it and to whom.
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
			Action:      string(activity.ActionVisitLogged),
			Diff:        diff,
			Actor:       activity.StaffActor(c.staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The attachment a named employee gets is grantAssignee's rule,
		// shared with the reassign path. It runs here, after the row and
		// the activity entry, so nothing is attached on a write that
		// failed.
		if !grantAssignee(w, r, c, engagementID, staffID, isEmployee) {
			// coverage:ignore reason: grantAssignee only reports false on a DB write failure, not exercised by unit tests
			return
		}

		// ADR-0015's one automatic status move (#895). A Visit created
		// already scheduled is "the first time a Visit is scheduled" just
		// as much as a later PATCH .../schedule is, so both write paths
		// call the same activation; one created with no scheduledAt (the
		// "log a past meeting" shape) leaves the Engagement where it is.
		//
		// The actor is c.staffID, the caller, never staffID, the person
		// the Visit is assigned to: ADR-0015 records "the person who
		// scheduled", and an Admin booking a Visit for a colleague is
		// that person.
		if scheduledAt != nil {
			if err := engagement.ActivateOnVisitScheduled(r.Context(), c.tx, c.practiceID, engagementID, c.staffID); err != nil {
				// coverage:ignore reason: DB write failure inside the activation, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusCreated, CreateResponse{
			VisitID:     visitID,
			StaffID:     staffID,
			ScheduledAt: scheduledAt,
		})
	})
}
