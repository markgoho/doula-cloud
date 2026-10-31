package engagement

import (
	"database/sql"
	"errors"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// The refusals of AttachDoulaHandler. The Engagement hub prints each as
// it is, so each is a sentence a person can read, and the contractor's
// names the path that is open to her.
const (
	msgAttachDoulaNoEngagement  = "This Engagement was not found at this Practice."
	msgAttachDoulaNotAtPractice = "That person is not on the Staff at this Practice."
	msgAttachDoulaNotADoula     = " does not hold the Doula role, so cannot be put on an Engagement."
	msgAttachDoulaContractor    = " contracts for this Practice, so goes on an Engagement by accepting an Offer. Offer this work instead."
)

// AttachDoulaHandler puts a named Doula on an Engagement: the Doula row's
// Change on the Engagement page, for an Owner or an Admin (#2230). It is
// the fifth writer of a granted Attachment ADR-0008 names, and the one
// GLOSSARY.md's Attachment entry gave the Practice from the start: "an
// Admin may attach an employee directly". An Offer stays the only way on
// for a contractor who holds neither Owner nor Admin, because nobody can
// put an outsider on a Client's birth without her agreement.
//
// Who may be put on is attachment.Membership.WhyNotAttachable, the rule
// the Start work form's list, approval, "Put me on this Engagement" and
// the Visit's pickers read, so this write cannot disagree with them. The
// Attachment is granted, carries no fee, and names the caller as
// attached_by; the doula_attached entry names the caller as the actor
// and the Doula in its diff (attachment.Grant), which is how the ledger
// answers who put her on and when.
//
// A PUT keyed on the Doula, and naturally idempotent: a Doula already on
// it gets the same 200 and nothing is written. A completed Engagement
// refuses, as AttachSelfHandler does. It carries no
// staffauth.AttachingWrite, for AttachSelfHandler's reason. Must be
// mounted behind staffauth.Middleware, and the mount admits only an
// Owner or an Admin (staffauth.OwnerAndAdmin).
func AttachDoulaHandler() http.Handler {
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
		if status == StatusCompleted {
			apierr.WriteError(w, msgAttachSelfIsCompleted, http.StatusConflict)
			return
		}

		membership, err := attachment.ReadMembership(r.Context(), tx, practiceID, doulaID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		switch membership.WhyNotAttachable() {
		case attachment.NotAtPractice:
			apierr.WriteError(w, msgAttachDoulaNotAtPractice, http.StatusUnprocessableEntity)
			return
		case attachment.NotADoula:
			apierr.WriteError(w, membership.Name+msgAttachDoulaNotADoula, http.StatusUnprocessableEntity)
			return
		case attachment.IsContractor:
			apierr.WriteError(w, membership.Name+msgAttachDoulaContractor, http.StatusUnprocessableEntity)
			return
		case attachment.Attachable:
		}

		doulas, err := readAttachedDoulas(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !isOn(doulas, doulaID) {
			if err := attachment.Grant(r.Context(), tx, practiceID, engagementID, doulaID, callerID); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			doulas, err = readAttachedDoulas(r.Context(), tx, engagementID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		apierr.WriteJSON(w, http.StatusOK, AttachSelfResponse{
			EngagementID:  engagementID,
			Doulas:        doulas,
			CanAttachSelf: canAttachSelf(reader, callerID, status, doulas),
		})
	})
}
