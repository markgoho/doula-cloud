package visit_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// Named once each so golangci-lint's goconst check does not see a handful
// of independent literals across this file.
const (
	ownerRole      = "owner"
	employeeType   = "employee"
	fromParameter  = "from"
	toParameter    = "to"
	staffParameter = "staffId"
)

// scheduleURL is the Practice-wide schedule route with whatever narrowing
// a test wants, so no test has to hand-assemble a query string.
func (e *env) scheduleURL(params url.Values) string {
	if len(params) == 0 {
		return e.PracticeURL("/visits")
	}
	return e.PracticeURL("/visits?" + params.Encode())
}

// readSchedule reads the Practice-wide schedule as the Staff member at the
// screen and requires the 200.
func (e *env) readSchedule(params url.Values) visit.PracticeScheduleResponse {
	e.T.Helper()
	return reqtest.Expect[visit.PracticeScheduleResponse](e.T, http.MethodGet, e.scheduleURL(params), http.StatusOK, e.SignedIn)
}

// visitIDs pulls the identities out of a page, so an assertion compares
// what is in the list rather than how it was formatted.
func visitIDs(page visit.PracticeScheduleResponse) []string {
	ids := make([]string, 0, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.VisitID)
	}
	return ids
}

// scheduleEnv is one Practice with two Clients, two Doulas and four
// Visits, read by the Staff member at the screen -- the smallest shape
// that can tell every acceptance criterion apart: a Visit with no
// scheduled instant, a Visit outside the window, two Doulas to filter
// between, and one Engagement a contractor is attached to.
type scheduleEnv struct {
	*env
	// engagementA is the Engagement the contractor holds an open, granted
	// attachment on; engagementB is the one she does not.
	engagementA, engagementB string
	// clientA is engagementA's Client, named in full by a test (#2114).
	clientA                   string
	employeeDoula, otherDoula fixture.StaffRecord
	// soon and later are both inside the default window; unscheduledVisit
	// has no instant at all and past is behind it.
	soonVisit, laterVisit, unscheduledVisit, pastVisit string
	base                                               time.Time
}

func newScheduleEnv(t *testing.T, staff ...fixture.StaffOption) *scheduleEnv {
	t.Helper()
	e := newEnv(t, signedInAs(staff...), withEngagement(fixture.NewClient(fixture.ClientName("Ada Client"))))
	f := &scheduleEnv{
		env:           e,
		engagementA:   e.Engagement.ID,
		clientA:       e.Engagement.Client.ID,
		engagementB:   e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Bea Client"))).ID,
		employeeDoula: e.Colleague(),
		otherDoula:    e.Colleague(fixture.StaffName("Other Doula")),
		base:          time.Now().UTC().Add(time.Hour).Truncate(time.Second),
	}
	f.soonVisit = seedScheduledVisit(t, e.DB, f.engagementA, f.employeeDoula.ID, f.base)
	f.laterVisit = seedScheduledVisit(t, e.DB, f.engagementB, f.otherDoula.ID, f.base.Add(48*time.Hour))
	f.unscheduledVisit = seedVisit(t, e.DB, f.engagementA, f.employeeDoula.ID)
	f.pastVisit = seedScheduledVisit(t, e.DB, f.engagementA, f.employeeDoula.ID, f.base.Add(-72*time.Hour))
	return f
}

// window is the from and to query for a window of the given bounds.
func window(from, to time.Time) url.Values {
	return url.Values{fromParameter: {from.Format(time.RFC3339)}, toParameter: {to.Format(time.RFC3339)}}
}

