package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MsgWorkStateRequired is what a caller who omitted a work state, or sent
// something that is not a US state, gets back. One literal, shared by
// signup and invitation acceptance, because a person meets whichever of
// the two her Practice sends her to and should read the same sentence.
const MsgWorkStateRequired = "workState is required, and must be a two-letter US state abbreviation"

// usStates is the 50 states and the District of Columbia as USPS
// two-letter abbreviations -- the same set 00043's staff_work_state_usps
// CHECK constraint holds. Duplicated deliberately: the constraint is what
// makes a bad value impossible, and this is what makes a bad value a 400
// rather than a 500.
var usStates = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true,
	"CT": true, "DE": true, "DC": true, "FL": true, "GA": true, "HI": true,
	"ID": true, "IL": true, "IN": true, "IA": true, "KS": true, "KY": true,
	"LA": true, "ME": true, "MD": true, "MA": true, "MI": true, "MN": true,
	"MS": true, "MO": true, "MT": true, "NE": true, "NV": true, "NH": true,
	"NJ": true, "NM": true, "NY": true, "NC": true, "ND": true, "OH": true,
	"OK": true, "OR": true, "PA": true, "RI": true, "SC": true, "SD": true,
	"TN": true, "TX": true, "UT": true, "VT": true, "VA": true, "WA": true,
	"WV": true, "WI": true, "WY": true,
}

// NormalizeWorkState trims and upper-cases what the caller sent and
// reports whether it names a US state. It is the server-side half of the
// requirement: the form offers a fixed <select>, but a form is not an
// enforcement boundary, so nothing reaches the database on the strength
// of the browser having behaved.
func NormalizeWorkState(raw string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	return s, usStates[s]
}

// RecordFirstWorkStateAssertion appends the onboarding row to the audit
// trail: what this person said her work state was, and when. It leaves
// previous_work_state NULL, which is what a first assertion means -- there
// was no earlier value.
//
// A change to an existing value is a different row with both sides filled
// in, and belongs to whoever builds the self-edit screen; there is no way
// to change a work state today, so there is no such caller to write for.
//
// actorStaffID is passed rather than assumed equal to staffID: only the
// person herself may assert where she works, so the two are always equal
// today, and a row where they differ is a signal rather than an
// impossibility.
func RecordFirstWorkStateAssertion(ctx context.Context, tx *sql.Tx, staffID, workState, actorStaffID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO staff_work_state_events (staff_id, work_state, actor_staff_id)
		 VALUES ($1, $2, $3)`,
		staffID, workState, actorStaffID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("staffauth: record first work state assertion: %w", err)
	}
	return nil
}
