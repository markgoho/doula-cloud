package visit_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/visit"
)

// #2596: a Visit that has not happened (future or undated, #2665) held by a Doula who was taken off the Engagement is
// flagged on both Staff reads. A past Visit, a Visit of a Doula still on it,
// and a Visit on a Doula who never had an Attachment are not.
func TestVisitReads_FlagAnOpenVisitWhoseDoulaWasTakenOff(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	takenOff, staysOn, never := e.Colleague(), e.Colleague(), e.Colleague()
	e.Exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at, ended_by, ended_reason)
		 VALUES ($1, $2, 'granted', $2, now(), $2, 'taken_off')`, e.Engagement.ID, takenOff.ID)
	e.Exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by)
		 VALUES ($1, $2, 'granted', $2)`, e.Engagement.ID, staysOn.ID)
	soon := time.Now().Add(3 * 24 * time.Hour)
	offBooked := seedScheduledVisit(t, e.DB, e.Engagement.ID, takenOff.ID, soon)
	offPast := seedScheduledVisit(t, e.DB, e.Engagement.ID, takenOff.ID, time.Now().Add(-3*24*time.Hour))
	onBooked := seedScheduledVisit(t, e.DB, e.Engagement.ID, staysOn.ID, soon)
	neverBooked := seedScheduledVisit(t, e.DB, e.Engagement.ID, never.ID, soon)
	unscheduled := seedVisit(t, e.DB, e.Engagement.ID, takenOff.ID)
	want := map[string]bool{offBooked: true, offPast: false, onBooked: false, neverBooked: false, unscheduled: true}

	list := reqtest.Expect[visit.ListResponse](t, http.MethodGet, e.visitsURL(""), http.StatusOK, e.SignedIn)
	for _, v := range list.Items {
		if v.IsOffEngagement != want[v.VisitID] {
			t.Errorf("engagement list: visit %s isOffEngagement = %v, want %v", v.VisitID, v.IsOffEngagement, want[v.VisitID])
		}
	}

	page := reqtest.Expect[visit.PracticeScheduleResponse](t, http.MethodGet, e.PracticeURL("/visits"), http.StatusOK, e.SignedIn)
	if len(page.Items) != 3 {
		t.Fatalf("schedule items = %d, want the 3 dated Visits", len(page.Items))
	}
	for _, v := range page.Items {
		if v.IsOffEngagement != want[v.VisitID] {
			t.Errorf("schedule: visit %s isOffEngagement = %v, want %v", v.VisitID, v.IsOffEngagement, want[v.VisitID])
		}
	}
}
