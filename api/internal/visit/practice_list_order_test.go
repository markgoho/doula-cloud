package visit_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const orderParameter = "order"

func latestFirst() url.Values { return url.Values{orderParameter: {"latest"}} }

// TestPracticeSchedule_LatestFirstReversesTheOrderAndKeepsTheTotal is #2869:
// the order is the reader's, and the count is of the same set either way.
func TestPracticeSchedule_LatestFirstReversesTheOrderAndKeepsTheTotal(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	page := f.readSchedule(latestFirst())
	if got, want := visitIDs(page), []string{f.laterVisit, f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("latest first = %v, want %v", got, want)
	}
	wantVisitCount(t, page, 2)

	empty := f.readSchedule(url.Values{orderParameter: {""}})
	if got, want := visitIDs(empty), []string{f.soonVisit, f.laterVisit}; !slices.Equal(got, want) {
		t.Fatalf("empty order = %v, want soonest first %v", got, want)
	}
}

// TestPracticeSchedule_PagesLatestFirst walks the descending cursor across a
// page boundary, with two Visits sharing an instant so the id tiebreak is
// exercised.
func TestPracticeSchedule_PagesLatestFirst(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	seeded := make([]string, 0, 31)
	for i := range 31 {
		at := base.Add(time.Duration(i) * time.Minute)
		if i == 1 {
			at = base
		}
		seeded = append(seeded, seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, at))
	}

	first := e.readSchedule(latestFirst())
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}
	wantVisitCount(t, first, 31)

	params := latestFirst()
	params.Set(cursorParameter, *first.NextCursor)
	second := e.readSchedule(params)
	if second.HasMore || second.NextCursor != nil || second.VisitCount != nil {
		t.Fatalf("second page = hasMore %v, cursor %v, count %v; want the last page with no count",
			second.HasMore, second.NextCursor, second.VisitCount)
	}

	got := append(visitIDs(first), visitIDs(second)...)
	if len(got) != 31 {
		t.Fatalf("paged %d rows, want 31", len(got))
	}
	seen := map[string]bool{}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("Visit %s appeared on two pages", id)
		}
		seen[id] = true
	}
	if last := seeded[30]; got[0] != last {
		t.Fatalf("first row = %s, want the latest Visit %s", got[0], last)
	}
}

func TestPracticeSchedule_RefusesAnUnknownOrder(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	reqtest.ExpectStatus(t, http.MethodGet, f.scheduleURL(url.Values{orderParameter: {"name"}}), http.StatusBadRequest, f.SignedIn)
}

// TestPracticeSchedule_RefusesACursorFromTheOtherOrder: a cursor is bound to
// the order it was minted in, both ways round.
func TestPracticeSchedule_RefusesACursorFromTheOtherOrder(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for i := range 31 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, base.Add(time.Duration(i)*time.Minute))
	}

	soonest := e.readSchedule(nil)
	latest := e.readSchedule(latestFirst())

	cases := []struct {
		name   string
		order  url.Values
		cursor string
	}{
		{"soonest cursor under latest", latestFirst(), *soonest.NextCursor},
		{"latest cursor under soonest", url.Values{}, *latest.NextCursor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.order.Set(cursorParameter, tc.cursor)
			reqtest.ExpectStatus(t, http.MethodGet, e.scheduleURL(tc.order), http.StatusBadRequest, e.SignedIn)
		})
	}
}

func TestPracticeSchedule_RefusesACursorWhoseKeysAreNotAnInstant(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	const id = "11111111-1111-1111-1111-111111111111"
	for name, cursor := range map[string]string{
		"no keys":        pagecursor.EncodeKeyed("", nil, id),
		"not an instant": pagecursor.EncodeKeyed("", []string{"soon"}, id),
	} {
		t.Run(name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodGet, f.scheduleURL(url.Values{cursorParameter: {cursor}}), http.StatusBadRequest, f.SignedIn)
		})
	}
}

func TestPracticeSchedule_ContractorLatestFirstReadsOnlyHerRows(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
	later := seedScheduledVisit(t, f.DB, f.engagementA, f.employeeDoula.ID, f.base.Add(24*time.Hour))

	page := f.readSchedule(latestFirst())
	if got, want := visitIDs(page), []string{later, f.soonVisit}; !slices.Equal(got, want) {
		t.Fatalf("contractor latest first = %v, want %v", got, want)
	}
	wantVisitCount(t, page, 2)
}
