package visit_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// #895 / ADR-0015: "intake -> active happens by itself, the first time a
// Visit is scheduled." Every test in this file drives one of the two
// Visit write paths that can leave a scheduled_at behind and reads the
// Engagement's own status and audit rows back, never the Visit response
// -- neither endpoint's contract changed.

// firstScheduledAt is the instant every activating write in this file
// sends. A single named constant so goconst does not see one literal per
// test.
const firstScheduledAt = "2027-05-04T10:00:00Z"

// engagementStatus reads an Engagement's status back on the superuser
// connection -- the fact all of ADR-0015's automatic move is about.
func engagementStatus(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&status); err != nil {
		t.Fatalf("read engagement status: %v", err)
	}
	return status
}

// countCarePhaseEntries counts the activity ledger's care_phase_changed
// entries -- the one record of the move since #1423 folded
// engagement_events into activity. "One-way, one-time" is a claim about
// this count, not only about the status column, so a test that scheduled
// twice asserts it rather than re-reading 'active'.
func countCarePhaseEntries(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'care_phase_changed'`,
		engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("count care phase entries: %v", err)
	}
	return count
}

// carePhaseActor reads the Staff id recorded on the newest
// care_phase_changed entry -- ADR-0015's "the event records the person
// who scheduled, not a null system actor".
func carePhaseActor(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var actor *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id::text FROM activity
		  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'care_phase_changed'
		  ORDER BY created_at DESC LIMIT 1`, engagementID,
	).Scan(&actor); err != nil {
		t.Fatalf("read care phase actor: %v", err)
	}
	if actor == nil {
		t.Fatal("care phase actor is NULL; ADR-0015 requires the person who scheduled")
	}
	return *actor
}

// schedule PATCHes a Visit's scheduled instant and requires 200. at == ""
// sends an explicit null, which is the clearing shape.
func (e *env) schedule(visitID, at string) {
	e.T.Helper()
	req := visit.ScheduleRequest{}
	if at != "" {
		req.ScheduledAt = &at
	}
	reqtest.ExpectStatus(e.T, http.MethodPatch, e.visitsURL("/"+visitID+"/schedule"), http.StatusOK, e.SignedIn, reqtest.JSON(req))
}

// requireEngagementAt proves the Engagement is at status with entries
// care_phase_changed rows behind it.
func (e *env) requireEngagementAt(status string, entries int) {
	e.T.Helper()
	if got := engagementStatus(e.T, e.DB, e.Engagement.ID); got != status {
		e.T.Fatalf("engagement status = %q, want %q", got, status)
	}
	if n := countCarePhaseEntries(e.T, e.DB, e.Engagement.ID); n != entries {
		e.T.Fatalf("care_phase_changed entries = %d, want %d", n, entries)
	}
}

// #895's headline on the creation path: booking a Visit for a date leaves
// the Engagement 'active', with no separate status request, and records
// the person who scheduled. When an Owner books a colleague onto a birth,
// two Staff ids are in play and they are not interchangeable: the assignee
// is who the Visit is for, and the actor is who did the booking. The
// trigger is "first scheduled", not "first created": logging a past
// meeting -- a Visit with no scheduledAt -- says nothing about the
// calendar, so the Engagement stays where the Practice put it.
func TestCreateHandler_ActivatesOnlyOnAScheduledVisit(t *testing.T) {
	cases := []struct {
		name        string
		caller      []fixture.StaffOption
		request     func(e *env) visit.CreateRequest
		wantStatus  string
		wantEntries int
	}{
		{"a scheduled Visit for herself", []fixture.StaffOption{fixture.Doula()}, func(*env) visit.CreateRequest {
			return visit.CreateRequest{ScheduledAt: new(firstScheduledAt)}
		}, engagement.StatusActive, 1},
		{"a scheduled Visit an Owner books for a colleague", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}, func(e *env) visit.CreateRequest {
			return visit.CreateRequest{StaffID: new(e.Colleague().ID), ScheduledAt: new(firstScheduledAt)}
		}, engagement.StatusActive, 1},
		{"a Visit with no date", []fixture.StaffOption{fixture.Doula()}, func(*env) visit.CreateRequest {
			return visit.CreateRequest{}
		}, engagement.StatusIntake, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.caller...))

			e.create(tc.request(e))

			e.requireEngagementAt(tc.wantStatus, tc.wantEntries)
			if tc.wantEntries > 0 {
				if got := carePhaseActor(t, e.DB, e.Engagement.ID); got != e.Staff.ID {
					t.Fatalf("care phase actor = %q, want the booker %q", got, e.Staff.ID)
				}
			}
		})
	}
}

// The other half of "first scheduled, not first created": a Visit logged
// with no date and given one later activates on that later write.
func TestScheduleHandler_ActivatesOnTheLaterSchedule(t *testing.T) {
	e := newEnv(t)
	visitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)
	e.requireEngagementAt(engagement.StatusIntake, 0)

	e.schedule(visitID, firstScheduledAt)

	e.requireEngagementAt(engagement.StatusActive, 1)
	if got := carePhaseActor(t, e.DB, e.Engagement.ID); got != e.Staff.ID {
		t.Fatalf("care phase actor = %q, want the scheduling Staff member %q", got, e.Staff.ID)
	}
}

// Clearing a Visit's instant is the opposite act, and ADR-0015's move is
// one-way: an Engagement is never activated by a write that leaves
// nothing on the calendar.
func TestScheduleHandler_ClearingAnInstantActivatesNothing(t *testing.T) {
	e := newEnv(t)
	visitID := seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, parseRFC3339(t, firstScheduledAt))

	e.schedule(visitID, "")

	e.requireEngagementAt(engagement.StatusIntake, 0)
}

// "One-way, one-time, never a repeated write": a Practice that books a
// second Visit, and reschedules the first, leaves exactly one status move
// behind, not one per booking.
func TestScheduleHandler_SecondScheduleWritesNoSecondMove(t *testing.T) {
	e := newEnv(t)
	firstVisitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)
	secondVisitID := seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	e.schedule(firstVisitID, firstScheduledAt)
	e.schedule(secondVisitID, "2027-06-01T09:00:00Z")
	e.schedule(firstVisitID, "2027-05-05T10:00:00Z")

	e.requireEngagementAt(engagement.StatusActive, 1)
}

// An Engagement past 'intake' is untouched, by either write path.
// 'completed' is the case that matters most: ADR-0015 keeps Visits open on
// a completed Engagement, so booking one must not quietly reopen the
// Practice's record.
func TestVisitWrites_LeaveAnEngagementPastIntakeAlone(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		// isCreated picks which write path books the Visit: creating one
		// already scheduled, or scheduling one that already exists. Both
		// leave a scheduled_at behind, so both would activate an
		// Engagement still at 'intake'.
		isCreated bool
	}{
		{"scheduling on an already active Engagement", engagement.StatusActive, false},
		{"scheduling on an already completed Engagement", engagement.StatusCompleted, false},
		{"creating a scheduled Visit on an already active Engagement", engagement.StatusActive, true},
		{"creating a scheduled Visit on an already completed Engagement", engagement.StatusCompleted, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, withEngagement(fixture.Status(tc.status)))

			if tc.isCreated {
				e.create(visit.CreateRequest{ScheduledAt: new(firstScheduledAt)})
			} else {
				e.schedule(seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID), firstScheduledAt)
			}

			e.requireEngagementAt(tc.status, 0)
		})
	}
}
