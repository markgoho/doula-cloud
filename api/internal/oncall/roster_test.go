package oncall_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// rosterEnv is one Practice holding several births, the shape the
// contractor criterion is about: she is on one Engagement, and the
// Practice holds several she is not on. Ada is the contractor's; Bea,
// Cleo and Dot (no due date) are the employee's. Every due date puts its
// 37-week window over October 2026.
type rosterEnv struct {
	*env
	Contractor, Employee fixture.StaffRecord
	Ada, Bea, Cleo, Dot  fixture.EngagementRecord
	BeaVisit             string
}

func newRosterEnv(t *testing.T) rosterEnv {
	t.Helper()
	e := rosterEnv{env: newBareEnv(t)}
	e.Employee = fixture.Staff(t, e.db, e.Practice)
	e.Contractor = fixture.Staff(t, e.db, e.Practice, fixture.Contractor())
	birth := func(name, due string, doula fixture.StaffRecord) fixture.EngagementRecord {
		opts := []fixture.EngagementOption{fixture.NewClient(fixture.ClientName(name)), fixture.Status(fixture.StatusActive), fixture.WithDoula(doula)}
		if due != "" {
			opts = append(opts, fixture.DueDate(due))
		}
		return fixture.Engagement(t, e.db, e.Practice, opts...)
	}
	e.Ada = birth("Ada Whitfield", birthDue, e.Contractor)
	e.Bea = birth("Bea Okafor", "2026-10-28", e.Employee)
	e.Cleo = birth("Cleo Marsh", "2026-11-02", e.Employee)
	e.Dot = birth("Dot Nakamura", "", e.Employee)

	e.BeaVisit = testdb.Scalar[string](t, e.db.Admin,
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id`, e.Bea.ID, e.Employee.ID)

	// The employee cannot be reached one evening on Bea's birth.
	e.exec(`INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, reason, created_by)
		VALUES ($1, $2, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', 'Sister''s wedding', $2)`,
		e.Bea.ID, e.Employee.ID)
	return e
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
	e := newRosterEnv(t)
	// #2114: the roster names each Client in full, her preferred name
	// followed by the family name.
	e.exec(`UPDATE clients SET preferred_name = 'Addie', family_name = 'Brennan' WHERE id = $1`, e.Ada.Client.ID)
	url := e.rosterURL(octFirst, octLast)
	asContractor := e.as(e.Contractor)

	body := string(reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, asContractor))

	for _, leak := range []string{
		"Bea", "Okafor", "Cleo", "Marsh", "Dot", "Nakamura",
		"2026-10-28", "2026-11-02", "wedding",
		e.Bea.ID, e.Cleo.ID, e.Dot.ID, e.BeaVisit,
	} {
		if strings.Contains(body, leak) {
			t.Errorf("the contractor's roster carries %q, a fact of an Engagement she is not on", leak)
		}
	}

	roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, url, http.StatusOK, asContractor)
	if got := windowIDs(roster); !slices.Equal(got, []string{e.Ada.ID}) {
		t.Fatalf("windows = %v, want only her own %s", got, e.Ada.ID)
	}
	if roster.Windows[0].ClientName != "Addie Brennan" {
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
	e := newRosterEnv(t)

	roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, e.rosterURL("2026-10-17", "2026-10-17"), http.StatusOK, e.as(e.Contractor))

	byID := map[string]oncall.RosterDoula{}
	for _, d := range roster.Doulas {
		byID[d.StaffID] = d
	}
	colleague, ok := byID[e.Employee.ID]
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
	self := byID[e.Contractor.ID]
	if self.ConcurrentWindows == nil || *self.ConcurrentWindows != 1 {
		t.Errorf("her own count = %v, want 1", self.ConcurrentWindows)
	}
}

// TestRoster_AmbientReadersReadTheWholePractice: an Owner, an Admin and
// an employee Doula each read every window and every no-window row.
func TestRoster_AmbientReadersReadTheWholePractice(t *testing.T) {
	e := newRosterEnv(t)
	readers := map[string]fixture.StaffRecord{
		"an Owner":          fixture.Staff(t, e.db, e.Practice, fixture.Owner()),
		"an Admin":          fixture.Staff(t, e.db, e.Practice, fixture.Admin()),
		"an employee Doula": e.Employee,
	}

	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, e.rosterURL(octFirst, octLast), http.StatusOK, e.as(reader))

			want := sorted(e.Ada.ID, e.Bea.ID, e.Cleo.ID)
			if got := windowIDs(roster); !slices.Equal(got, want) {
				t.Fatalf("windows = %v, want %v", got, want)
			}
			if len(roster.NoWindow) != 1 || roster.NoWindow[0].EngagementID != e.Dot.ID ||
				roster.NoWindow[0].Reason != oncall.NoWindowNoDueDate {
				t.Fatalf("noWindow = %+v, want Dot's Engagement with reason no_due_date", roster.NoWindow)
			}
			for _, d := range roster.Doulas {
				if d.ConcurrentWindows == nil {
					t.Fatalf("%s has no count; an ambient reader holds every count", d.Name)
				}
				if d.StaffID == e.Employee.ID && *d.ConcurrentWindows != 2 {
					t.Errorf("the employee's count = %d, want 2 concurrent windows (Bea, Cleo)", *d.ConcurrentWindows)
				}
			}
		})
	}
}