// TestPracticeSchedule_ListsEveryScheduledVisitSoonestFirst is the first
// acceptance criterion: one view, across every Client and every Doula,
// ordered by when the Visit happens.
func TestPracticeSchedule_ListsEveryScheduledVisitSoonestFirst(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	// #2114: the schedule names each Client in full.
	testdb.NameClient(t, f.DB, f.clientA, "Ada", "Okafor")

	page := f.readSchedule(nil)
	if got := visitIDs(page); !slices.Equal(got, []string{f.soonVisit, f.laterVisit}) {
		t.Fatalf("schedule = %v, want [%s %s]", got, f.soonVisit, f.laterVisit)
	}

	first := page.Items[0]
	if first.EngagementID != f.engagementA {
		t.Errorf("engagementId = %q, want %q", first.EngagementID, f.engagementA)
	}
	if first.ClientName != "Ada Okafor" {
		t.Errorf("clientName = %q, want %q", first.ClientName, "Ada Okafor")
	}
	if first.StaffID != f.employeeDoula.ID {
		t.Errorf("staffId = %q, want %q", first.StaffID, f.employeeDoula.ID)
	}
	if first.StaffName == "" {
		t.Error("staffName is empty; the row has to name the assigned Doula")
	}
	if !first.ScheduledAt.Equal(f.base) {
		t.Errorf("scheduledAt = %v, want %v", first.ScheduledAt, f.base)
	}
}

// TestPracticeSchedule_OmitsAVisitWithNoScheduledInstant is the third
// acceptance criterion: an unscheduled Visit has no place on a schedule.
// The listing test above already proves it is absent from the default
// window; this one proves it is absent from a window wide enough to hold
// every Visit at the Practice, so "absent" is the predicate rather than
// an artifact of the range.
func TestPracticeSchedule_OmitsAVisitWithNoScheduledInstant(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	page := f.readSchedule(window(f.base.Add(-365*24*time.Hour), f.base.Add(365*24*time.Hour)))

	for _, item := range page.Items {
		if item.VisitID == f.unscheduledVisit {
			t.Fatalf("unscheduled Visit %q appeared on the schedule", f.unscheduledVisit)
		}
	}
	if len(page.Items) != 3 {
		t.Fatalf("wide window = %d items, want 3 (the two upcoming and the past one)", len(page.Items))
	}
}

// TestPracticeSchedule_NarrowsToADateRange is half of the fourth
// criterion: the caller's own window replaces the default one, in both
// directions.
func TestPracticeSchedule_NarrowsToADateRange(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	// A window that starts before the past Visit reaches it; the default
	// one, which starts at "now", never does.
	past := f.readSchedule(window(f.base.Add(-96*time.Hour), f.base))
	if got := visitIDs(past); !slices.Equal(got, []string{f.pastVisit}) {
		t.Fatalf("backward window = %v, want [%s]", got, f.pastVisit)
	}

	// `to` is exclusive: a window ending exactly on the later Visit's
	// instant holds the soon one and not the later one.
	upToLater := f.readSchedule(window(f.base, f.base.Add(48*time.Hour)))
	if got := visitIDs(upToLater); !slices.Equal(got, []string{f.soonVisit}) {
		t.Fatalf("exclusive upper bound = %v, want [%s]", got, f.soonVisit)
	}
}

// TestPracticeSchedule_NarrowsToOneDoula is the other half of the fourth
// criterion.
func TestPracticeSchedule_NarrowsToOneDoula(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	page := f.readSchedule(url.Values{staffParameter: {f.otherDoula.ID}})

	if got := visitIDs(page); !slices.Equal(got, []string{f.laterVisit}) {
		t.Fatalf("narrowed to one Doula = %v, want [%s]", got, f.laterVisit)
	}
}

// TestPracticeSchedule_RefusesMalformedNarrowing proves each narrowing
// input is validated rather than silently ignored -- a mistyped range
// that quietly returned the default window would be a lie about what the
// screen is showing.
func TestPracticeSchedule_RefusesMalformedNarrowing(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())

	cases := []struct {
		name   string
		params url.Values
	}{
		{"unparseable from", url.Values{fromParameter: {"yesterday"}}},
		{"unparseable to", url.Values{toParameter: {"soon"}}},
		{"empty range", window(f.base, f.base)},
		{"inverted range", window(f.base, f.base.Add(-time.Hour))},
		{"non-uuid staffId", url.Values{staffParameter: {notAUUID}}},
		{"undecodable cursor", url.Values{"cursor": {"not!valid!base64!"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodGet, f.scheduleURL(tc.params), http.StatusBadRequest, f.SignedIn)
		})
	}
}

