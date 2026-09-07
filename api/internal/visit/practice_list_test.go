package visit_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"

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

// schedulePath builds the Practice-wide schedule URL with whatever
// narrowing a test wants, so no test has to hand-assemble a query string.
func schedulePath(srvURL, practiceID string, params url.Values) string {
	path := srvURL + "/api/practices/" + practiceID + "/visits"
	if len(params) == 0 {
		return path
	}
	return path + "?" + params.Encode()
}

// getSchedule performs the read and decodes the envelope, failing the
// test on any status other than 200.
func getSchedule(t *testing.T, session, requestURL string) visit.PracticeScheduleResponse {
	t.Helper()
	resp := getScheduleRaw(t, session, requestURL)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var page visit.PracticeScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("decode schedule: %v", err)
	}
	return page
}

func getScheduleRaw(t *testing.T, session, requestURL string) *http.Response {
	t.Helper()
	return authedGet(t, session, requestURL)
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

func equalIDs(got, want []string) bool {
	return slices.Equal(got, want)
}

// scheduleFixture is one Practice with two Clients, three Staff and four
// Visits -- the smallest shape that can tell every acceptance criterion
// apart: a Visit with no scheduled instant, a Visit outside the window,
// two Doulas to filter between, and one Engagement a contractor is
// attached to.
type scheduleFixture struct {
	db         *testdb.DB
	practiceID string
	// engagementA is the Engagement the contractor holds an open, granted
	// attachment on; engagementB is the one she does not.
	engagementA, engagementB string
	employeeDoulaID          string
	otherDoulaID             string
	// soon and later are both inside the default window; unscheduledVisit
	// has no instant at all and past is behind it.
	soonVisit, laterVisit, unscheduledVisit, pastVisit string
	base                                               time.Time
}

func newScheduleFixture(t *testing.T, db *testdb.DB, prefix string) scheduleFixture {
	t.Helper()
	practiceID, employeeDoulaID := testdb.SeedStaffAtNewPractice(
		t, db, prefix+"-employee-doula", []string{doulaRole}, employeeType)
	otherDoulaID := testdb.SeedNamedStaffAtPractice(
		t, db, practiceID, prefix+"-other-doula", "Other Doula", []string{doulaRole}, employeeType)

	_, engagementA := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Client", prefix+"-ada@example.com")
	_, engagementB := testdb.SeedNamedEngagement(t, db, practiceID, "Bea Client", prefix+"-bea@example.com")

	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	f := scheduleFixture{
		db:              db,
		practiceID:      practiceID,
		engagementA:     engagementA,
		engagementB:     engagementB,
		employeeDoulaID: employeeDoulaID,
		otherDoulaID:    otherDoulaID,
		base:            base,
	}
	f.soonVisit = seedScheduledVisit(t, db, engagementA, employeeDoulaID, base)
	f.laterVisit = seedScheduledVisit(t, db, engagementB, otherDoulaID, base.Add(48*time.Hour))
	f.unscheduledVisit = seedVisit(t, db, engagementA, employeeDoulaID)
	f.pastVisit = seedScheduledVisit(t, db, engagementA, employeeDoulaID, base.Add(-72*time.Hour))
	return f
}

// TestPracticeSchedule_ListsEveryScheduledVisitSoonestFirst is the first
// acceptance criterion: one view, across every Client and every Doula,
// ordered by when the Visit happens.
func TestPracticeSchedule_ListsEveryScheduledVisitSoonestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-owner"
	f := newScheduleFixture(t, db, "listing")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, nil))
	if got := visitIDs(page); !equalIDs(got, []string{f.soonVisit, f.laterVisit}) {
		t.Fatalf("schedule = %v, want [%s %s]", got, f.soonVisit, f.laterVisit)
	}

	first := page.Items[0]
	if first.EngagementID != f.engagementA {
		t.Errorf("engagementId = %q, want %q", first.EngagementID, f.engagementA)
	}
	if first.ClientName != "Ada Client" {
		t.Errorf("clientName = %q, want %q", first.ClientName, "Ada Client")
	}
	if first.StaffID != f.employeeDoulaID {
		t.Errorf("staffId = %q, want %q", first.StaffID, f.employeeDoulaID)
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
	db := testdb.New(t)
	const identityUID = "schedule-omits-unscheduled"
	f := newScheduleFixture(t, db, "unscheduled")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, url.Values{
		fromParameter: {f.base.Add(-365 * 24 * time.Hour).Format(time.RFC3339)},
		toParameter:   {f.base.Add(365 * 24 * time.Hour).Format(time.RFC3339)},
	}))
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
	db := testdb.New(t)
	const identityUID = "schedule-range"
	f := newScheduleFixture(t, db, "range")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	// A window that starts before the past Visit reaches it; the default
	// one, which starts at "now", never does.
	past := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, url.Values{
		fromParameter: {f.base.Add(-96 * time.Hour).Format(time.RFC3339)},
		toParameter:   {f.base.Format(time.RFC3339)},
	}))
	if got := visitIDs(past); !equalIDs(got, []string{f.pastVisit}) {
		t.Fatalf("backward window = %v, want [%s]", got, f.pastVisit)
	}

	// `to` is exclusive: a window ending exactly on the later Visit's
	// instant holds the soon one and not the later one.
	upToLater := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, url.Values{
		fromParameter: {f.base.Format(time.RFC3339)},
		toParameter:   {f.base.Add(48 * time.Hour).Format(time.RFC3339)},
	}))
	if got := visitIDs(upToLater); !equalIDs(got, []string{f.soonVisit}) {
		t.Fatalf("exclusive upper bound = %v, want [%s]", got, f.soonVisit)
	}
}

