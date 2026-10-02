package engagementrequest

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachment"
	"doula-cloud/api/internal/staffauth"
)

// fieldDoula is RequestBody's doulaStaffId json tag, the key every
// Details map about the named Doula is written under (#488).
const fieldDoula = "doulaStaffId"

// The rule for who a Request may name is attachment.Membership's
// WhyNotAttachable, shared with the Engagement's "Put me on this
// Engagement" control (#1598). Three readers in this package call it and
// none owns a second copy: the request write (requireNameableDoula),
// approval (approve, which asks again because days can pass between the
// ask and the decision), and the form's own list (listRequestDoulas). So
// the form cannot offer a name the write refuses.

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
func detailFor(reason attachment.NotAttachable) string {
	switch reason {
	case attachment.NotAtPractice:
		return MsgDoulaNotAtPractice
	case attachment.NotADoula:
		return MsgDoulaNotADoula
	case attachment.IsContractor, attachment.Attachable:
		// Attachable never reaches here: both callers ask only for a
		// reason they were given. It is named so the switch is whole.
		fallthrough
	default:
		return MsgDoulaIsContractor
	}
}

// requireNameableDoula decides who the new Request names, and refuses
// the ask itself where it may not name her. requested is the body's
// doulaStaffId: nil, or blank once trimmed, is "No Doula yet", and the
// Request then names nobody.
//
// The order of the checks is the security of it. A person who holds no
// approval authority may name herself and nobody else, and she is told
// so before any lookup of the person she named: ADR-0008 gives a plain
// Doula no read of the roster, and a refusal that changed with the
// person named would be that read by another door.
func requireNameableDoula(w http.ResponseWriter, r *http.Request, tx *sql.Tx, practiceID, callerStaffID string, reader staffauth.Reader, requested *string) (doula sql.NullString, ok bool) {
	if requested == nil || strings.TrimSpace(*requested) == "" {
		return sql.NullString{}, true
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*requested))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "doulaStaffId must be a staff id",
			map[string]string{fieldDoula: MsgDoulaMalformed})
		return sql.NullString{}, false
	}
	staffID := parsed.String()
	if staffID != callerStaffID && !mayApproveDirectly(reader) {
		apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden,
			"only an Owner or an Admin can name another Staff member as the Doula",
			map[string]string{fieldDoula: MsgDoulaNotHerself})
		return sql.NullString{}, false
	}
	named, err := attachment.ReadMembership(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return sql.NullString{}, false
	}
	if reason := named.WhyNotAttachable(); reason != attachment.Attachable {
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
func refusalFor(reason attachment.NotAttachable, name string) string {
	const wayOn = " Refuse this request, then start the work again and name another Doula or no Doula yet."
	switch reason {
	case attachment.NotAtPractice:
		return "The Doula this request names is no longer Staff at this Practice." + wayOn
	case attachment.NotADoula:
		return name + " no longer holds the Doula role at this Practice." + wayOn
	case attachment.IsContractor, attachment.Attachable:
		// Attachable never reaches here: both callers ask only for a
		// reason they were given. It is named so the switch is whole.
		fallthrough
	default:
		return name + " is now a contractor, and a contractor goes on an Engagement when she accepts an Offer." + wayOn
	}
}

// requireStillAttachable is approval's own check of the named Doula. It
// runs before approve writes anything.
func requireStillAttachable(ctx context.Context, tx *sql.Tx, practiceID, staffID string) error {
	named, err := attachment.ReadMembership(ctx, tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("engagementrequest: read named doula: %w", err)
	}
	if reason := named.WhyNotAttachable(); reason != attachment.Attachable {
		return doulaNotAttachableError{message: refusalFor(reason, named.Name)}
	}
	return nil
}
