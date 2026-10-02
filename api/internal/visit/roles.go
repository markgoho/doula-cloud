package visit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// doulaRole is the practice_role enum member (00002_practice_staff_tenancy.sql)
// this package cares about. It is the role that puts a person on a birth:
// a Visit can only ever be assigned to a holder of it, and a Staff member
// logging a Visit for *herself* must hold it. Naming somebody else is a
// different act with a different rule -- see resolveAssignee below.
const doulaRole = "doula"

// visitWriteContext is the per-request state every Visit write needs:
// the request-scoped tx staffauth.Middleware opened, the Practice it is
// scoped to, the caller's own Reader, and her staff id. Resolved once,
// rather than each handler reaching into context three times.
type visitWriteContext struct {
	tx         *sql.Tx
	practiceID string
	reader     staffauth.Reader
	staffID    string
}

// requireVisitWrite resolves that state, writing the error response
// itself if any of it is missing. It asserts no role of its own: who may
// write a Visit is decided per act, not per endpoint -- reaching the
// Engagement at all is already gated by staffauth.AttachingWrite, and
// naming a person is gated by resolveAssignee below.
func requireVisitWrite(w http.ResponseWriter, r *http.Request) (visitWriteContext, bool) {
	tx, practiceID, ok := staffauth.RequireTx(w, r)
	// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
	if !ok {
		return visitWriteContext{}, false
	}
	reader, has := staffauth.ReaderFrom(r.Context())
	if !has {
		// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return visitWriteContext{}, false
	}
	staffID, _ := staffauth.StaffID(r.Context())
	return visitWriteContext{tx: tx, practiceID: practiceID, reader: reader, staffID: staffID}, true
}

// resolveAssignee decides who a Visit is for, whether this caller may say
// so, and whether that person is to be granted an attachment -- writing
// the refusal itself when she may not (#268, #914). It is the one place
// the assignment is decided, so the create and the reassign path cannot
// drift apart on who may be named, who is eligible, or who gets attached.
//
// Two different acts hide behind one field. Logging a Visit for yourself
// is the Doula's own act, and needs the Doula role -- an Owner or Admin
// who is not also a Doula has no self to fall back on, so an absent
// assignee is a refusal for her rather than a silent self-assignment.
// Naming somebody else is scheduling, which CONTEXT.md's Attachment entry
// and ADR-0006 both put with the Owner and the Admin ("booking a Visit
// means picking a Doula -- an Admin who cannot read the roster cannot do
// the job the glossary gives her"), and never with a plain Doula: she
// cannot read the Staff roster, so she has no list to pick a colleague
// from in the first place.
//
// requested == nil means "me" -- an absent assignee on create. An
// explicit id equal to the caller's own takes the self path too, so an
// Admin who is also a Doula naming herself is not held to the stricter
// rule for no reason.
//
// grants is whether the Visit is to write a granted attachment for the
// person it lands on. It is the answer of the one rule for who is
// attached without an Offer (attachment.Membership.WhyNotAttachable),
// asked of that person and never of the caller: for a colleague,
// requireEligibleAssignee reads her Membership; for the caller herself,
// her own Reader already carries it and no second query is needed. An
// employee Doula is granted one, and so is a Doula who holds Owner or
// Admin whatever her Employment type is (#1625). A contractor who holds
// neither is not -- see grantAssignee.
func resolveAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID string, requested *string) (staffID string, grants, ok bool) {
	if requested == nil || *requested == c.staffID {
		if !c.reader.Has(doulaRole) {
			apierr.WriteError(w, "only a Staff member with the Doula role can log a Visit for herself -- name the colleague this Visit is for instead", http.StatusForbidden)
			return "", false, false
		}
		return c.staffID, attachment.OfReader(c.reader).WhyNotAttachable() == attachment.Attachable, true
	}
	if !c.reader.IsOwnerOrAdmin() {
		apierr.WriteError(w, "only an Owner or an Admin can assign a Visit to another Staff member", http.StatusForbidden)
		return "", false, false
	}
	grants, eligible := requireEligibleAssignee(w, r, c, engagementID, *requested)
	if !eligible {
		return "", false, false
	}
	return *requested, grants, true
}

