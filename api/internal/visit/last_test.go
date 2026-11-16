package visit_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/visit"
)

func readVisits(t *testing.T, e *env, query string) visit.ListResponse {
	t.Helper()
	return reqtest.Expect[visit.ListResponse](t, http.MethodGet, e.visitsURL(query), http.StatusOK, e.SignedIn)
}

// #2991: the first page names the newest Visit scheduled before now, over
// the whole Engagement.
func TestListHandler_LastIsTheNewestVisitBeforeNow(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(-96*time.Hour))
	newest := seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(-24*time.Hour))
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, now.Add(24*time.Hour))
	seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	last := readVisits(t, e, "").Last

	if last == nil || last.VisitID != newest {
		t.Fatalf("last = %+v, want the Visit a day ago", last)
	}
}

func TestListHandler_LastIsAbsentWithNoPastVisit(t *testing.T) {
	e := newEnv(t)
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, time.Now().Add(24*time.Hour))
	seedVisit(t, e.DB, e.Engagement.ID, e.Staff.ID)

	if last := readVisits(t, e, "").Last; last != nil {
		t.Errorf("last = %+v, want none", last)
	}
}

func TestListHandler_LastIsOnTheFirstPageOnly(t *testing.T) {
	e := newEnv(t)
	for i := range 32 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, time.Now().Add(-time.Duration(i+1)*time.Hour))
	}

	first := readVisits(t, e, "")
	if first.Last == nil || first.NextCursor == nil {
		t.Fatalf("first page last = %+v cursor = %v, want both", first.Last, first.NextCursor)
	}
	if second := readVisits(t, e, "?cursor="+*first.NextCursor); second.Last != nil {
		t.Errorf("second page last = %+v, want none", second.Last)
	}
}
