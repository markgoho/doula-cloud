package visit_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/visit"
)

// nextOnFirstPage reads the Engagement's Visits and answers the list's
// own "next" (#2755).
func nextOnFirstPage(t *testing.T, e *env) *visit.Visit {
	t.Helper()
	return reqtest.Expect[visit.ListResponse](t, http.MethodGet, e.visitsURL(""), http.StatusOK, e.SignedIn).Next
}

// #2755: the first page of the list names the earliest Visit scheduled
// from now on, over the whole Engagement.
func TestListHandler_NextIsTheEarliestVisitFromNow(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(-48*time.Hour))
	later := seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(72*time.Hour))
	soonest := seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(24*time.Hour))
	seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	next := nextOnFirstPage(t, e)

	if next == nil || next.VisitID != soonest || next.VisitID == later {
		t.Fatalf("next = %+v, want the Visit a day from now", next)
	}
	if next.StaffName == "" || next.Type == "" {
		t.Errorf("next = %+v, want the Visit as the list draws it", next)
	}
}

func TestListHandler_NextIsAbsentWithNoUpcomingVisit(t *testing.T) {
	e := newEnv(t)
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, time.Now().Add(-48*time.Hour))
	seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	if next := nextOnFirstPage(t, e); next != nil {
		t.Errorf("next = %+v, want none", next)
	}
}

func TestListHandler_NextIsOnTheFirstPageOnly(t *testing.T) {
	e := newEnv(t)
	for i := range 32 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, time.Now().Add(time.Duration(i+1)*time.Hour))
	}

	first := reqtest.Expect[visit.ListResponse](t, http.MethodGet, e.visitsURL(""), http.StatusOK, e.SignedIn)
	if first.Next == nil || first.NextCursor == nil {
		t.Fatalf("first page next = %+v cursor = %v, want both", first.Next, first.NextCursor)
	}
	second := reqtest.Expect[visit.ListResponse](t, http.MethodGet, e.visitsURL("?cursor="+*first.NextCursor), http.StatusOK, e.SignedIn)
	if second.Next != nil {
		t.Errorf("second page next = %+v, want none", second.Next)
	}
}
