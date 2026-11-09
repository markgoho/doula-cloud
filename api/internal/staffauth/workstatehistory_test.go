package staffauth_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// seedWorkStateEvent writes one row of the audit trail directly, so a
// test can lay out a history spanning years without driving the
// self-edit endpoint once per entry. previous is the empty string for
// the first assertion, which is stored as NULL.
func seedWorkStateEvent(t *testing.T, db *testdb.DB, staffID, previous, next string, at time.Time) {
	t.Helper()
	var prev any
	if previous != "" {
		prev = previous
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_work_state_events (staff_id, previous_work_state, work_state, actor_staff_id, created_at)
		 VALUES ($1, $2, $3, $1, $4)`,
		staffID, prev, next, at,
	); err != nil {
		t.Fatalf("seed work state event: %v", err)
	}
}

// workStateHistory reads staffID's work state history as the Staff member
// at the screen.
func (e *env) workStateHistory(staffID string) staffauth.WorkStateHistory {
	e.T.Helper()
	return reqtest.Expect[staffauth.WorkStateHistory](e.T, http.MethodGet, e.staffRoute(staffID, "work-state-history"), http.StatusOK, e.SignedIn)
}

// TestListWorkStateHistory_FirstAssertionAndChange is the ticket's core
// case: a first assertion carries no previous value and a move carries
// both sides, so the screen can print two different sentences (#459's
// "a first assertion is not printed as a change").
func TestListWorkStateHistory_FirstAssertionAndChange(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()

	joined := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	moved := time.Date(2027, 3, 14, 9, 30, 0, 0, time.UTC)
	seedWorkStateEvent(t, e.DB, doula.ID, "", "NY", joined)
	seedWorkStateEvent(t, e.DB, doula.ID, "NY", "NJ", moved)

	history := e.workStateHistory(doula.ID)
	if len(history.Items) != 2 {
		t.Fatalf("items = %+v, want 2", history.Items)
	}
	// Newest first.
	if history.Items[0].PreviousWorkState != "NY" || history.Items[0].WorkState != "NJ" {
		t.Errorf("newest item = %+v, want NY -> NJ", history.Items[0])
	}
	if history.Items[1].PreviousWorkState != "" {
		t.Errorf("first assertion previousWorkState = %q, want empty", history.Items[1].PreviousWorkState)
	}
	if history.Items[1].WorkState != "NY" {
		t.Errorf("first assertion workState = %q, want NY", history.Items[1].WorkState)
	}
	if history.HasMore {
		t.Error("hasMore = true, want false for a two-entry history")
	}
	if history.NextCursor != nil {
		t.Errorf("nextCursor = %v, want nil", history.NextCursor)
	}
}

// TestListWorkStateHistory_ReAssertionIsNotAChange pins the changes-only
// filter. Saving an unchanged value is a real act (#437) and its row
// stays in the table, but the roster already shows it as the date beside
// the current value, so the history prints the moves and not the
// repetitions.
func TestListWorkStateHistory_ReAssertionIsNotAChange(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()

	seedWorkStateEvent(t, e.DB, doula.ID, "", "NY", time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC))
	seedWorkStateEvent(t, e.DB, doula.ID, "NY", "NY", time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
	seedWorkStateEvent(t, e.DB, doula.ID, "NY", "NY", time.Date(2027, 1, 5, 12, 0, 0, 0, time.UTC))

	history := e.workStateHistory(doula.ID)
	if len(history.Items) != 1 {
		t.Fatalf("items = %+v, want only the first assertion", history.Items)
	}
	if history.Items[0].WorkState != "NY" || history.Items[0].PreviousWorkState != "" {
		t.Errorf("item = %+v, want the NULL-previous first assertion", history.Items[0])
	}
}

// TestListWorkStateHistory_MemberSinceDatesTheMembership is #459's
// inherited-value criterion. A contractor doula who asserted her work
// state at an earlier Practice carries that row into this one -- 00043's
// table has no practice_id and joining a second Practice writes no event
// -- so the response has to date the Membership, which is what lets the
// screen say the assertion was made before she joined.
func TestListWorkStateHistory_MemberSinceDatesTheMembership(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()

	assertedElsewhere := time.Date(2025, 5, 4, 12, 0, 0, 0, time.UTC)
	seedWorkStateEvent(t, e.DB, doula.ID, "", "NY", assertedElsewhere)

	history := e.workStateHistory(doula.ID)
	if history.MemberSince.IsZero() {
		t.Fatal("memberSince is zero, want the Membership's created_at")
	}
	if !history.MemberSince.After(assertedElsewhere) {
		t.Errorf("memberSince = %v, want later than the assertion at %v", history.MemberSince, assertedElsewhere)
	}
}
