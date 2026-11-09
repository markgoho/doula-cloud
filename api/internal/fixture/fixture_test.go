package fixture_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func TestPractice_DefaultsAndOptions(t *testing.T) {
	db := testdb.New(t)

	plain := fixture.Practice(t, db)
	named := fixture.Practice(t, db, fixture.PracticeName("Birth Right"), fixture.Credits(3))

	if plain.Name != "Test Practice" || plain.ID == named.ID {
		t.Fatalf("plain = %+v, named = %+v, want distinct Practices, the first named by default", plain, named)
	}
	if got := testdb.Scalar[string](t, db.Admin, `SELECT name FROM practices WHERE id = $1`, named.ID); got != "Birth Right" {
		t.Fatalf("name = %q, want Birth Right", got)
	}
	balance := `SELECT coalesce(sum(quantity), 0) FROM credit_ledger WHERE practice_id = $1`
	if got := testdb.Scalar[int](t, db.Admin, balance, named.ID); got != 3 {
		t.Fatalf("credits = %d, want 3", got)
	}
	if got := testdb.Scalar[int](t, db.Admin, balance, plain.ID); got != 0 {
		t.Fatalf("default credits = %d, want 0", got)
	}
}

func TestStaff_DefaultsToAnEmployeeDoula(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	s := fixture.Staff(t, db, p)
	other := fixture.Staff(t, db, p)

	if s.UID == other.UID || s.ID == other.ID {
		t.Fatalf("staff = %+v and %+v, want distinct identities", s, other)
	}
	roles := testdb.Scalar[string](t, db.Admin,
		`SELECT roles::text FROM practice_memberships WHERE staff_id = $1 AND practice_id = $2`, s.ID, p.ID)
	if roles != "{doula}" || s.Employment != "employee" || s.PracticeID != p.ID {
		t.Fatalf("roles = %s, record = %+v, want an employee Doula at the Practice", roles, s)
	}
}

func TestStaff_OptionsShowTheState(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	s := fixture.Staff(t, db, p,
		fixture.Owner(), fixture.Admin(), fixture.Doula(), fixture.Contractor(),
		fixture.StaffName("Renata Alvarez"), fixture.StaffUID("renata"))

	if s.UID != "renata" || !slices.Equal(s.Roles, []string{fixture.RoleOwner, fixture.RoleAdmin, fixture.RoleDoula}) || s.Employment != "contractor" {
		t.Fatalf("record = %+v, want Renata as a contractor Owner, Admin and Doula", s)
	}
	first := testdb.Scalar[string](t, db.Admin, `SELECT first_name || ' ' || last_name FROM staff WHERE id = $1`, s.ID)
	employment := testdb.Scalar[string](t, db.Admin, `SELECT employment_type::text FROM practice_memberships WHERE staff_id = $1`, s.ID)
	if first != "Renata Alvarez" || employment != "contractor" {
		t.Fatalf("stored name = %q, employment = %q", first, employment)
	}
}

func TestStaff_RolesReplacesWhatEarlierOptionsAdded(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	s := fixture.Staff(t, db, p, fixture.Owner(), fixture.Roles(fixture.RoleAdmin, fixture.RoleDoula))

	if !slices.Equal(s.Roles, []string{fixture.RoleAdmin, fixture.RoleDoula}) {
		t.Fatalf("roles = %v, want admin and doula", s.Roles)
	}
}

func TestStaff_NoRoleHoldsNoRole(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	s := fixture.Staff(t, db, p, fixture.NoRole())

	if got := testdb.Scalar[int](t, db.Admin, `SELECT cardinality(roles) FROM practice_memberships WHERE staff_id = $1`, s.ID); got != 0 {
		t.Fatalf("roles held = %d, want none", got)
	}
}

func TestStaff_FounderHoldsOwnerAdminAndDoula(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	s := fixture.Staff(t, db, p, fixture.Founder())

	if !slices.Equal(s.Roles, []string{fixture.RoleOwner, fixture.RoleAdmin, fixture.RoleDoula}) {
		t.Fatalf("roles = %v, want owner, admin and doula", s.Roles)
	}
}

func TestClient_DefaultsAndOptions(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	a := fixture.Client(t, db, p)
	b := fixture.Client(t, db, p, fixture.ClientName("Hannah Sorensen"), fixture.ClientEmail("hannah@example.com"))
	none := fixture.Client(t, db, p, fixture.NoEmail())

	if a.Name != "Test Client" || a.Email == "" || a.Email == none.Email || a.PracticeID != p.ID {
		t.Fatalf("default client = %+v", a)
	}
	if got := testdb.Scalar[string](t, db.Admin, `SELECT given_name || ' ' || email FROM clients WHERE id = $1`, b.ID); got != "Hannah Sorensen hannah@example.com" {
		t.Fatalf("stored client = %q", got)
	}
	if testdb.Scalar[bool](t, db.Admin, `SELECT email IS NOT NULL FROM clients WHERE id = $1`, none.ID) {
		t.Fatal("NoEmail stored an address, want NULL")
	}
}

