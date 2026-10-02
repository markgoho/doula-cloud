package engagementrequest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// doulaRole is the practice_role a person must hold to be named on a
// Request, and employeeType is the Employment type she must work under.
// ADR-0017's amendment on #1515: "one employee Doula at the Practice".
const (
	doulaRole    = "doula"
	employeeType = "employee"
)

// fieldDoula is RequestBody's doulaStaffId json tag, the key every
// Details map about the named Doula is written under (#488).
const fieldDoula = "doulaStaffId"

// Why a person cannot be named as the Doula on a Request. The empty
// string means she can.
type notAttachable string

const (
	// notAtPractice: she holds no Membership at this Practice. At
	// approval this is "her Membership ended" (a Membership ends by a
	// DELETE of the row, staffauth's removal).
	notAtPractice notAttachable = "not_at_practice"
	// notADoula: she is Staff here, and her Membership does not carry
	// the Doula role. Attachment is for Doulas only (CONTEXT.md).
	notADoula notAttachable = "not_a_doula"
	// isContractor: a contractor is attached by her own acceptance of an
	// Offer and by nothing else (CONTEXT.md's Attachment entry).
	isContractor notAttachable = "contractor"
)

// decideAttachable is the whole rule for who a Request may name, in one
// place: she is a Member here, her Membership carries the Doula role,
// and she is an employee. The empty value means she may be named.
//
// Three readers call it and none owns a second copy: the request write
// (requireNameableDoula), approval (approve, which asks again because
// days can pass between the ask and the decision), and the form's own
// list (listRequestDoulas). So the form cannot offer a name the write
// refuses, the same reason visit.decideNameable is one function (#911).
func decideAttachable(hasMembership, isDoula bool, employmentType string) notAttachable {
	switch {
	case !hasMembership:
		return notAtPractice
	case !isDoula:
		return notADoula
	case employmentType != employeeType:
		return isContractor
	}
	return ""
}

// The sentences the request write puts in APIError.Details for the
// doulaStaffId field (#488), held to the GOV.UK rules apierr's
// TestDetailsWording gates. Each says what to do, because "No Doula yet"
// is always open to her.
const (
	MsgDoulaMalformed     = "Select a Doula from the list, or select No Doula yet"
	MsgDoulaNotAtPractice = "Select a Doula who is Staff at this Practice, or select No Doula yet"
	MsgDoulaNotADoula     = "Select a person who holds the Doula role, or select No Doula yet"
	MsgDoulaIsContractor  = "Select an employee Doula, or select No Doula yet. A contractor goes on an Engagement when she accepts an Offer."
	MsgDoulaNotHerself    = "Select yourself, or select No Doula yet. An Owner or an Admin names another Doula."
)

// detailFor is the Details sentence the request write answers reason
// with.
func detailFor(reason notAttachable) string {
	switch reason {
	case notAtPractice:
		return MsgDoulaNotAtPractice
	case notADoula:
		return MsgDoulaNotADoula
	case isContractor:
		fallthrough
	default:
		return MsgDoulaIsContractor
	}
}

// doulaFacts reads what decideAttachable needs about one person, with
// her name where the reader can still see it. A person with no
// Membership here is a normal answer and not an error: staffID is a
// value the caller supplied, or a person who has left since the Request
// named her. RLS hides the staff row of a person with no Membership at
// this Practice (staff_practice_visibility, 00002), so name is empty for
// her.
func doulaFacts(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (hasMembership, isDoula bool, employmentType, name string, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(m.roles), m.employment_type::text, s.name
		   FROM practice_memberships m
		   JOIN staff s ON s.id = m.staff_id
		  WHERE m.practice_id = $2 AND m.staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&isDoula, &employmentType, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, "", "", nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, false, "", "", fmt.Errorf("engagementrequest: read named doula: %w", err)
	}
	return true, isDoula, employmentType, name, nil
}

// requireNameableDoula decides who the new Request names, and refuses
// the ask itself where it may not name her. requested is the body's
// doulaStaffId: nil, or blank, is "No Doula yet", and the Request then
// names nobody.
//
// The order of the checks is the security of it. A person who holds no
// approval authority may name herself and nobody else, and she is told
// so before any lookup of the person she named: ADR-0008 gives a plain
// Doula no read of the roster, and a refusal that changed with the
// person named would be that read by another door.
func requireNameableDoula(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID, callerStaffID string, reader staffauth.Reader, requested *string) (doula sql.NullString, ok bool) {
	if requested == nil || *requested == "" {
		return sql.NullString{}, true
	}
	named, err := uuid.Parse(*requested)
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "doulaStaffId must be a staff id",
			map[string]string{fieldDoula: MsgDoulaMalformed})
		return sql.NullString{}, false
	}
	staffID := named.String()
	if staffID != callerStaffID && !mayApproveDirectly(reader) {
		apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden,
			"only an Owner or an Admin can name another Staff member as the Doula",
			map[string]string{fieldDoula: MsgDoulaNotHerself})
		return sql.NullString{}, false
	}
	hasMembership, isDoula, employmentType, _, err := doulaFacts(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return sql.NullString{}, false
	}
	if reason := decideAttachable(hasMembership, isDoula, employmentType); reason != "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument,
			"the named staff member cannot be the Doula on this request: "+string(reason),
			map[string]string{fieldDoula: detailFor(reason)})
		return sql.NullString{}, false
	}
	return sql.NullString{String: staffID, Valid: true}, true
}

// doulaNotAttachableError is approve's refusal where the Doula the Request
// names can no longer be attached. It carries the sentence the approver
// reads, which names the reason (ADR-0017: "approval is refused with
// that reason and the Request stays pending").
type doulaNotAttachableError struct{ message string }

func (e doulaNotAttachableError) Error() string { return e.message }

// refusalFor is the sentence approval answers reason with. It names the
// Doula where the approver can still read her name, and says what the
// approver can do: she amends nothing (ADR-0017), so the way on is to
// refuse the Request and ask again.
func refusalFor(reason notAttachable, name string) string {
	const wayOn = " Refuse this request, then start the work again and name another Doula or no Doula yet."
	switch reason {
	case notAtPractice:
		return "The Doula this request names is no longer Staff at this Practice." + wayOn
	case notADoula:
		return name + " no longer holds the Doula role at this Practice." + wayOn
	case isContractor:
		fallthrough
	default:
		return name + " is now a contractor, and a contractor goes on an Engagement when she accepts an Offer." + wayOn
	}
}

// requireStillAttachable is approval's own check of the named Doula. It
// runs before approve writes anything.
func requireStillAttachable(ctx context.Context, tx *sql.Tx, practiceID, staffID string) error {
	hasMembership, isDoula, employmentType, name, err := doulaFacts(ctx, tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if reason := decideAttachable(hasMembership, isDoula, employmentType); reason != "" {
		return doulaNotAttachableError{message: refusalFor(reason, name)}
	}
	return nil
}