// requireEligibleAssignee runs the three rules a *named* Staff member has
// to pass before a Visit can be put on her, writing the refusal itself:
// she is a Staff member at this Practice, her Membership carries the
// Doula role, and -- if she is a contractor who holds neither Owner nor
// Admin -- she already holds the open granted attachment her own
// acceptance of an Offer opened. Each refusal carries its own message, so
// the caller learns which rule she failed.
//
// Reached only through resolveAssignee, so create (#268) and reassign
// share it by construction: the two are the same act at two moments, and
// a second copy of these rules is exactly how the create path would
// drift more permissive than the reassign path.
//
// Reports whether the named Staff member is to be granted an attachment,
// which is the attachment rule's own answer for her. This package holds
// no comparison of an Employment type: attachment.Membership reads it,
// so the Visit, the Engagement Request, and "Put me on this Engagement"
// cannot disagree about who is attached without an Offer (#1625).
//
// The three rules themselves live in decideNameable (nameable.go), which
// AssigneesHandler's read calls too -- so the picker on the screen and
// the refusal here cannot disagree (#911). What stayed here is the
// gathering of the facts for one named person and the writing of the
// refusal: which rule failed, and the sentence it has always answered
// with, are both unchanged.
func requireEligibleAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID, staffID string) (grants, ok bool) {
	named, err := attachment.ReadMembership(r.Context(), c.tx, c.practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false, false
	}
	why := named.WhyNotAttachable()
	// A contractor who holds neither Owner nor Admin is put on a birth by
	// her own acceptance of an Offer and by nothing else (CONTEXT.md's
	// Attachment entry), so handing her a Visit is refused unless she
	// already holds the attachment that says she agreed. Read only when
	// that is the one thing in her way, so a Staff member who is not a
	// Doula here still costs one query rather than two.
	attached := false
	if why == attachment.IsContractor {
		attached, err = hasGrantedAttachment(r.Context(), c.tx, engagementID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return false, false
		}
	}
	if reason := decideNameable(named, attached); reason != "" {
		apierr.WriteError(w, refusalMessage(reason), http.StatusBadRequest)
		return false, false
	}
	return why == attachment.Attachable, true
}

// hasGrantedAttachment reports whether staffID holds an open, granted
// attachment to engagementID -- the record that she agreed to be on this
// birth, which is the only way a contractor who holds neither Owner nor
// Admin gets onto one.
func hasGrantedAttachment(ctx context.Context, tx *sql.Tx, engagementID, staffID string) (bool, error) {
	var attached bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM engagement_attachments
			WHERE engagement_id = $1 AND staff_id = $2 AND `+grantedAttachmentPredicate+`
		)`,
		engagementID, staffID,
	).Scan(&attached)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("visit: check granted attachment: %w", err)
	}
	return attached, nil
}

// grantAssignee writes the granted attachment the Visit's assignee gets,
// when she is one to get it. Naming a Doula on a Visit puts her on this
// birth, which is a granted attachment, not the accrual
// staffauth.AttachingWrite's seam mints -- ADR-0008 names Visit-create
// and Visit-reassign as the two places granted is written explicitly. No
// fee rides it: a fee is only ever copied from an Offer. attached_by is
// the acting person, who is the caller whether she named herself or a
// colleague.
//
// Only for a person the attachment rule admits, though: grants is
// resolveAssignee's answer. CONTEXT.md's Attachment entry gives a
// contractor who holds neither Owner nor Admin exactly one way onto a
// birth -- her own acceptance of an Offer -- so granting here would let
// her hand herself the reach an Offer exists to ask for. Logging her own
// Visit gets her the seam's accrued record instead, which is a record of
// work and never a key; being *named* by an Owner or Admin needs no
// grant at all, because resolveAssignee has already proved she holds the
// one her acceptance opened.
//
// Both handlers call this once, after their own row write and their own
// activity entry (#914), so the reassign path's rows == 0 404 still
// returns before any attachment is written. It writes the error response
// itself and reports that it did.
func grantAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID, staffID string, grants bool) bool {
	if !grants {
		return true
	}
	if err := staffauth.Grant(r.Context(), c.tx, engagementID, staffID, c.staffID, nil, nil); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}
