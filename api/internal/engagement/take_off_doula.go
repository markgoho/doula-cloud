package engagement

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// TakeOffDoulaHandler takes a named Doula off an Engagement: the Doula
// row's "Take off" on the Engagement page, for an Owner or an Admin
// (#2596). It is the Practice-takes-her-off trigger ADR-0008 names for
// ending an Attachment, and the counterpart of AttachDoulaHandler.
//
// The Attachment is ended, never deleted (attachment.TakeOff): ended_at
// and ended_by say when and by whom, a doula_taken_off entry names the
// Doula, and the Doula gets one mail unless she is the person who pressed.
// Her booked Visits stay hers and flag themselves on every Staff read
// until someone reassigns them. A contractor's ended-work behavior is the
// one #2512 built: the Engagement now answers her with the work-ended
// refusal.
//
// Taking off the last Doula is allowed; the Engagement then reads "No
// Doula yet" on the Overview (offer.NoDoulaHandler). A DELETE keyed on the
// Doula, and naturally idempotent: a Doula who is not on it gets the same
// 200 and nothing is written. It carries no staffauth.AttachingWrite, for
// AttachSelfHandler's reason. Must be mounted behind staffauth.Middleware,
// and the mount admits only an Owner or an Admin (staffauth.OwnerAndAdmin).
func TakeOffDoulaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		callerID, _ := staffauth.StaffID(r.Context())

		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID := r.PathValue("engagementId")
		if !staffauth.ParseUUID(w, "engagement", engagementID) {
			return
		}
		doulaID := r.PathValue("staffId")
		if !staffauth.ParseUUID(w, "staff", doulaID) {
			return
		}

		var status string
		err := tx.QueryRowContext(r.Context(),
			`SELECT status::text FROM engagements WHERE id = $1 AND practice_id = $2 FOR UPDATE`,
			engagementID, practiceID,
		).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, msgAttachDoulaNoEngagement, http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if _, err := attachment.TakeOff(r.Context(), tx, practiceID, engagementID, doulaID, callerID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		doulas, err := readAttachedDoulas(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, AttachSelfResponse{
			EngagementID:  engagementID,
			Doulas:        doulas,
			CanAttachSelf: canAttachSelf(reader, callerID, status, doulas),
		})
	})
}
