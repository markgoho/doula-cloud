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

// bypassesRestriction is ADR-0008's money-tier rule, as amended by #282,
// applied the same way to every subject kind that has any
// RestrictedActions: an Owner, an Admin, and an employed Doula see them;
// only a plain contractor Doula does not. Employment type is the
// boundary and the only one -- an employee is inside the business and
// reads what the Practice charges; a contractor reads her own agreed fee
// elsewhere, never this.
func bypassesRestriction(reader staffauth.Reader) bool {
	return !reader.IsAmbientContractor()
}

// registry is the one place a subject kind's Rule is stated -- AC6's
// "adding a new subject kind requires registering its rule in one place."
// A subject kind absent here is refused by CanAccessSubject/CanSeeAction,
// never silently allowed.
//
// A kind deliberately left out belongs in unregistered below, with the
// reason -- never merely absent. registry_test.go's own class guard
// reads the write side's subject kinds and fails unless each one appears
// in exactly one of the two maps, which is what stops the next event
// family being dropped from #486's feed as quietly as membership was
// (#1148).
var registry = map[string]Rule{
	activity.SubjectEngagement: {
		CanAccessSubject: func(ctx context.Context, tx *sql.Tx, reader staffauth.Reader, subjectID string) (bool, error) {
			return reader.CanAccessEngagement(ctx, tx, subjectID)
		},
		RestrictedActions: engagementRestrictedActions(),
	},
	activity.SubjectClient: {
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
	activity.SubjectMembership: {
		// A Membership row's subject_id is a staff_id, and the row says
		// how that person's standing at this Practice came to be what it
		// is: joined, roles_changed, employment_type_changed, removed,
		// and the sessions_ended an Owner performs against the same
		// relationship. Who may read it is who may read the roster it
		// describes -- staffauth.ListStaffHandler and
		// ListMembershipHistoryHandler are both mounted OwnerAndAdmin,
		// ADR-0008's read table -- so the same predicate is stated here
		// rather than left inline in the feed (#1148).
		//
		// Not reader.CanAccessClient's shape: there is no per-subject
		// database question to ask, because the roster is not something a
		// Doula reaches part of. An employed Doula holds ambient reach
		// over every Engagement and Client at the Practice and still does
		// not read the roster's history, so the check is the reader's own
		// role and nothing about subjectID. It takes tx anyway because
		// Rule's signature is one shape for every kind; a kind that needs
		// no query simply asks none. It ignores subjectID for the same
		// reason, so this Rule alone never says a staff id belongs to the
		// current Practice -- every reader that calls it scopes its own
		// query by practice_id, and activity's RLS policy scopes it
		// again, which is where that question is answered.
		//
		// RestrictedActions stays nil: no Membership action carries what
		// the Practice charges, which is the whole of what ADR-0008's
		// money tier holds back. Employment type is on this feed and is
		// meant to be -- an Owner and an Admin are the only readers who
		// reach it at all, and both sit inside that tier already.
		CanAccessSubject: func(_ context.Context, _ *sql.Tx, reader staffauth.Reader, _ string) (bool, error) {
			return reader.IsOwnerOrAdmin(), nil
		},
	},
}

// unregistered names every subject kind a write site records that
// registry deliberately states no Rule for, mapped to the reason. It is
// the other half of the class guard registry's own comment describes:
// absent-with-a-reason is a decision, plain absence is the bug #1148 was.
//
// A kind here is refused by CanAccessSubject/CanSeeAction exactly as any
// unknown string is -- this map changes no behavior at all, and is read
// only by registry_test.go. Registering a Rule is what admits a kind to
// #486's feed; moving it out of here is the same edit.
//
// A reason here is prose for whoever reads this file next, never a
// string any response carries -- but apierr's own TestDetailsWording
// reads every map[string]string literal in the module as if it were an
// APIError.Details, so a reason written with one of GOV.UK's forbidden
// error words ("please", "valid", "invalid", "required") fails that gate
// from here. Write around the word rather than widening the gate: it is
// checking the right thing and cannot tell these two maps apart.
//
// It lives beside registry rather than in that test file, though it is
// the test's only reader, because the two maps are one statement: this is
// the one place a subject kind's disposition toward the feed is written
// down, and splitting the "yes, on these terms" half from the "no, for
// this reason" half would leave a reader of registry alone unable to tell
// a considered omission from the bug this ticket fixed.
var unregistered = map[string]string{
	activity.SubjectPractice: "a Practice-scoped row (the switch that turns MFA on for all staff, a whole-Practice export) names the Practice itself rather than a record inside it, so who may read one is a separate decision from the roster's and the Client's -- tracked on #1255, not settled here.",
	"client_field_template":  "written by clientfieldtemplate.Save (which spells the kind as a literal, having no constant) and read back by nothing -- no handler queries activity WHERE subject_kind = 'client_field_template'. A reader added later must register a Rule before this gate will ever return true for it (#485's AC5).",
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
// actions regardless -- ADR-0008's money tier, as amended by #282, which
// admits an employed Doula alongside an Owner and an Admin. Exposed
// alongside RestrictedActions for a caller (engagement.ListActivityHandler)
// building a SQL boolean parameter rather than calling CanSeeAction per
// row.
func Bypasses(reader staffauth.Reader) bool {
	return bypassesRestriction(reader)
}
