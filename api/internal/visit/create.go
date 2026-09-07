package visit

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// CreateRequest carries the Visit's scheduled instant, optionally -- the
// only thing POST .../visits ever took no body for. ScheduledAt is a
// pointer so an absent key, an explicit `null`, and no body at all (see
// apierr.DecodeJSONOptional) all mean the same thing: this Visit is not
// yet scheduled. A *string here, not a *time.Time: a malformed value
// (json.Unmarshal failing straight into a time.Time) would otherwise
// surface as DecodeJSONOptional's generic "invalid request body" 400
// rather than parseScheduledAt's own "scheduledAt must be an RFC3339
// timestamp" -- deliberately narrower than CreateResponse/list.Visit's
// *time.Time below, which have no such format-message to lose.
type CreateRequest struct {
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
// calling Staff member. Must be mounted behind staffauth.Middleware; the
// caller must hold the Doula role at the current Practice.
func CreateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := requireDoula(w, r)
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())

		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
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

		var req CreateRequest
		if !apierr.DecodeJSONOptional(w, r, &req) {
			return
		}
		scheduledAt, ok := parseScheduledAt(w, req.ScheduledAt)
		if !ok {
			return
		}

		visitID := uuid.NewString()
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO visits (id, engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3, $4)`,
			visitID, engagementID, staffID, scheduledAt,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionVisitLogged),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		// Logging a Visit puts a named Doula on this birth, which is a
		// granted attachment, not the accrual staffauth.AttachingWrite's
		// seam mints -- ADR-0008 names Visit-create as one of the two
		// places granted is written explicitly. No fee rides it: a fee is
		// only ever copied from an Offer.
		//
		// Only for an employee, though. CONTEXT.md's Attachment entry
		// gives a contractor exactly one way onto a birth -- her own
		// acceptance of an Offer -- so granting here would let her hand
		// herself the reach an Offer exists to ask for. She gets the
		// seam's accrued record instead, which is a record of work and
		// never a key.
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !reader.IsContractor() {
			if err := staffauth.Grant(r.Context(), tx, engagementID, staffID, staffID, nil, nil); err != nil {
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
