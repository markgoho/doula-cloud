package visit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
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
// isEmployee turns on the employment type of the person the Visit lands
// on, never the caller's: for a colleague, requireEligibleAssignee reads
// it off her Membership; for the caller herself, her own Reader already
// carries it and no second query is needed. Only an employee is granted
// an attachment -- see grantAssignee.
func resolveAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID string, requested *string) (staffID string, isEmployee, ok bool) {
	if requested == nil || *requested == c.staffID {
		if !c.reader.Has(doulaRole) {
			apierr.WriteError(w, "only a Staff member with the Doula role can log a Visit for herself -- name the colleague this Visit is for instead", http.StatusForbidden)
			return "", false, false
		}
		return c.staffID, !c.reader.IsContractor(), true
	}
	if !c.reader.IsOwnerOrAdmin() {
		apierr.WriteError(w, "only an Owner or an Admin can assign a Visit to another Staff member", http.StatusForbidden)
		return "", false, false
	}
	isEmployee, eligible := requireEligibleAssignee(w, r, c, engagementID, *requested)
	if !eligible {
		return "", false, false
	}
	return *requested, isEmployee, true
}

// requireEligibleAssignee runs the three rules a *named* Staff member has
// to pass before a Visit can be put on her, writing the refusal itself:
// she is a Staff member at this Practice, her Membership carries the
// Doula role, and -- if she is a contractor -- she already holds the open
// granted attachment her own acceptance of an Offer opened. Each refusal
// carries its own message, so the caller learns which rule she failed.
//
// Reached only through resolveAssignee, so create (#268) and reassign
// share it by construction: the two are the same act at two moments, and
// a second copy of these rules is exactly how the create path would
// drift more permissive than the reassign path.
//
// Reports whether the named Staff member is an employee, which is what
// decides whether she goes on to be granted an attachment. The string
// comparison against employeeType lives here and nowhere else (#914).
//
// The three rules themselves live in decideNameable (nameable.go), which
// AssigneesHandler's read calls too -- so the picker on the screen and
// the refusal here cannot disagree (#911). What stayed here is the
// gathering of the facts for one named person and the writing of the
// refusal: which rule failed, and the sentence it has always answered
// with, are both unchanged.
func requireEligibleAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID, staffID string) (isEmployee, ok bool) {
	hasMembership, isDoula, employmentType, err := doulaMembership(r.Context(), c.tx, c.practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false, false
	}
	isEmployee = employmentType == employeeType
	// A contractor is put on a birth by her own acceptance of an Offer
	// and by nothing else (CONTEXT.md's Attachment entry), so handing
	// her a Visit is refused unless she already holds the attachment
	// that says she agreed. Read only when the earlier two rules have
	// already passed and she is a contractor, so a Staff member who is
	// not a Doula here still costs one query rather than two.
	attached := false
	if hasMembership && isDoula && !isEmployee {
		attached, err = hasGrantedAttachment(r.Context(), c.tx, engagementID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return false, false
		}
	}
	if reason := decideNameable(hasMembership, isDoula, employmentType, attached); reason != "" {
		apierr.WriteError(w, refusalMessage(reason), http.StatusBadRequest)
		return false, false
	}
	return isEmployee, true
}

// doulaMembership reports whether staffID holds a practice_memberships row
// at practiceID, and if so whether that membership includes the Doula
// role and which employment type it carries. Unlike staffauth.Roles
// (built for the caller, whom staffauth.Middleware already guarantees a
// membership for), this treats "no membership" as a normal, expected
// outcome rather than an error -- staffID here is an arbitrary
// assignment target the caller supplied, which may not be a Staff
// member at this Practice at all.
func doulaMembership(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (hasMembership, isDoula bool, employmentType string, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(roles), employment_type::text
		   FROM practice_memberships WHERE practice_id = $2 AND staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&isDoula, &employmentType)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "", nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, false, "", fmt.Errorf("visit: check doula membership: %w", err)
	}
	return true, isDoula, employmentType, nil
}

// employeeType is the employment_type a Practice may put on a birth
// directly. CONTEXT.md's Attachment entry draws the line: "An Admin may
// attach an employee directly -- naming her on a Visit is granted, not
// accrued, because she has done nothing... A contractor can only be
// attached by her own acceptance of an Offer: nobody can put an outsider
// on a Client's birth without her agreement" -- so a direct grant is for
// an employee and nobody else.
const employeeType = "employee"

// hasGrantedAttachment reports whether staffID holds an open, granted
// attachment to engagementID -- the record that she agreed to be on this
// birth, which is the only way a contractor gets onto one.
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
// Only for an employee, though. CONTEXT.md's Attachment entry gives a
// contractor exactly one way onto a birth -- her own acceptance of an
// Offer -- so granting here would let her hand herself the reach an
// Offer exists to ask for. Logging her own Visit gets her the seam's
// accrued record instead, which is a record of work and never a key;
// being *named* by an Owner or Admin needs no grant at all, because
// resolveAssignee has already proved she holds the one her acceptance
// opened.
//
// Both handlers call this once, after their own row write and their own
// activity entry (#914), so the reassign path's rows == 0 404 still
// returns before any attachment is written. It writes the error response
// itself and reports that it did.
func grantAssignee(w http.ResponseWriter, r *http.Request, c visitWriteContext, engagementID, staffID string, isEmployee bool) bool {
	if !isEmployee {
		return true
	}
	if err := staffauth.Grant(r.Context(), c.tx, engagementID, staffID, c.staffID, nil, nil); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	return true
}
