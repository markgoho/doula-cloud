// Package attachment holds the one rule for who may be put on an
// Engagement by a decision that is not her own acceptance of an Offer,
// and the one write that puts her there.
//
// It is a package of its own because two packages need it and neither
// may own it: engagementrequest names a Doula when an Engagement starts
// (#1596), and engagement lets a Doula put herself on one (#1598).
// ADR-0008's amendments on #1515 name them as the third and the fourth
// writer of a granted Attachment. One rule and one write, so the two
// cannot come to disagree about who is attachable or about what the
// Engagement's ledger says afterward. visit reads the rule too (#1625),
// for the Visit that names a Doula, and keeps its own write.
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

// doulaRole is the practice_role a person must hold to be attached.
// employeeType is the Employment type that needs no more than that role.
// ownerRole and adminRole are the two roles that make a person a part of
// the Practice whatever her Employment type is: ADR-0008's amendment on
// #1625 says "attached only by her own acceptance of an Offer" is the
// rule for a contractor who holds neither.
const (
	doulaRole    = "doula"
	ownerRole    = "owner"
	adminRole    = "admin"
	employeeType = "employee"
)

// NotAttachable is why a person cannot be attached without an Offer.
type NotAttachable string

const (
	// NotAtPractice is the person who holds no Membership at this Practice. At
	// approval of a Request this is "her Membership ended" (a Membership
	// ends by a DELETE of the row, staffauth's removal).
	NotAtPractice NotAttachable = "not_at_practice"
	// NotADoula is the person who is Staff here with a Membership that does not carry
	// the Doula role. Attachment is for Doulas only (CONTEXT.md).
	NotADoula NotAttachable = "not_a_doula"
	// IsContractor is the contractor who holds neither the Owner role nor
	// the Admin role, who is attached by her own acceptance of an Offer and
	// by nothing else (CONTEXT.md's Attachment entry). It is the population
	// staffauth.Reader.IsAmbientContractor confines for reach. A contractor
	// who holds Owner or Admin is not this person, and never gets this
	// answer (#1625).
	IsContractor NotAttachable = "contractor"
)

// Attachable is the empty NotAttachable: nothing stands in the way.
const Attachable NotAttachable = ""

// Membership is what the rule reads about one person at one Practice:
// whether she is a Member here at all, whether that Membership carries
// the Doula role, whether it carries the Owner role or the Admin role,
// and the Employment type she works under. Name is hers where the reader
// can see it, for a sentence that refuses her by name.
type Membership struct {
	Exists         bool
	IsDoula        bool
	IsOwnerOrAdmin bool
	EmploymentType string
	Name           string
}

// WhyNotAttachable is the whole rule, in one place: she is a Member
// here, her Membership carries the Doula role, and she is not a
// contractor who holds neither the Owner role nor the Admin role. It
// answers Attachable where she may be attached.
//
// The contractor rule exists because "nobody can put an outsider on a
// Client's birth without her agreement" (CONTEXT.md's Attachment entry).
// A person who holds Owner or Admin is not an outsider: she starts
// Engagements, approves Requests, and names Doulas. So the rule confines
// the same contractor that each reach rule confines
// (staffauth.Reader.IsAmbientContractor), and an Owner or an Admin who is
// a contractor Doula is attachable (ADR-0008's amendment on #1625).
//
// Each writer of a granted Attachment that needs no Offer calls it, and
// none owns a second copy: the request write, approval (which asks again
// because days can pass between the ask and the decision), the Start
// work form's own list, the Engagement's "Put me on this Engagement"
// control with the write behind it, and the Visit that names a Doula
// (visit.decideNameable and visit.resolveAssignee). So no screen can
// offer what its write refuses, and no two writers can disagree about
// who is attachable.
func (m Membership) WhyNotAttachable() NotAttachable {
	switch {
	case !m.Exists:
		return NotAtPractice
	case !m.IsDoula:
		return NotADoula
	case m.EmploymentType != employeeType && !m.IsOwnerOrAdmin:
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
		IsOwnerOrAdmin: reader.IsOwnerOrAdmin(),
		EmploymentType: reader.EmploymentType(),
	}
}

// OwnerOrAdminSQL is Membership.IsOwnerOrAdmin as a SQL expression over a
// practice_memberships row aliased m. ReadMembership and the two roster
// reads that build a Membership for each row (the Start work form's list
// and the Visit pickers') share it, so the fact has one spelling. A
// const of literals, with no parameter, so each query built from it stays
// a constant expression.
const OwnerOrAdminSQL = `('` + ownerRole + `' = ANY(m.roles) OR '` + adminRole + `' = ANY(m.roles))`

// ReadMembership reads what the rule needs about one person who is not
// the caller. A person with no Membership here is a normal answer and
// not an error: staffID is a value a caller supplied, or a person who
// has left since a Request named her, and she comes back as the zero
// Membership, which holds no name.
func ReadMembership(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (Membership, error) {
	m := Membership{Exists: true}
	err := tx.QueryRowContext(ctx,
		`SELECT $1 = ANY(m.roles), `+OwnerOrAdminSQL+`, m.employment_type::text, s.name
		   FROM practice_memberships m
		   JOIN staff s ON s.id = m.staff_id
		  WHERE m.practice_id = $2 AND m.staff_id = $3`,
		doulaRole, practiceID, staffID,
	).Scan(&m.IsDoula, &m.IsOwnerOrAdmin, &m.EmploymentType, &m.Name)
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
