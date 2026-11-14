package visit_test

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/visit"
)

func (e *env) waitingURL(params url.Values) string {
	if len(params) == 0 {
		return e.PracticeURL("/visits/waiting")
	}
	return e.PracticeURL("/visits/waiting?" + params.Encode())
}

func (e *env) readWaitingVisits(params url.Values) visit.WaitingVisitsResponse {
	e.T.Helper()
	return reqtest.Expect[visit.WaitingVisitsResponse](e.T, http.MethodGet, e.waitingURL(params), http.StatusOK, e.SignedIn)
}

func waitingEngagementIDs(resp visit.WaitingVisitsResponse) []string {
	ids := make([]string, 0, len(resp.Items))
	for _, item := range resp.Items {
		ids = append(ids, item.EngagementID)
	}
	return ids
}

func TestWaitingVisits_CountsAndListsDatedAndUndatedVisits(t *testing.T) {
	for name, staff := range map[string]fixture.StaffOption{"owner": fixture.Owner(), "admin": fixture.Admin()} {
		t.Run(name, func(t *testing.T) {
			f := newScheduleEnv(t, staff)
			f.takeOff(f.engagementA, f.employeeDoula.ID)
			f.takeOff(f.engagementB, f.otherDoula.ID)

			seedVisit(t, f.DB, f.engagementB, f.otherDoula.ID)
			seedVisit(t, f.DB, f.engagementB, f.otherDoula.ID)

			resp := f.readWaitingVisits(nil)

			if resp.Total != 5 || resp.DatedCount != 2 || resp.UndatedCount != 3 {
				t.Fatalf("total, dated, undated = %d, %d, %d, want 5, 2, 3", resp.Total, resp.DatedCount, resp.UndatedCount)
			}
			if want := []string{f.engagementA, f.engagementB}; !slices.Equal(waitingEngagementIDs(resp), want) {
				t.Fatalf("engagements = %v, want %v", waitingEngagementIDs(resp), want)
			}
			a, b := resp.Items[0], resp.Items[1]
			if a.ClientName != "Ada Client" || a.UndatedCount != 1 {
				t.Fatalf("engagement A = %+v, want Ada Client with one undated Visit", a)
			}
			if b.ClientName != "Bea Client" || b.UndatedCount != 2 {
				t.Fatalf("engagement B = %+v, want Bea Client with two undated Visits", b)
			}
			if resp.LatestScheduledAt == nil || !resp.LatestScheduledAt.Equal(f.base.Add(48*time.Hour)) {
				t.Fatalf("latestScheduledAt = %v, want %v", resp.LatestScheduledAt, f.base.Add(48*time.Hour))
			}
		})
	}
}

func TestWaitingVisits_DatedCountIsTheSchedulesCountOverTheSameDays(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.takeOff(f.engagementB, f.otherDoula.ID)
	far := seedScheduledVisitAt(f, f.engagementB, f.otherDoula.ID, f.base.Add(400*24*time.Hour))

	resp := f.readWaitingVisits(nil)

	days := window(f.base.Add(-time.Hour), resp.LatestScheduledAt.Add(time.Second))
	schedule := f.readWaiting(days)
	wantVisitCount(t, schedule, resp.DatedCount)
	if !slices.Contains(visitIDs(schedule), far) {
		t.Fatalf("schedule rows %v miss the Visit a year out %s", visitIDs(schedule), far)
	}
	if resp.DatedCount != 3 {
		t.Fatalf("datedCount = %d, want 3", resp.DatedCount)
	}
}

func seedScheduledVisitAt(f *scheduleEnv, engagementID, staffID string, at time.Time) string {
	return seedScheduledVisit(f.T, f.DB, engagementID, staffID, at)
}

func TestWaitingVisits_LeavesOutWhatIsNotWaiting(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	resp := f.readWaitingVisits(nil)

	if resp.Total != 0 || resp.DatedCount != 0 || resp.UndatedCount != 0 || len(resp.Items) != 0 || resp.LatestScheduledAt != nil {
		t.Fatalf("a Practice with no one taken off = %+v, want nothing waiting", resp)
	}
}