// TestPracticeSchedule_ReadFollowsTheRole is the fifth criterion, all
// four cases in one table: an Owner, an Admin and an employee Doula each
// read every scheduled Visit at the Practice; a contractor Doula reads
// only the ones on Engagements she holds an open, granted attachment on.
// Every case is a 200 -- the contractor's narrowing is a shorter list,
// not a refusal -- and it is enforced by the endpoint, which is what
// makes it more than a hidden control.
func TestPracticeSchedule_ReadFollowsTheRole(t *testing.T) {
	cases := []struct {
		name     string
		staff    []fixture.StaffOption
		attached bool
		wantAll  bool
	}{
		{"owner reads every Visit", []fixture.StaffOption{fixture.Owner()}, false, true},
		{"admin reads every Visit", []fixture.StaffOption{fixture.Admin()}, false, true},
		{"employee doula reads every Visit", []fixture.StaffOption{fixture.Doula()}, false, true},
		{"contractor doula reads only what she is attached to", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newScheduleEnv(t, tc.staff...)
			if tc.attached {
				testdb.SeedGrantedAttachment(t, f.DB, f.engagementA, f.Staff.ID)
			}

			want := []string{f.soonVisit, f.laterVisit}
			if !tc.wantAll {
				// engagementA only: the Engagement she is attached to.
				want = []string{f.soonVisit}
			}
			if got := visitIDs(f.readSchedule(nil)); !slices.Equal(got, want) {
				t.Fatalf("schedule = %v, want %v", got, want)
			}
		})
	}
}

// TestPracticeSchedule_RefusesAnUnattachedContractorEveryRow is the
// contractor rule at its sharpest: with no attachment at all she reads an
// empty schedule rather than the Practice's book.
func TestPracticeSchedule_RefusesAnUnattachedContractorEveryRow(t *testing.T) {
	f := newScheduleEnv(t, fixture.Doula(), fixture.Contractor())

	if page := f.readSchedule(nil); len(page.Items) != 0 {
		t.Fatalf("unattached contractor read %d Visits, want 0", len(page.Items))
	}
}

// TestPracticeSchedule_ExcludesAnotherPracticesVisits proves the join to
// engagements really is the Practice filter, on top of RLS.
func TestPracticeSchedule_ExcludesAnotherPracticesVisits(t *testing.T) {
	f := newScheduleEnv(t, fixture.Owner())
	other := fixture.Practice(t, f.DB)
	foreignVisit := seedScheduledVisit(t, f.DB, fixture.Engagement(t, f.DB, other).ID, fixture.Staff(t, f.DB, other).ID, f.base)

	for _, item := range f.readSchedule(nil).Items {
		if item.VisitID == foreignVisit {
			t.Fatalf("another Practice's Visit %q appeared on the schedule", foreignVisit)
		}
	}
}

// TestPracticeSchedule_PagesSoonestFirst walks the cursor across a full
// page boundary, proving the ascending cursor comparison and that a page
// is bounded rather than unlimited.
func TestPracticeSchedule_PagesSoonestFirst(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	const total = 31 // schedulePageSize (30) + 1, to force a second page
	seeded := make([]string, 0, total)
	for i := range total {
		seeded = append(seeded, seedScheduledVisit(t, e.DB, e.Engagement.ID, e.Staff.ID,
			base.Add(time.Duration(i)*time.Minute)))
	}

	first := e.readSchedule(nil)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}
	if !slices.Equal(visitIDs(first), seeded[:30]) {
		t.Fatalf("first page order = %v, want %v", visitIDs(first), seeded[:30])
	}

	second := e.readSchedule(url.Values{"cursor": {*first.NextCursor}})
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].VisitID != seeded[30] {
		t.Fatalf("second page = %q, want %q", second.Items[0].VisitID, seeded[30])
	}
}