func TestEngagement_DefaultsToAnIntakeBirthForANewClient(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	e := fixture.Engagement(t, db, p)

	if e.Kind != fixture.KindBirth || e.Status != fixture.StatusIntake || e.Client.ID == "" || e.PracticeID != p.ID {
		t.Fatalf("engagement = %+v, want an intake birth for a new Client", e)
	}
	got := testdb.Scalar[string](t, db.Admin,
		`SELECT client_id::text || kind::text || status::text FROM engagements WHERE id = $1`, e.ID)
	if got != e.Client.ID+"birthintake" {
		t.Fatalf("stored = %q", got)
	}
}

func TestEngagement_NewClientTakesTheClientOptions(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)

	e := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Tess"), fixture.NoEmail()))

	if e.Client.Name != "Tess" || e.Client.Email != "" {
		t.Fatalf("client = %+v, want Tess with no address", e.Client)
	}
}

func TestEngagement_OptionsShowTheState(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	c := fixture.Client(t, db, p)
	d1 := fixture.Staff(t, db, p)
	d2 := fixture.Staff(t, db, p)

	e := fixture.Engagement(t, db, p,
		fixture.ForClient(c), fixture.Kind(fixture.KindPostpartum), fixture.Status(fixture.StatusCompleted),
		fixture.WithDoula(d1), fixture.WithDoula(d2), fixture.DueDate("2026-10-30"))

	if due := testdb.Scalar[string](t, db.Admin, `SELECT due_date::text FROM engagements WHERE id = $1`, e.ID); due != "2026-10-30" {
		t.Fatalf("due date = %q, want 2026-10-30", due)
	}
	if e.Client.ID != c.ID {
		t.Fatalf("client = %q, want the one passed in, %q", e.Client.ID, c.ID)
	}
	stored := testdb.Scalar[string](t, db.Admin,
		`SELECT kind::text || '/' || status::text || '/' || coalesce(ending_reason::text, '') FROM engagements WHERE id = $1`, e.ID)
	if stored != "postpartum/completed/care_complete" {
		t.Fatalf("stored = %q, want a completed postpartum Engagement with its ending reason", stored)
	}
	attached := testdb.Column[string](t, db.Admin,
		`SELECT staff_id::text FROM engagement_attachments WHERE engagement_id = $1 AND origin = 'granted' AND ended_at IS NULL ORDER BY staff_id`, e.ID)
	want := []string{d1.ID, d2.ID}
	slices.Sort(want)
	if !slices.Equal(attached, want) {
		t.Fatalf("attachments = %v, want %v", attached, want)
	}
}

func TestSession_SignsAStaffMemberAndAClientIn(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	s := fixture.Staff(t, db, p)
	c := fixture.Client(t, db, p, fixture.WithPortalLogin())

	staffToken := fixture.Session(t, db, s)
	clientToken := fixture.Session(t, db, c)

	if authntest.CountFor(t, db.App, s.UID) != 1 || authntest.CountFor(t, db.App, c.PortalUID) != 1 {
		t.Fatalf("sessions: staff %d, client %d, want one each",
			authntest.CountFor(t, db.App, s.UID), authntest.CountFor(t, db.App, c.PortalUID))
	}
	var seen string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /staff", func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db.App, authn.TierStaff)
		if !ok {
			return
		}
		_ = tx.Rollback()
		seen = uid
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /portal", func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db.App, authn.TierPortal)
		if !ok {
			return
		}
		_ = tx.Rollback()
		seen = uid
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := reqtest.Get(t, srv.URL+"/staff", reqtest.Session(staffToken))
	reqtest.RequireStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	if seen != s.UID {
		t.Fatalf("staff identity = %q, want %q", seen, s.UID)
	}

	resp = reqtest.Get(t, srv.URL+"/portal", reqtest.Session(clientToken))
	reqtest.RequireStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	if seen != c.PortalUID {
		t.Fatalf("client identity = %q, want %q", seen, c.PortalUID)
	}
}

func TestRefusesEngagementsItCannotReach_ChecksBothRefusals(t *testing.T) {
	db := testdb.New(t)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /engagements/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "not-a-uuid" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fixture.RefusesEngagementsItCannotReach(t, db, http.MethodPut, func(id string) string { return srv.URL + "/engagements/" + id })
}
