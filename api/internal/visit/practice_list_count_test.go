package visit_test

import (
	"maps"
	"net/url"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// wantVisitCount fails unless the page carries a VisitCount equal to want.
func wantVisitCount(t *testing.T, page visit.PracticeScheduleResponse, want int) {
	t.Helper()
	if page.VisitCount == nil {
		t.Fatalf("visitCount absent, want %d", want)
	}
	if *page.VisitCount != want {
		t.Fatalf("visitCount = %d, want %d", *page.VisitCount, want)
	}
}

// pagedRows walks every page of the schedule and returns the rows it holds.
func (e *env) pagedRows(params url.Values) int {
	e.T.Helper()
	rows := 0
	page := e.readSchedule(params)
	for {
		rows += len(page.Items)
		if page.NextCursor == nil {
			return rows
		}
		next := maps.Clone(params)
		if next == nil {
			next = url.Values{}
		}
		next.Set(cursorParameter, *page.NextCursor)
		page = e.readSchedule(next)
	}
}

// TestPracticeSchedule_FirstPageCarriesTheTotalOfTheNarrowing is #2867: the
// first page says how many Visits the days and the Doula chosen hold, and a
// page after it says nothing.
func TestPracticeSchedule_FirstPageCarriesTheTotalOfTheNarrowing(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	wantVisitCount(t, f.readSchedule(nil), 2)
	wantVisitCount(t, f.readSchedule(url.Values{staffParameter: {f.otherDoula.ID}}), 1)
	wantVisitCount(t, f.readSchedule(window(f.base.Add(-96*time.Hour), f.base)), 1)
	wantVisitCount(t, f.readSchedule(window(f.base.Add(200*time.Hour), f.base.Add(300*time.Hour))), 0)
}

func TestPracticeSchedule_TotalIsWholeListNotThePageAndLaterPagesOmitIt(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	for i := range 31 {
		seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID, base.Add(time.Duration(i)*time.Minute))
	}

	first := e.readSchedule(nil)
	if len(first.Items) != 30 {
		t.Fatalf("first page = %d items, want 30", len(first.Items))
	}
	wantVisitCount(t, first, 31)

	second := e.readSchedule(url.Values{cursorParameter: {*first.NextCursor}})
	if second.VisitCount != nil {
		t.Fatalf("second page visitCount = %d, want it absent", *second.VisitCount)
	}
}

// TestPracticeSchedule_ContractorTotalIsTheRowsSheCanPageThrough: her total
// is narrowed by her attachments as her rows are, so it never tells her the
// Practice's size.
func TestPracticeSchedule_ContractorTotalIsTheRowsSheCanPageThrough(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
	for i := range 31 {
		seedScheduledVisit(t, f.DB, f.engagementA, f.employeeDoula.ID, f.base.Add(time.Duration(i+1)*time.Minute))
	}

	first := f.readSchedule(nil)
	if !first.HasMore {
		t.Fatal("first page hasMore = false, want a second page to walk")
	}
	// soonVisit plus the 31 seeded, all on the one Engagement she holds;
	// laterVisit is on the other and is not hers.
	wantVisitCount(t, first, 32)
	if rows := f.pagedRows(nil); rows != 32 {
		t.Fatalf("rows paged through = %d, want the total 32", rows)
	}
}

func TestPracticeSchedule_UnattachedContractorTotalIsZero(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())

	wantVisitCount(t, f.readSchedule(nil), 0)
}
