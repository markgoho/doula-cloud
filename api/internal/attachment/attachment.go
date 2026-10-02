// Package attachment holds the one rule for who may be put on an
// Engagement by a decision that is not her own acceptance of an Offer,
// and the one write that puts her there.
//
// It is a package of its own because two packages need it and neither
// may own it: engagementrequest names a Doula when an Engagement starts
// (#1596), and engagement lets an employee Doula put herself on one
// (#1598). ADR-0008's amendments on #1515 name them as the third and the
// fourth writer of a granted Attachment. One rule and one write, so the
// two cannot come to disagree about who is attachable or about what the
// Engagement's ledger says afterward.
package attachment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
)

// doulaRole is the practice_role a person must hold to be attached, and
// employeeType is the Employment type she must work under. ADR-0017's
// amendment on #1515: "one employee Doula at the Practice".
const (
	doulaRole    = "doula"
	employeeType = "employee"
)

// NotAttachable is why a person cannot be attached without an Offer.
type NotAttachable string

const (
	// NotAtPractice: she holds no Membership at this Practice. At
	// approval of a Request this is "her Membership ended" (a Membership
	// ends by a DELETE of the row, staffauth's removal).
	NotAtPractice NotAttachable = "not_at_practice"
	// NotADoula: she is Staff here, and her Membership does not carry
	// the Doula role. Attachment is for Doulas only (CONTEXT.md).
	NotADoula NotAttachable = "not_a_doula"
	// IsContractor: a contractor is attached by her own acceptance of an
	// Offer and by nothing else (CONTEXT.md's Attachment entry).
	IsContractor NotAttachable = "contractor"
)

// Attachable is the empty NotAttachable: nothing stands in the way.
const Attachable NotAttachable = ""

// Membership is what the rule reads about one person at one Practice:
// whether she is a Member here at all, whether that Membership carries
// the Doula role, and the Employment type she works under. Name is hers
// where the reader can see it, for a sentence that refuses her by name.
type Membership struct {
	Exists         bool
	IsDoula        bool
	EmploymentType string
	Name           string
}

// WhyNotAttachable is the whole rule, in one place: she is a Member
// here, her Membership carries the Doula role, and she is an employee.
// It answers Attachable where she may be attached.
//
// Four readers call it and none owns a second copy: the request write,
// approval (which asks again because days can pass between the ask and
// the decision), the Start work form's own list, and the Engagement's
// "Put me on this Engagement" control with the write behind it. So no
// screen can offer what its write refuses, the same reason
// visit.decideNameable is one function (#911).
func (m Membership) WhyNotAttachable() NotAttachable {
	switch {
	case !m.Exists:
		return NotAtPractice
	case !m.IsDoula:
		return NotADoula
	case m.EmploymentType != employeeType:
		return IsContractor
	}
	return Attachable
}

// OfReader is the caller's own Membership, from the Reader
// staffauth.Middleware already resolved for this request. It makes no
// query: the Reader is the caller's practice_memberships row. Name is
// empty, because the Reader does not carry it.
func OfReader(reader staffauth.Reader) Membership {
	return Membership{
		Exists:         true,
		IsDoula:        reader.Has(doulaRole),
		EmploymentType: reader.EmploymentType(),
	}
}

// ReadMembership reads what the rule needs about one person who is not
// the caller. A person with no Membership here is a normal answer and
// not an error: staffID is a value a caller supplied, or a person who
// has left since a Request named her, and she comes back as the zero
// Membership, which holds no name.
func ReadMembership(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (Membership, error) {
	m := Membership{Exists: true}
	err := tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(m.roles), m.employment_type::text, s.name
		   FROM practice_memberships m
		   JOIN staff s ON s.id = m.staff_id
		  WHERE m.practice_id = $2 AND m.staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&m.IsDoula, &m.EmploymentType, &m.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Membership{}, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return Membership{}, fmt.Errorf("attachment: read membership: %w", err)
	}
	return m, nil
}

// Grant puts doulaStaffID on engagementID: a granted Attachment through
// staffauth.Grant, and the activity row that answers "who attached the
// Doula, and when" on the Engagement's own ledger. No fee rides it,
// because a fee is only ever copied from an Offer.
//
// attachedBy is the person whose decision it is: the approver of a
// Request that names the Doula, or the Doula herself where she pressed
// "Put me on this Engagement". She is attached_by on the row and the
// actor on the activity entry, the diff names the Doula, and the row's
// created_at is the instant.
//
// The caller has already asked WhyNotAttachable. Grant does not ask
// again, so that each caller refuses in its own words.
func Grant(ctx context.Context, tx *sql.Tx, practiceID, engagementID, doulaStaffID, attachedBy string) error {
	if err := staffauth.Grant(ctx, tx, engagementID, doulaStaffID, attachedBy, nil, nil); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachment: grant: %w", err)
	}
	diff, err := json.Marshal(map[string]string{activity.DiffKeyAttachedStaffID: doulaStaffID})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("attachment: marshal doula attached diff: %w", err)
	}
	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(activity.ActionDoulaAttached),
		Diff:        diff,
		Actor:       activity.StaffActor(attachedBy),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("attachment: record doula attached: %w", err)
	}
	return nil
}
