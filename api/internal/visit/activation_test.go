package visit_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/engagement"
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

// countStatusEvents counts the engagement_events rows ADR-0015's audit
// table holds for a status move. "One-way, one-time" is a claim about
// this count, not only about the status column, so a test that scheduled
// twice asserts it rather than re-reading 'active'.
func countStatusEvents(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_events
		  WHERE engagement_id = $1 AND event_type = 'status_changed'`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("count status events: %v", err)
	}
	return count
}

// countCarePhaseEntries is countStatusEvents' twin for the (portal-visible)
// activity ledger's own care_phase_changed action.
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

// statusEventActor reads the Staff id recorded on the newest
// 'status_changed' engagement_events row -- ADR-0015's "the event records
// the person who scheduled, not a null system actor".
func statusEventActor(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var actor *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id::text FROM engagement_events
		  WHERE engagement_id = $1 AND event_type = 'status_changed'
		  ORDER BY created_at DESC LIMIT 1`, engagementID,
	).Scan(&actor); err != nil {
		t.Fatalf("read status event actor: %v", err)
	}
	if actor == nil {
		t.Fatal("status event actor is NULL; ADR-0015 requires the person who scheduled")
	}
	return *actor
}

// carePhaseActor is statusEventActor for the activity ledger's own copy of
// the same move.
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

// scheduleVisit PATCHes a Visit's scheduled instant. at == "" sends an
// explicit null, which is the clearing shape.
func scheduleVisit(t *testing.T, session, srvURL, practiceID, engagementID, visitID, at string) *http.Response {
	t.Helper()
	req := visit.ScheduleRequest{}
	if at != "" {
		req.ScheduledAt = &at
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal schedule body: %v", err)
	}
	return authedPatch(t, session,
		srvURL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/visits/"+visitID+"/schedule", body)
}

// The headline of #895 on the creation path: booking a Visit for a date
// leaves the Engagement 'active', with no separate status request.
func TestCreateHandler_ScheduledVisitActivatesTheEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-activating-on-create"
	practiceID, actorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "",
		visit.CreateRequest{ScheduledAt: new(firstScheduledAt)})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusActive {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusActive)
	}
	if n := countStatusEvents(t, db, engagementID); n != 1 {
		t.Fatalf("status_changed events = %d, want 1", n)
	}
	if n := countCarePhaseEntries(t, db, engagementID); n != 1 {
		t.Fatalf("care_phase_changed entries = %d, want 1", n)
	}
	if got := statusEventActor(t, db, engagementID); got != actorStaffID {
		t.Fatalf("status event actor = %q, want the scheduling Staff member %q", got, actorStaffID)
	}
	if got := carePhaseActor(t, db, engagementID); got != actorStaffID {
		t.Fatalf("care phase actor = %q, want the scheduling Staff member %q", got, actorStaffID)
	}
}

// ADR-0015 records "the person who scheduled". When an Owner books a
// colleague onto a birth, two Staff ids are in play and they are not
// interchangeable: the assignee is who the Visit is for, and the actor is
// who did the booking.
func TestCreateHandler_ActivationActorIsTheBookerNotTheAssignee(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-booking-a-colleague"
	practiceID, bookerStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole, doulaRole}, "employee")
	assigneeStaffID := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-booked-onto-birth", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "",
		visit.CreateRequest{StaffID: &assigneeStaffID, ScheduledAt: new(firstScheduledAt)})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusActive {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusActive)
	}
	if got := statusEventActor(t, db, engagementID); got != bookerStaffID {
		t.Fatalf("status event actor = %q, want the booker %q (assignee is %q)", got, bookerStaffID, assigneeStaffID)
	}
	if got := carePhaseActor(t, db, engagementID); got != bookerStaffID {
		t.Fatalf("care phase actor = %q, want the booker %q (assignee is %q)", got, bookerStaffID, assigneeStaffID)
	}
}

