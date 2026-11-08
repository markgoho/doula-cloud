package engagement

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// The refusals of AttachDoulaHandler. The Engagement hub prints each as
// it is, so each is a sentence a person can read, and the contractor's
// names the path that is open to her.
const (
	msgAttachDoulaNoEngagement  = "This work was not found at this Practice."
	msgAttachDoulaNotAtPractice = "That person is not on the Staff at this Practice."
	msgAttachDoulaNotADoula     = " does not hold the Doula role, so cannot be put on this work."
	msgAttachDoulaContractor    = " contracts for this Practice, so takes on work by accepting an Offer. Offer this work instead."
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
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, call staffauth.Call) {
		c, ok := beginDoulaChange(w, r, call)
		if !ok {
			return
		}
		if c.status == StatusCompleted {
			apierr.WriteError(w, msgAttachSelfIsCompleted, http.StatusConflict)
			return
		}

		membership, err := attachment.ReadMembership(r.Context(), c.tx, c.practiceID, c.doulaID)
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

		doulas, ok := attachIfAbsent(w, r, c.tx, c.practiceID, c.engagementID, c.doulaID, c.callerID)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}

		apierr.WriteJSON(w, http.StatusOK, AttachSelfResponse{
			EngagementID:  c.engagementID,
			Doulas:        doulas,
			CanAttachSelf: canAttachSelf(c.reader, c.callerID, c.status, doulas),
		})
	}, staffauth.EngagementID(), staffauth.PathID("staff", "staffId"))
}
