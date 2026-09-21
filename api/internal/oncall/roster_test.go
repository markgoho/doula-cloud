package oncall_test

import (
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

// rosterFixture is one Practice holding several births, the shape the
// contractor criterion is about: she is on one Engagement, and the
// Practice holds several she is not on.
type rosterFixture struct {
	practiceID                      string
	contractorID, employeeID        string
	contractorUID, employeeUID      string
	adaEngagement, beaEngagement    string
	cleoEngagement, noDueEngagement string
	beaVisit                        string
}

// Ada is the contractor's; Bea, Cleo and Dot (no due date) are the
// employee's. Every due date puts its 37-week window over October 2026.
func newRosterFixture(t *testing.T, db *testdb.DB, prefix string) rosterFixture {
	t.Helper()
	f := rosterFixture{contractorUID: prefix + "-contractor", employeeUID: prefix + "-employee"}
	f.practiceID, f.employeeID = testdb.SeedStaffAtNewPractice(t, db, f.employeeUID, []string{doulaRole}, employeeType)
	testdb.SetPracticeTimezone(t, db, f.practiceID, zone)
	f.contractorID = testdb.SeedContractorAtPractice(t, db, f.practiceID, f.contractorUID)

	f.adaEngagement = seedBirth(t, db, f.practiceID, "Ada Whitfield", "2026-10-30")
	f.beaEngagement = seedBirth(t, db, f.practiceID, "Bea Okafor", "2026-10-28")
	f.cleoEngagement = seedBirth(t, db, f.practiceID, "Cleo Marsh", "2026-11-02")
	f.noDueEngagement = seedBirth(t, db, f.practiceID, "Dot Nakamura", "")

	testdb.SeedGrantedAttachment(t, db, f.adaEngagement, f.contractorID)
	testdb.SeedGrantedAttachment(t, db, f.beaEngagement, f.employeeID)
	testdb.SeedGrantedAttachment(t, db, f.cleoEngagement, f.employeeID)
	testdb.SeedGrantedAttachment(t, db, f.noDueEngagement, f.employeeID)

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id`,
		f.beaEngagement, f.employeeID,
	).Scan(&f.beaVisit); err != nil {
		t.Fatalf("seed visit: %v", err)
	}

	// The employee cannot be reached one evening on Bea's birth.
	exec(t, db, `INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, reason, created_by)
		VALUES ($1, $2, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', 'Sister''s wedding', $2)`,
		f.beaEngagement, f.employeeID)
	return f
}

func windowIDs(resp oncall.RosterResponse) []string {
	ids := []string{}
	for _, w := range resp.Windows {
		ids = append(ids, w.EngagementID)
	}
	slices.Sort(ids)
	return ids
}

func sorted(ids ...string) []string {
	slices.Sort(ids)
	return ids
}

// TestRoster_ContractorReadsOnlyHerOwnEngagements is the ticket's bold
// criterion: a contractor on one Engagement, in a Practice holding
// several, reads that Engagement and no other Client name, due date or
// Visit. Asserted against the raw body as well as the decoded one, so a
// field this test does not know about cannot carry a fact out either.
func TestRoster_ContractorReadsOnlyHerOwnEngagements(t *testing.T) {
	db := testdb.New(t)
	f := newRosterFixture(t, db, "roster-contractor")

	srv, session := newServer(t, db, f.contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, rosterURL(srv.URL, f.practiceID, octFirst, octLast))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)

	for _, leak := range []string{
		"Bea", "Okafor", "Cleo", "Marsh", "Dot", "Nakamura",
		"2026-10-28", "2026-11-02", "wedding",
		f.beaEngagement, f.cleoEngagement, f.noDueEngagement, f.beaVisit,
	} {
		if strings.Contains(body, leak) {
			t.Errorf("the contractor's roster carries %q, a fact of an Engagement she is not on", leak)
		}
	}

	roster := getJSON[oncall.RosterResponse](t, session, rosterURL(srv.URL, f.practiceID, octFirst, octLast), http.StatusOK)
	if got := windowIDs(roster); !slices.Equal(got, []string{f.adaEngagement}) {
		t.Fatalf("windows = %v, want only her own %s", got, f.adaEngagement)
	}
	if roster.Windows[0].ClientName != "Ada Whitfield" {
		t.Errorf("clientName = %q, want her own Client's name", roster.Windows[0].ClientName)
	}
	if len(roster.NoWindow) != 0 {
		t.Errorf("noWindow = %+v, want none of another Doula's Engagements", roster.NoWindow)
	}
}