// TestPracticeSchedule_NarrowsToOneDoula is the other half of the fourth
// criterion.
func TestPracticeSchedule_NarrowsToOneDoula(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-one-doula"
	f := newScheduleFixture(t, db, "one-doula")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, url.Values{
		staffParameter: {f.otherDoulaID},
	}))
	if got := visitIDs(page); !equalIDs(got, []string{f.laterVisit}) {
		t.Fatalf("narrowed to one Doula = %v, want [%s]", got, f.laterVisit)
	}
}

// TestPracticeSchedule_RefusesMalformedNarrowing proves each narrowing
// input is validated rather than silently ignored -- a mistyped range
// that quietly returned the default window would be a lie about what the
// screen is showing.
func TestPracticeSchedule_RefusesMalformedNarrowing(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-bad-input"
	f := newScheduleFixture(t, db, "bad-input")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	cases := []struct {
		name   string
		params url.Values
	}{
		{"unparseable from", url.Values{fromParameter: {"yesterday"}}},
		{"unparseable to", url.Values{toParameter: {"soon"}}},
		{"empty range", url.Values{
			fromParameter: {f.base.Format(time.RFC3339)},
			toParameter:   {f.base.Format(time.RFC3339)},
		}},
		{"inverted range", url.Values{
			fromParameter: {f.base.Format(time.RFC3339)},
			toParameter:   {f.base.Add(-time.Hour).Format(time.RFC3339)},
		}},
		{"non-uuid staffId", url.Values{staffParameter: {"not-a-uuid"}}},
		{"undecodable cursor", url.Values{"cursor": {"not!valid!base64!"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := getScheduleRaw(t, session, schedulePath(srv.URL, f.practiceID, tc.params))
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
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
		name           string
		roles          []string
		employmentType string
		attached       bool
		wantAll        bool
	}{
		{"owner reads every Visit", []string{ownerRole}, employeeType, false, true},
		{"admin reads every Visit", []string{adminRole}, employeeType, false, true},
		{"employee doula reads every Visit", []string{doulaRole}, employeeType, false, true},
		{"contractor doula reads only what she is attached to", []string{doulaRole}, "contractor", true, false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			identityUID := fmt.Sprintf("schedule-role-%d", i)
			f := newScheduleFixture(t, db, fmt.Sprintf("role-%d", i))
			readerID := testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, tc.roles, tc.employmentType)
			if tc.attached {
				testdb.SeedGrantedAttachment(t, db, f.engagementA, readerID)
			}

			srv, session := newServer(t, db, identityUID)
			defer srv.Close()

			page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, nil))
			want := []string{f.soonVisit, f.laterVisit}
			if !tc.wantAll {
				// engagementA only: the Engagement she is attached to.
				want = []string{f.soonVisit}
			}
			if got := visitIDs(page); !equalIDs(got, want) {
				t.Fatalf("schedule = %v, want %v", got, want)
			}
		})
	}
}

// TestPracticeSchedule_RefusesAnUnattachedContractorEveryRow is the
// contractor rule at its sharpest: with no attachment at all she reads an
// empty schedule rather than the Practice's book.
func TestPracticeSchedule_RefusesAnUnattachedContractorEveryRow(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-unattached-contractor"
	f := newScheduleFixture(t, db, "unattached")
	testdb.SeedContractorAtPractice(t, db, f.practiceID, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, nil))
	if len(page.Items) != 0 {
		t.Fatalf("unattached contractor read %d Visits, want 0", len(page.Items))
	}
}

// TestPracticeSchedule_ExcludesAnotherPracticesVisits proves the join to
// engagements really is the Practice filter, on top of RLS.
func TestPracticeSchedule_ExcludesAnotherPracticesVisits(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-cross-practice"
	f := newScheduleFixture(t, db, "cross")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, identityUID, []string{ownerRole}, employeeType)

	otherPractice, otherStaff := testdb.SeedStaffAtNewPractice(
		t, db, "cross-other-owner", []string{ownerRole}, employeeType)
	_, otherEngagement := testdb.SeedEngagement(t, db, otherPractice)
	foreignVisit := seedScheduledVisit(t, db, otherEngagement, otherStaff, f.base)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	page := getSchedule(t, session, schedulePath(srv.URL, f.practiceID, nil))
	for _, item := range page.Items {
		if item.VisitID == foreignVisit {
			t.Fatalf("another Practice's Visit %q appeared on the schedule", foreignVisit)
		}
	}
}

// TestPracticeSchedule_PagesSoonestFirst walks the cursor across a full
// page boundary, proving the ascending cursor comparison and that a page
// is bounded rather than unlimited.
func TestPracticeSchedule_PagesSoonestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "schedule-paging"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(
		t, db, identityUID, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	base := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	const total = 31 // schedulePageSize (30) + 1, to force a second page
	seeded := make([]string, 0, total)
	for i := range total {
		seeded = append(seeded, seedScheduledVisit(t, db, engagementID, staffID,
			base.Add(time.Duration(i)*time.Minute)))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	first := getSchedule(t, session, schedulePath(srv.URL, practiceID, nil))
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}
	if !equalIDs(visitIDs(first), seeded[:30]) {
		t.Fatalf("first page order = %v, want %v", visitIDs(first), seeded[:30])
	}

	second := getSchedule(t, session, schedulePath(srv.URL, practiceID, url.Values{
		"cursor": {*first.NextCursor},
	}))
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
	if second.Items[0].VisitID != seeded[30] {
		t.Fatalf("second page = %q, want %q", second.Items[0].VisitID, seeded[30])
	}
}
