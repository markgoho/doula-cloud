package visit_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const offEngagementParameter = "offEngagement"

// takeOff ends staff's Attachment on an Engagement, which leaves the Visits she
// holds there waiting for a new Doula (#2596).
func (e *env) takeOff(engagementID, staffID string) {
	e.T.Helper()
	e.Exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at, ended_by, ended_reason)
		 VALUES ($1, $2, 'granted', $2, now(), $2, 'taken_off')`, engagementID, staffID)
}

func (f *scheduleEnv) readWaiting(params url.Values) visit.PracticeScheduleResponse {
	f.T.Helper()
	if params == nil {
		params = url.Values{}
	}
	params.Set(offEngagementParameter, "true")
	return f.readSchedule(params)
}

func wantOffEngagementCount(t *testing.T, page visit.PracticeScheduleResponse, want int) {
	t.Helper()
	if page.OffEngagementCount == nil {
		t.Fatalf("offEngagementCount absent, want %d", want)
	}
	if *page.OffEngagementCount != want {
		t.Fatalf("offEngagementCount = %d, want %d", *page.OffEngagementCount, want)
	}
}

// #2863: the narrowing is the list's own predicate, so the rows and the count
// are of the same set.
func TestPracticeSchedule_OffEngagementNarrowsTheRowsAndTheCountTogether(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)

	page := f.readWaiting(nil)
	if got := visitIDs(page); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("narrowed rows = %v, want [%s]", got, f.soonVisit)
	}
	wantVisitCount(t, page, 1)
	if page.OffEngagementCount != nil {
		t.Fatalf("a narrowed read carries offEngagementCount = %d, want it absent", *page.OffEngagementCount)
	}
}

func TestPracticeSchedule_FirstUnnarrowedPageCountsTheVisitsWaitingForAnOwnerAndAnAdmin(t *testing.T) {
	for name, staff := range map[string]fixture.StaffOption{"owner": fixture.Owner(), "admin": fixture.Admin()} {
		t.Run(name, func(t *testing.T) {
			f := newScheduleEnv(t, staff)
			f.takeOff(f.engagementA, f.employeeDoula.ID)

			page := f.readSchedule(nil)
			wantVisitCount(t, page, 2)
			wantOffEngagementCount(t, page, 1)
		})
	}
}

func TestPracticeSchedule_OffEngagementCountIsZeroNotAbsentWhenNothingWaits(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	wantOffEngagementCount(t, f.readSchedule(nil), 0)
}

func TestPracticeSchedule_OffEngagementCountFollowsTheOtherNarrowings(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.takeOff(f.engagementB, f.otherDoula.ID)

	wantOffEngagementCount(t, f.readSchedule(nil), 2)
	wantOffEngagementCount(t, f.readSchedule(url.Values{staffParameter: {f.otherDoula.ID}}), 1)
	wantOffEngagementCount(t, f.readSchedule(window(f.base.Add(-time.Hour), f.base.Add(2*time.Hour))), 1)
}

func TestPracticeSchedule_OffEngagementComposesWithTheDaysTheDoulaAndTheOrder(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.takeOff(f.engagementB, f.otherDoula.ID)

	if got := visitIDs(f.readWaiting(nil)); !slices.Equal(got, []string{f.soonVisit, f.laterVisit}) {
		t.Fatalf("soonest first = %v, want [%s %s]", got, f.soonVisit, f.laterVisit)
	}
	latest := f.readWaiting(latestFirst())
	if got := visitIDs(latest); !slices.Equal(got, []string{f.laterVisit, f.soonVisit}) {
		t.Fatalf("latest first = %v, want [%s %s]", got, f.laterVisit, f.soonVisit)
	}

	days := f.readWaiting(window(f.base.Add(-time.Hour), f.base.Add(2*time.Hour)))
	if got := visitIDs(days); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("days = %v, want [%s]", got, f.soonVisit)
	}
	wantVisitCount(t, days, 1)

	doula := f.readWaiting(url.Values{staffParameter: {f.otherDoula.ID}})
	if got := visitIDs(doula); !slices.Equal(got, []string{f.laterVisit}) {
		t.Fatalf("doula = %v, want [%s]", got, f.laterVisit)
	}
	wantVisitCount(t, doula, 1)
}

func TestPracticeSchedule_OffEngagementComposesWithTheSearch(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.takeOff(f.engagementB, f.otherDoula.ID)

	page := f.searchSchedule(visit.ScheduleListRequest{Search: searchAda, OffEngagement: true})
	if got := visitIDs(page); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("search = %v, want [%s]", got, f.soonVisit)
	}
	wantVisitCount(t, page, 1)

	unnarrowed := f.searchSchedule(visit.ScheduleListRequest{Search: searchAda})
	wantOffEngagementCount(t, unnarrowed, 1)
}

func TestPracticeSchedule_OffEngagementPagesKeepTheNarrowingAndOmitTheCount(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	colleague := e.Colleague()
	e.takeOff(e.Engagement.ID, colleague.ID)
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for i := range 31 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, colleague.ID, base.Add(time.Duration(i)*time.Minute))
	}
	seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, base.Add(time.Hour))
	params := url.Values{offEngagementParameter: {"true"}}

	first := e.readSchedule(params)
	wantVisitCount(t, first, 31)
	if rows := e.pagedRows(params); rows != 31 {
		t.Fatalf("rows paged through = %d, want the 31 that wait", rows)
	}
	params.Set(cursorParameter, *first.NextCursor)
	if second := e.readSchedule(params); second.OffEngagementCount != nil || second.VisitCount != nil {
		t.Fatalf("second page carries a count: %+v", second)
	}
}

func TestPracticeSchedule_ADoulaWhoIsNotOwnerOrAdminGetsNoOffEngagementCount(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula())
	f.takeOff(f.engagementA, f.employeeDoula.ID)

	page := f.readSchedule(nil)
	if page.OffEngagementCount != nil {
		t.Fatalf("offEngagementCount = %d for a Doula, want it absent", *page.OffEngagementCount)
	}
	if got := visitIDs(f.readWaiting(nil)); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("narrowed rows = %v, want [%s]", got, f.soonVisit)
	}
}

// A contractor's rows stay narrowed by her Attachments under the narrowing, and
// her read says nothing about how many Visits the Practice has waiting.
func TestPracticeSchedule_OffEngagementKeepsAContractorToHerAttachments(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
	f.takeOff(f.engagementA, f.employeeDoula.ID)
	f.takeOff(f.engagementB, f.otherDoula.ID)

	page := f.readWaiting(nil)
	if got := visitIDs(page); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("contractor rows = %v, want only the Visit on her Engagement [%s]", got, f.soonVisit)
	}
	wantVisitCount(t, page, 1)
	if f.readSchedule(nil).OffEngagementCount != nil {
		t.Fatal("a contractor's read carries offEngagementCount, want it absent")
	}
}

func TestPracticeSchedule_OffEngagementIsTrueOrAbsent(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	for _, value := range []string{"false", "", "1", "yes"} {
		t.Run("value "+value, func(t *testing.T) {
			body := reqtest.ExpectBody(t, http.MethodGet, f.scheduleURL(url.Values{offEngagementParameter: {value}}), http.StatusBadRequest, f.SignedIn)
			if !strings.Contains(string(body), visit.MsgInvalidOffEngagement) {
				t.Fatalf("body = %s, want %q", body, visit.MsgInvalidOffEngagement)
			}
		})
	}
}