func TestWaitingVisits_AVisitBehindNowDoesNotWait(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.Exec(`UPDATE visits SET scheduled_at = now() - interval '1 hour' WHERE id = $1`, f.soonVisit)

	resp := f.readWaitingVisits(nil)

	if resp.DatedCount != 0 || resp.UndatedCount != 1 {
		t.Fatalf("dated, undated = %d, %d, want 0, 1", resp.DatedCount, resp.UndatedCount)
	}
}

func TestWaitingVisits_AnUndatedVisitOnACompletedEngagementDoesNotWait(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.Exec(`UPDATE engagements SET status = 'completed', birth_outcome = 'unknown', ending_reason = 'care_complete' WHERE id = $1`, f.engagementA)

	if resp := f.readWaitingVisits(nil); resp.Total != 0 {
		t.Fatalf("total = %d, want 0", resp.Total)
	}
}

func TestWaitingVisits_StaysInsideItsPractice(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	elsewhere := fixture.Practice(f.T, f.DB, fixture.PracticeName("Another Practice"))
	other := fixture.Engagement(f.T, f.DB, elsewhere)
	otherDoula := fixture.Staff(f.T, f.DB, elsewhere)
	f.takeOff(other.ID, otherDoula.ID)
	seedVisit(f.T, f.DB, other.ID, otherDoula.ID)

	if resp := f.readWaitingVisits(nil); resp.Total != 0 || len(resp.Items) != 0 {
		t.Fatalf("response = %+v, want nothing from another Practice", resp)
	}
}

func TestWaitingVisits_PagesKeepTheCountsWhole(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	colleague := e.Colleague()
	for i := range 31 {
		engagement := e.AnotherEngagement(fixture.NewClient(fixture.ClientName(fmt.Sprintf("Client %02d", i))))
		e.takeOff(engagement.ID, colleague.ID)
		seedVisit(t, e.DB, engagement.ID, colleague.ID)
	}

	first := e.readWaitingVisits(nil)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore %v", len(first.Items), first.HasMore)
	}
	second := e.readWaitingVisits(url.Values{cursorParameter: {*first.NextCursor}})
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page = %d items, hasMore %v, want 1 and no more", len(second.Items), second.HasMore)
	}
	if first.Total != 31 || second.Total != 31 {
		t.Fatalf("totals = %d, %d, want 31 on every page", first.Total, second.Total)
	}
	if slices.Contains(waitingEngagementIDs(first), second.Items[0].EngagementID) {
		t.Fatal("the second page repeats a row of the first")
	}
}

func TestWaitingVisits_RefusesABadCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	for name, cursor := range map[string]string{
		"not a cursor":  "not-a-cursor",
		"no name key":   pagecursor.EncodeKeyed("", nil, e.Engagement.ID),
		"another order": pagecursor.EncodeKeyed("latest", []string{"ada"}, e.Engagement.ID),
	} {
		t.Run(name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodGet, e.waitingURL(url.Values{cursorParameter: {cursor}}), http.StatusBadRequest, e.SignedIn)
		})
	}
}

func TestWaitingVisits_RefusesAReaderWhoIsNotOwnerOrAdmin(t *testing.T) {
	cases := map[string][]fixture.StaffOption{
		"an employee Doula":  {fixture.Doula()},
		"a contractor Doula": {fixture.Doula(), fixture.Contractor()},
		"a contractor alone": {fixture.Contractor()},
	}
	for name, staff := range cases {
		t.Run(name, func(t *testing.T) {
			f := newScheduleEnv(t, staff...)
			f.takeOff(f.engagementA, f.employeeDoula.ID)

			reqtest.ExpectStatus(t, http.MethodGet, f.waitingURL(nil), http.StatusForbidden, f.SignedIn)
		})
	}
}
