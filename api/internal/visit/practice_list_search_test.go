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

// searchSchedule reads the schedule as a POST with the search in the body
// and requires the 200.
func (e *env) searchSchedule(req visit.ScheduleListRequest) visit.PracticeScheduleResponse {
	e.T.Helper()
	return reqtest.Expect[visit.PracticeScheduleResponse](e.T, http.MethodPost, e.PracticeURL("/visits/list"),
		http.StatusOK, e.SignedIn, reqtest.JSON(req))
}

func (e *env) expectSearchRefused(req visit.ScheduleListRequest, status int) string {
	e.T.Helper()
	body := reqtest.ExpectBody(e.T, http.MethodPost, e.PracticeURL("/visits/list"), status, e.SignedIn, reqtest.JSON(req))
	return string(body)
}

const (
	searchAda    = "ada"
	searchClient = "Client"
)

func rfc3339Window(from, to time.Time) (string, string) {
	return from.Format(time.RFC3339), to.Format(time.RFC3339)
}

// TestScheduleList_MatchesTheClientNameAndCountsWhatItMatches is #2870: the
// search narrows the rows and the count together.
func TestScheduleList_MatchesTheClientNameAndCountsWhatItMatches(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	page := f.searchSchedule(visit.ScheduleListRequest{Search: searchAda})
	if got, want := visitIDs(page), []string{f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("search ada = %v, want %v", got, want)
	}
	wantVisitCount(t, page, 1)

	mixed := f.searchSchedule(visit.ScheduleListRequest{Search: "  BEA CLIENT "})
	if got, want := visitIDs(mixed), []string{f.laterVisit}; !slices.Equal(got, want) {
		t.Fatalf("search BEA CLIENT = %v, want %v", got, want)
	}
	wantVisitCount(t, mixed, 1)
}

func TestScheduleList_NoMatchIsAnEmptyPageWithAZeroCount(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	page := f.searchSchedule(visit.ScheduleListRequest{Search: "Zelda"})
	if len(page.Items) != 0 {
		t.Fatalf("search Zelda = %v, want none", visitIDs(page))
	}
	wantVisitCount(t, page, 0)
}

func TestScheduleList_APercentSignMatchesOnlyItself(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	if page := f.searchSchedule(visit.ScheduleListRequest{Search: "%"}); len(page.Items) != 0 {
		t.Fatalf("search %% = %v, want none", visitIDs(page))
	}
}

func TestScheduleList_NoSearchIsTheSameListAsTheGet(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	posted := f.searchSchedule(visit.ScheduleListRequest{})
	if got, want := visitIDs(posted), visitIDs(f.readSchedule(nil)); !slices.Equal(got, want) {
		t.Fatalf("POST = %v, GET = %v", got, want)
	}
	wantVisitCount(t, posted, 2)
}

// TestScheduleList_ASearchComposesWithTheDaysTheDoulaAndTheOrder: each
// narrowing holds beside the search, and the count follows.
func TestScheduleList_ASearchComposesWithTheDaysTheDoulaAndTheOrder(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	// A second Visit for Ada, with the other Doula, two days after the first.
	adaWithOther := seedScheduledVisit(t, f.DB, f.engagementA, f.otherDoula.ID, f.base.Add(24*time.Hour))

	both := f.searchSchedule(visit.ScheduleListRequest{Search: searchClient, Order: "latest"})
	if got, want := visitIDs(both), []string{f.laterVisit, adaWithOther, f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("Client, latest first = %v, want %v", got, want)
	}
	wantVisitCount(t, both, 3)

	byDoula := f.searchSchedule(visit.ScheduleListRequest{Search: searchAda, StaffID: f.otherDoula.ID})
	if got, want := visitIDs(byDoula), []string{adaWithOther}; !slices.Equal(got, want) {
		t.Fatalf("ada with the other Doula = %v, want %v", got, want)
	}
	wantVisitCount(t, byDoula, 1)

	from, to := rfc3339Window(f.base.Add(-time.Hour), f.base.Add(12*time.Hour))
	byDays := f.searchSchedule(visit.ScheduleListRequest{Search: searchClient, From: from, To: to})
	if got, want := visitIDs(byDays), []string{f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("Client in the first twelve hours = %v, want %v", got, want)
	}
	wantVisitCount(t, byDays, 1)
}

