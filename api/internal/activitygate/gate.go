// Package activitygate is the subject-kind-agnostic read gate #485 asks
// for: given a subject kind, a reader, and (for a row) an action, it
// decides visibility, the way engagement.ListActivityHandler decided it
// inline before this package existed. It lives beside activity rather
// than inside it because staffauth already imports activity (to record
// Membership/session activity rows), and this gate needs staffauth's
// Reader and its per-kind access checks (CanAccessEngagement,
// CanAccessClient) -- importing staffauth from inside activity would
// cycle.
//
// A subject kind reaches through this gate only once it is registered in
// registry below. An unregistered kind is refused, never silently
// allowed -- #486's practice-wide feed (the reason this package exists)
// must not be able to widen a subject kind's visibility by simply adding
// a reader to it without also stating the Rule ADR-0008 requires.
package activitygate

import (
	"context"
	"database/sql"
	"slices"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffauth"
)

// Rule is what one subject kind registers with the gate.
type Rule struct {
	// CanAccessSubject reports whether reader may see any activity row
	// for one instance of this subject kind at all.
	CanAccessSubject func(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, subjectID string) (bool, error)

	// RestrictedActions is the set of this subject kind's action strings
	// ADR-0008's money tier keeps off anyone but Owner/Admin. Nil means
	// this subject kind restricts nothing.
	RestrictedActions []string
}

// bypassesRestriction is ADR-0008's one money-tier role rule, applied the
// same way to every subject kind that has any RestrictedActions: an
// Owner or Admin sees them, nobody else does, regardless of employment
// type. "This ticket moves that rule, it does not change it" (#485).
func bypassesRestriction(reader staffauth.Reader) bool {
	return reader.Has("owner") || reader.Has("admin")
}

// subjectClient is the subject_kind literal client/events.go's
// recordEvent writes -- that package has no exported constant for it the
// way activity.SubjectEngagement is one.
const subjectClient = "client"

// registry is the one place a subject kind's Rule is stated -- AC6's
// "adding a new subject kind requires registering its rule in one place."
// A subject kind absent here is refused by CanAccessSubject/CanSeeAction,
// never silently allowed.
//
// client_field_template is deliberately absent: clientfieldtemplate.Save
// writes it (template.go), but nothing reads it back today -- no handler
// queries activity WHERE subject_kind = 'client_field_template'. A reader
// added later must register a Rule here before this gate will ever
// return true for it (#485's AC5).
var registry = map[string]Rule{
	activity.SubjectEngagement: {
		CanAccessSubject: func(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, subjectID string) (bool, error) {
			return reader.CanAccessEngagement(ctx, tx, subjectID)
		},
		RestrictedActions: engagementRestrictedActions(),
	},
	subjectClient: {
		// client.DetailHandler already gates its whole page -- record,
		// resolved fields, Engagements and this same merged history --
		// behind reader.CanAccessClient before it reads a row
		// (client/detail.go). Registering the identical predicate here
		// does not add a second DB check to that handler; it lets a
		// cross-subject reader (#486), which has not already made that
		// check, apply it per row. No action on the 'client' subject kind
		// (created/updated/erased, client/events.go) overlaps ADR-0008's
		// money tier, so RestrictedActions stays nil.
		CanAccessSubject: func(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, subjectID string) (bool, error) {
			return reader.CanAccessClient(ctx, tx, subjectID)
		},
	},
}

// engagementRestrictedActions adapts activity.MoneyActions() (the write
// side's own source of truth) to the []string shape Rule.RestrictedActions
// and RestrictedActions() below need, so the write side's action names and
// this gate's exclusion list can never drift apart.
func engagementRestrictedActions() []string {
	actions := activity.MoneyActions()
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}

// CanAccessSubject reports whether reader may see any activity row for
// subjectKind/subjectID. A subjectKind with no registered Rule is
// refused.
func CanAccessSubject(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, subjectKind, subjectID string) (bool, error) {
	rule, ok := registry[subjectKind]
	if !ok {
		return false, nil
	}
	return rule.CanAccessSubject(ctx, tx, reader, subjectID)
}

// CanSeeAction reports whether reader may see a row recording action for
// subjectKind, once CanAccessSubject has already allowed the subject
// itself -- the row-level decision a cross-subject reader spanning many
// subject kinds in one feed applies per row, rather than each kind
// building its own SQL exclusion (see RestrictedActions for that path,
// which engagement.ListActivityHandler keeps using for pagination
// correctness). A subjectKind with no registered Rule is refused.
func CanSeeAction(reader staffauth.Reader, subjectKind, action string) bool {
	rule, ok := registry[subjectKind]
	if !ok {
		return false
	}
	if !slices.Contains(rule.RestrictedActions, action) {
		return true
	}
	return bypassesRestriction(reader)
}

// RestrictedActions returns the action strings ADR-0008's money tier
// excludes for subjectKind, for a caller building its own SQL exclusion
// clause (see engagement.ListActivityHandler). Nil for a subject kind
// with no restricted actions, or none registered at all -- a caller must
// check CanAccessSubject first; RestrictedActions alone never says
// whether subjectKind is registered.
func RestrictedActions(subjectKind string) []string {
	return registry[subjectKind].RestrictedActions
}

// Bypasses reports whether reader sees every subject kind's restricted
// actions regardless -- ADR-0008's Owner/Admin money tier. Exposed
// alongside RestrictedActions for a caller (engagement.ListActivityHandler)
// building a SQL boolean parameter rather than calling CanSeeAction per
// row.
func Bypasses(reader staffauth.Reader) bool {
	return bypassesRestriction(reader)
}
