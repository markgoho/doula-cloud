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
// Who may do which of those two is `assignee`'s rule, and a named Staff
// member has to pass `requireEligibleAssignee` -- the same helper the
// reassign path uses, so the two can never drift apart.
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
		staffID, isSelf, ok := assignee(w, c, req.StaffID)
		if !ok {
			return
		}

		// Whether the person this Visit lands on gets a granted
		// attachment turns on *her* employment type, not the caller's.
		// For a colleague, requireEligibleAssignee has just read it off
		// her Membership; for the caller herself, her own Reader already
		// carries it and no second query is needed.
		isEmployee := !c.reader.IsContractor()
		if !isSelf {
			employmentType, eligible := requireEligibleAssignee(w, r, c, engagementID, staffID)
			if !eligible {
				return
			}
			isEmployee = employmentType == employeeType
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

		// Naming a Doula on a Visit puts her on this birth, which is a
		// granted attachment, not the accrual staffauth.AttachingWrite's
		// seam mints -- ADR-0008 names Visit-create as one of the two
		// places granted is written explicitly. No fee rides it: a fee is
		// only ever copied from an Offer. attached_by is the acting
		// person, who is the caller whether she named herself or a
		// colleague.
		//
		// Only for an employee, though. CONTEXT.md's Attachment entry
		// gives a contractor exactly one way onto a birth -- her own
		// acceptance of an Offer -- so granting here would let her hand
		// herself the reach an Offer exists to ask for. Logging her own
		// Visit gets her the seam's accrued record instead, which is a
		// record of work and never a key; being *named* by an Owner or
		// Admin needs no grant at all, because requireEligibleAssignee
		// has just proved she holds the one her acceptance opened.
		if isEmployee {
			if err := staffauth.Grant(r.Context(), c.tx, engagementID, staffID, c.staffID, nil, nil); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
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