// TestScheduleList_PageTwoUnderASearchByCursor walks the keyed cursor in the
// body across a page boundary and keeps the search.
func TestScheduleList_PageTwoUnderASearchByCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.NewClient(fixture.ClientName("Ada Client"))))
	other := e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Bea Client")))
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for i := range 31 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, base.Add(time.Duration(i)*time.Minute))
	}
	seedScheduledVisit(t, e.DB, other.ID, e.Staff.ID, base.Add(time.Minute))

	first := e.searchSchedule(visit.ScheduleListRequest{Search: searchAda})
	if len(first.Items) != 30 || first.NextCursor == nil {
		t.Fatalf("first page = %d items, cursor=%v; want 30 and a cursor", len(first.Items), first.NextCursor)
	}
	wantVisitCount(t, first, 31)

	second := e.searchSchedule(visit.ScheduleListRequest{Search: searchAda, Cursor: *first.NextCursor})
	if len(second.Items) != 1 || second.HasMore || second.VisitCount != nil {
		t.Fatalf("second page = %d items, hasMore=%v, count=%v; want the one last row and no count",
			len(second.Items), second.HasMore, second.VisitCount)
	}
	for _, item := range append(first.Items, second.Items...) {
		if item.ClientName != "Ada Client" {
			t.Fatalf("a row for %q came through a search for ada", item.ClientName)
		}
	}
}

// TestScheduleList_AContractorSearchingSeesOnlyHerOwn: the rule she did not
// choose holds under a search, in the rows and the count.
func TestScheduleList_AContractorSearchingSeesOnlyHerOwn(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)

	page := f.searchSchedule(visit.ScheduleListRequest{Search: searchClient})
	if got, want := visitIDs(page), []string{f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("contractor searching Client = %v, want only %v", got, want)
	}
	wantVisitCount(t, page, 1)

	other := f.searchSchedule(visit.ScheduleListRequest{Search: "Bea"})
	if len(other.Items) != 0 {
		t.Fatalf("contractor searching Bea = %v, want none", visitIDs(other))
	}
	wantVisitCount(t, other, 0)
}

func TestScheduleList_RefusesWhatTheGetRefuses(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	for name, req := range map[string]visit.ScheduleListRequest{
		"a from that is not a time":  {From: "yesterday"},
		"a to that is not a time":    {To: "tomorrow"},
		"an empty range":             {From: "2027-01-02T00:00:00Z", To: "2027-01-01T00:00:00Z"},
		"an unknown order":           {Order: "oldest"},
		"a staff id that is no uuid": {StaffID: "not-a-uuid"},
		"a cursor that is no cursor": {Cursor: "garbage"},
	} {
		t.Run(name, func(_ *testing.T) {
			f.expectSearchRefused(req, http.StatusBadRequest)
		})
	}
}

func TestScheduleList_RefusesASearchOverOneHundredCharacters(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	f.searchSchedule(visit.ScheduleListRequest{Search: strings.Repeat("é", 100)})
	body := f.expectSearchRefused(visit.ScheduleListRequest{Search: strings.Repeat("é", 101)}, http.StatusBadRequest)
	if !strings.Contains(body, visit.MsgSearchLength) {
		t.Fatalf("body = %s, want %q", body, visit.MsgSearchLength)
	}
}

func TestScheduleList_RefusesABodyThatIsNotJSON(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	reqtest.ExpectStatus(t, http.MethodPost, f.PracticeURL("/visits/list"), http.StatusBadRequest,
		f.SignedIn, reqtest.RawJSON([]byte("not json")))
}

func TestScheduleList_RefusesAnUnsignedCaller(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	reqtest.ExpectStatus(t, http.MethodPost, f.PracticeURL("/visits/list"), http.StatusUnauthorized,
		reqtest.JSON(visit.ScheduleListRequest{}))
}

// TestPracticeSchedule_AGetCarryingASearchIsRefused: a search in the address
// is refused, never ignored, since ignoring it would answer with the whole
// schedule (#1829).
func TestPracticeSchedule_AGetCarryingASearchIsRefused(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	for _, key := range []string{"q", "search"} {
		t.Run(key, func(t *testing.T) {
			body := reqtest.ExpectBody(t, http.MethodGet, f.scheduleURL(url.Values{key: {searchAda}}), http.StatusBadRequest, f.SignedIn)
			if !strings.Contains(string(body), visit.MsgSearchInBody) {
				t.Fatalf("body = %s, want %q", body, visit.MsgSearchInBody)
			}
		})
	}
}