// The trigger is "first scheduled", not "first created": logging a past
// meeting -- a Visit with no scheduledAt -- says nothing about the
// calendar, so the Engagement stays where the Practice put it.
func TestCreateHandler_UnscheduledVisitLeavesTheEngagementAtIntake(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-logging-a-past-visit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := createVisit(t, session, visitsURL(srv.URL, practiceID, engagementID), "", visit.CreateRequest{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusIntake {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusIntake)
	}
	if n := countStatusEvents(t, db, engagementID); n != 0 {
		t.Fatalf("status_changed events = %d, want 0", n)
	}
	if n := countCarePhaseEntries(t, db, engagementID); n != 0 {
		t.Fatalf("care_phase_changed entries = %d, want 0", n)
	}
}

// The other half of "first scheduled, not first created": a Visit logged
// with no date and given one later activates on that later write.
func TestScheduleHandler_ActivatesOnTheLaterSchedule(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-scheduling-later"
	practiceID, actorStaffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedVisit(t, db, engagementID, actorStaffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusIntake {
		t.Fatalf("engagement status before scheduling = %q, want %q", got, engagement.StatusIntake)
	}

	resp := scheduleVisit(t, session, srv.URL, practiceID, engagementID, visitID, firstScheduledAt)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusActive {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusActive)
	}
	if n := countStatusEvents(t, db, engagementID); n != 1 {
		t.Fatalf("status_changed events = %d, want 1", n)
	}
	if got := statusEventActor(t, db, engagementID); got != actorStaffID {
		t.Fatalf("status event actor = %q, want the scheduling Staff member %q", got, actorStaffID)
	}
}

// Clearing a Visit's instant is the opposite act, and ADR-0015's move is
// one-way: an Engagement is never activated by a write that leaves
// nothing on the calendar.
func TestScheduleHandler_ClearingAnInstantActivatesNothing(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-clearing-a-schedule"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	visitID := seedScheduledVisit(t, db, engagementID, staffID, parseRFC3339(t, firstScheduledAt))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := scheduleVisit(t, session, srv.URL, practiceID, engagementID, visitID, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusIntake {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusIntake)
	}
	if n := countStatusEvents(t, db, engagementID); n != 0 {
		t.Fatalf("status_changed events = %d, want 0", n)
	}
	if n := countCarePhaseEntries(t, db, engagementID); n != 0 {
		t.Fatalf("care_phase_changed entries = %d, want 0", n)
	}
}

// "One-way, one-time, never a repeated write": a Practice that books a
// second Visit, and reschedules the first, leaves exactly one status move
// behind, not one per booking.
func TestScheduleHandler_SecondScheduleWritesNoSecondMove(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-booking-twice"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	firstVisitID := seedVisit(t, db, engagementID, staffID)
	secondVisitID := seedVisit(t, db, engagementID, staffID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	first := scheduleVisit(t, session, srv.URL, practiceID, engagementID, firstVisitID, firstScheduledAt)
	_ = first.Body.Close()
	second := scheduleVisit(t, session, srv.URL, practiceID, engagementID, secondVisitID, "2027-06-01T09:00:00Z")
	_ = second.Body.Close()
	reschedule := scheduleVisit(t, session, srv.URL, practiceID, engagementID, firstVisitID, "2027-05-05T10:00:00Z")
	_ = reschedule.Body.Close()

	if got := engagementStatus(t, db, engagementID); got != engagement.StatusActive {
		t.Fatalf("engagement status = %q, want %q", got, engagement.StatusActive)
	}
	if n := countStatusEvents(t, db, engagementID); n != 1 {
		t.Fatalf("status_changed events = %d, want 1 (the move happens once)", n)
	}
	if n := countCarePhaseEntries(t, db, engagementID); n != 1 {
		t.Fatalf("care_phase_changed entries = %d, want 1 (the move happens once)", n)
	}
}

// An Engagement past 'intake' is untouched. 'completed' is the case that
// matters most: ADR-0015 keeps Visits open on a completed Engagement, so
// scheduling one must not quietly reopen the Practice's record.
func TestScheduleHandler_LeavesAnEngagementPastIntakeAlone(t *testing.T) {
	for _, tc := range []struct{ name, status string }{
		{"already active", engagement.StatusActive},
		{"already completed", engagement.StatusCompleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			identityUID := "doula-scheduling-on-" + tc.status
			practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID,
				"Client "+tc.status, tc.status+"@example.com", tc.status)
			visitID := seedVisit(t, db, engagementID, staffID)

			srv, session := newServer(t, db, identityUID)
			defer srv.Close()

			resp := scheduleVisit(t, session, srv.URL, practiceID, engagementID, visitID, firstScheduledAt)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}

			if got := engagementStatus(t, db, engagementID); got != tc.status {
				t.Fatalf("engagement status = %q, want it untouched at %q", got, tc.status)
			}
			if n := countStatusEvents(t, db, engagementID); n != 0 {
				t.Fatalf("status_changed events = %d, want 0", n)
			}
			if n := countCarePhaseEntries(t, db, engagementID); n != 0 {
				t.Fatalf("care_phase_changed entries = %d, want 0", n)
			}
		})
	}
}
