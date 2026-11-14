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
	e := rosterEnv{env: newBareEnv(t, fixture.NobodySignedIn())}
	e.Employee = fixture.Staff(t, e.DB, e.Practice)
	e.Contractor = fixture.Staff(t, e.DB, e.Practice, fixture.Contractor())
	birth := func(name, due string, doula fixture.StaffRecord) fixture.EngagementRecord {
		opts := []fixture.EngagementOption{fixture.NewClient(fixture.ClientName(name)), fixture.Status(fixture.StatusActive), fixture.WithDoula(doula)}
		if due != "" {
			opts = append(opts, fixture.DueDate(due))
		}
		return fixture.Engagement(t, e.DB, e.Practice, opts...)
	}
	e.Ada = birth(adaWhitfield, birthDue, e.Contractor)
	e.Bea = birth("Bea Okafor", "2026-10-28", e.Employee)
	e.Cleo = birth("Cleo Marsh", "2026-11-02", e.Employee)
	e.Dot = birth("Dot Nakamura", "", e.Employee)

	e.BeaVisit = testdb.Scalar[string](t, e.DB.Admin,
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id`, e.Bea.ID, e.Employee.ID)

	// The employee cannot be reached one evening on Bea's birth.
	e.Exec(`INSERT INTO engagement_coverage_gaps (engagement_id, staff_id, starts_at, ends_at, reason, created_by)
		VALUES ($1, $2, '2026-10-17T22:00:00Z', '2026-10-18T10:00:00Z', 'Sister''s wedding', $2)`,
		e.Bea.ID, e.Employee.ID)
	return e
}

// othersFacts are the names and ids of the three births a contractor is
// not on, none of which may reach her.
func (e rosterEnv) othersFacts() []string {
	return []string{"Bea", "Cleo", "Dot", e.Bea.ID, e.Cleo.ID, e.Dot.ID}
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
	e.Exec(`UPDATE clients SET preferred_name = 'Addie', family_name = 'Brennan' WHERE id = $1`, e.Ada.Client.ID)
	url := e.rosterURL(octFirst, octLast)
	asContractor := e.As(e.Contractor)

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

// TestRoster_ContractorSeesOnlyTheColleaguesOnHerBirths: a colleague who is
// on none of her births is not in her Doula list at all, and one who is on
// one of them reads there by her part in that birth alone -- no other birth
// of hers, no count, no overlap, no gap on another Client.
func TestRoster_ContractorSeesOnlyTheColleaguesOnHerBirths(t *testing.T) {
	e := newRosterEnv(t)
	url := e.rosterURL("2026-10-17", "2026-10-17")
	asContractor := e.As(e.Contractor)

	roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, url, http.StatusOK, asContractor)

	if len(roster.Doulas) != 1 || roster.Doulas[0].StaffID != e.Contractor.ID {
		t.Fatalf("doulas = %+v, want only herself while no colleague is on her births", roster.Doulas)
	}
	self := roster.Doulas[0]
	if len(self.Births) != 1 || self.Births[0].EngagementID != e.Ada.ID || self.Births[0].Covering {
		t.Errorf("her births = %+v, want Ada's, not covering", self.Births)
	}

	testdb.SeedGrantedAttachment(t, e.DB, e.Ada.ID, e.Employee.ID)
	roster = reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, url, http.StatusOK, asContractor)

	byID := map[string]oncall.RosterDoula{}
	for _, d := range roster.Doulas {
		byID[d.StaffID] = d
	}
	colleague, ok := byID[e.Employee.ID]
	if !ok {
		t.Fatal("a colleague on her birth is missing from the Doula list")
	}
	if len(colleague.Births) != 1 || colleague.Births[0].EngagementID != e.Ada.ID {
		t.Errorf("the colleague's births = %+v, want only the one that is also hers", colleague.Births)
	}
	if len(colleague.Unreachable) != 0 {
		t.Errorf("the colleague's gap on another Client's birth reached her: %+v", colleague.Unreachable)
	}
	if colleague.AtOnce != nil {
		t.Errorf("the colleague's overlaps reached her: %+v", colleague.AtOnce)
	}
	body := string(reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, asContractor))
	for _, leak := range append(e.othersFacts(), "wedding") {
		if strings.Contains(body, leak) {
			t.Errorf("the contractor's roster carries %q, a fact of a birth she is not on", leak)
		}
	}
}

// TestRoster_AContractorNeverReadsABirthWithNoDoula: a birth nobody is on
// has no Attachment she could hold, so it is neither a row nor a
// no-window entry for her, and an Owner reads it as one.
func TestRoster_AContractorNeverReadsABirthWithNoDoula(t *testing.T) {
	e := newRosterEnv(t)
	orphan := e.birthDue("Orla Unstaffed", "2026-10-20")
	url := e.rosterURL(octFirst, octLast)

	body := string(reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, e.As(e.Contractor)))

	if strings.Contains(body, "Orla") || strings.Contains(body, orphan.ID) {
		t.Errorf("the contractor's roster carries a birth with no Doula: %s", body)
	}
	owner := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, url, http.StatusOK, e.As(fixture.Staff(t, e.DB, e.Practice, fixture.Owner())))
	if !slices.Contains(windowIDs(owner), orphan.ID) {
		t.Errorf("an Owner's windows = %v, want the birth with no Doula among them", windowIDs(owner))
	}
}

// TestRoster_StaffIDOnlyNarrowsAContractorsOwnAnswer: naming a colleague
// who is on a birth she cannot read, or on none, never widens what she
// reads; naming herself or a colleague on her birth only narrows.
func TestRoster_StaffIDOnlyNarrowsAContractorsOwnAnswer(t *testing.T) {
	e := newRosterEnv(t)
	asContractor := e.As(e.Contractor)

	for name, staffID := range map[string]string{"a colleague on others' births": e.Employee.ID, "a stranger": "00000000-0000-0000-0000-000000000000", "herself": e.Contractor.ID} {
		t.Run(name, func(t *testing.T) {
			url := e.PracticeURL("/on-call?from=" + octFirst + "&to=" + octLast + "&staffId=" + staffID)
			body := string(reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, asContractor))
			roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, url, http.StatusOK, asContractor)

			for _, leak := range e.othersFacts() {
				if strings.Contains(body, leak) {
					t.Errorf("staffId widened her answer to %q", leak)
				}
			}
			for _, id := range windowIDs(roster) {
				if id != e.Ada.ID {
					t.Errorf("windows = %v, want only her own", windowIDs(roster))
				}
			}
			if staffID == e.Contractor.ID && len(roster.Windows) != 1 {
				t.Errorf("windows = %v, want her own birth for herself", windowIDs(roster))
			}
			if staffID != e.Contractor.ID && len(roster.Windows) != 0 {
				t.Errorf("windows = %v, want none for a Doula who is not on her birth", windowIDs(roster))
			}
		})
	}
}

// TestRoster_AmbientReadersReadTheWholePractice: an Owner, an Admin and
// an employee Doula each read every window and every no-window row.
func TestRoster_AmbientReadersReadTheWholePractice(t *testing.T) {
	e := newRosterEnv(t)
	readers := map[string]fixture.StaffRecord{
		"an Owner":          fixture.Staff(t, e.DB, e.Practice, fixture.Owner()),
		"an Admin":          fixture.Staff(t, e.DB, e.Practice, fixture.Admin()),
		"an employee Doula": e.Employee,
	}

	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			roster := reqtest.Expect[oncall.RosterResponse](t, http.MethodGet, e.rosterURL(octFirst, octLast), http.StatusOK, e.As(reader))

			want := sorted(e.Ada.ID, e.Bea.ID, e.Cleo.ID)
			if got := windowIDs(roster); !slices.Equal(got, want) {
				t.Fatalf("windows = %v, want %v", got, want)
			}
			if len(roster.NoWindow) != 1 || roster.NoWindow[0].EngagementID != e.Dot.ID ||
				roster.NoWindow[0].Reason != oncall.NoWindowNoDueDate {
				t.Fatalf("noWindow = %+v, want Dot's Engagement with reason no_due_date", roster.NoWindow)
			}
			byID := map[string]oncall.RosterDoula{}
			for _, d := range roster.Doulas {
				byID[d.StaffID] = d
			}
			if got := len(byID[e.Employee.ID].Births); got != 2 {
				t.Errorf("the employee's births = %d, want 2 (Bea, Cleo)", got)
			}
			if got := len(byID[e.Contractor.ID].Births); got != 1 {
				t.Errorf("the contractor's births = %d, want 1 (Ada)", got)
			}
		})
	}
}