// TestRoster_ContractorSeesAColleagueByNameAndAvailabilityOnly: she may
// know a colleague exists and cannot be reached, and nothing attached to
// that colleague -- not even how many births she is carrying.
func TestRoster_ContractorSeesAColleagueByNameAndAvailabilityOnly(t *testing.T) {
	db := testdb.New(t)
	f := newRosterFixture(t, db, "roster-colleague")

	srv, session := newServer(t, db, f.contractorUID)
	defer srv.Close()

	roster := getJSON[oncall.RosterResponse](t, session, rosterURL(srv.URL, f.practiceID, "2026-10-17", "2026-10-17"), http.StatusOK)

	byID := map[string]oncall.RosterDoula{}
	for _, d := range roster.Doulas {
		byID[d.StaffID] = d
	}
	colleague, ok := byID[f.employeeID]
	if !ok {
		t.Fatal("the colleague is missing from the Doula list; she may see that a colleague exists")
	}
	if colleague.Name == "" {
		t.Error("the colleague has no name")
	}
	if colleague.Available {
		t.Error("the colleague reads available on a night her gap is in force")
	}
	if colleague.ConcurrentWindows != nil {
		t.Errorf("the colleague's window count reached the contractor: %d", *colleague.ConcurrentWindows)
	}
	self := byID[f.contractorID]
	if self.ConcurrentWindows == nil || *self.ConcurrentWindows != 1 {
		t.Errorf("her own count = %v, want 1", self.ConcurrentWindows)
	}
}

// TestRoster_AmbientReadersReadTheWholePractice: an Owner, an Admin and
// an employee Doula each read every window and every no-window row.
func TestRoster_AmbientReadersReadTheWholePractice(t *testing.T) {
	db := testdb.New(t)
	f := newRosterFixture(t, db, "roster-ambient")
	testdb.SeedStaffAtPractice(t, db, f.practiceID, "roster-ambient-owner", []string{ownerRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, f.practiceID, "roster-ambient-admin", []string{adminRole}, employeeType)

	for _, uid := range []string{"roster-ambient-owner", "roster-ambient-admin", f.employeeUID} {
		t.Run(uid, func(t *testing.T) {
			srv, session := newServer(t, db, uid)
			defer srv.Close()

			roster := getJSON[oncall.RosterResponse](t, session, rosterURL(srv.URL, f.practiceID, octFirst, octLast), http.StatusOK)
			want := sorted(f.adaEngagement, f.beaEngagement, f.cleoEngagement)
			if got := windowIDs(roster); !slices.Equal(got, want) {
				t.Fatalf("windows = %v, want %v", got, want)
			}
			if len(roster.NoWindow) != 1 || roster.NoWindow[0].EngagementID != f.noDueEngagement ||
				roster.NoWindow[0].Reason != oncall.NoWindowNoDueDate {
				t.Fatalf("noWindow = %+v, want Dot's Engagement with reason no_due_date", roster.NoWindow)
			}
			for _, d := range roster.Doulas {
				if d.ConcurrentWindows == nil {
					t.Fatalf("%s has no count; an ambient reader holds every count", d.Name)
				}
				if d.StaffID == f.employeeID && *d.ConcurrentWindows != 2 {
					t.Errorf("the employee's count = %d, want 2 concurrent windows (Bea, Cleo)", *d.ConcurrentWindows)
				}
			}
		})
	}
}
