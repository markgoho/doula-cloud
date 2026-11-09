package fixture_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// whoAmI mounts GET /api/practices/{practiceId}/whoami behind staffauth.Middleware and answers
// with the instant the request was served at.
func whoAmI(g *staffauth.GatedRouter, _ *idempotency.Router, _ *sql.DB) {
	g.Get("/api/practices/{practiceId}/whoami", staffauth.AnyStaff, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(clock.Now(r.Context()).UTC().Format(time.RFC3339)))
	}))
}

func alsoThere(g *staffauth.GatedRouter, _ *idempotency.Router) {
	g.Get("/api/practices/{practiceId}/also", staffauth.AnyStaff, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
}

func TestScene_SignsTheStaffMemberInAndRefusesAnonymous(t *testing.T) {
	s := fixture.NewScene(t, whoAmI)

	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, s.SignedIn)
	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusUnauthorized)

	if s.Staff.PracticeID != s.Practice.ID || s.Staff.Employment != employeeType {
		t.Fatalf("staff = %+v, want an employee at the Practice", s.Staff)
	}
}

func TestScene_OptionsShapePracticeStaffAndClock(t *testing.T) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	s := fixture.NewScene(t, whoAmI,
		fixture.InPractice(fixture.PracticeName("Birth Right")),
		fixture.SignedInAs(fixture.Owner()),
		fixture.At(at),
		fixture.AlsoMounting(fixture.Routes(alsoThere)))

	if s.Practice.Name != "Birth Right" {
		t.Fatalf("practice = %q, want Birth Right", s.Practice.Name)
	}
	roles := testdb.Scalar[string](t, s.DB.Admin,
		`SELECT roles::text FROM practice_memberships WHERE staff_id = $1`, s.Staff.ID)
	if roles != "{owner}" {
		t.Fatalf("roles = %s, want the owner role", roles)
	}
	if got := string(reqtest.ExpectBody(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, s.SignedIn)); got != at.Format(time.RFC3339) {
		t.Fatalf("served at %q, want %q", got, at.Format(time.RFC3339))
	}
	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/also"), http.StatusNoContent, s.SignedIn)
}

func TestScene_SeedsAnEngagementOnlyWhenAsked(t *testing.T) {
	bare := fixture.NewScene(t, whoAmI)
	if bare.Engagement.ID != "" || bare.AsClient != nil {
		t.Fatalf("engagement = %+v, want none", bare.Engagement)
	}

	plain := fixture.NewScene(t, whoAmI, fixture.WithEngagement(fixture.Kind(fixture.KindPostpartum)))
	if plain.Engagement.Kind != fixture.KindPostpartum || plain.AsClient != nil {
		t.Fatalf("engagement = %+v, client session set = %t, want a postpartum Engagement and no portal login", plain.Engagement, plain.AsClient != nil)
	}
	attachments := `SELECT count(*) FROM engagement_attachments WHERE engagement_id = $1`
	if n := testdb.Scalar[int](t, plain.DB.Admin, attachments, plain.Engagement.ID); n != 0 {
		t.Fatalf("attachments = %d, want none", n)
	}

	portal := fixture.NewScene(t, whoAmI, fixture.WithClient(fixture.WithPortalLogin()), fixture.Attached())
	if portal.Engagement.Client.PortalUID == "" || portal.AsClient == nil {
		t.Fatalf("client = %+v, want a portal login and a session for her", portal.Engagement.Client)
	}
	if n := testdb.Scalar[int](t, portal.DB.Admin, attachments, portal.Engagement.ID); n != 1 {
		t.Fatalf("attachments = %d, want the Staff member's one", n)
	}
}

// TestScene_MintsSessionsAtTheFixedInstant proves a clock far from the real
// one still finds the sessions live: they are minted at the instant, for
// the Staff member at the screen and for any other person she signs in as.
func TestScene_MintsSessionsAtTheFixedInstant(t *testing.T) {
	at := time.Now().AddDate(5, 0, 0).UTC()
	s := fixture.NewScene(t, whoAmI, fixture.At(at))

	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, s.SignedIn)
	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, s.As(s.Colleague()))
}

func TestScene_NobodySignedInSeedsNoStaff(t *testing.T) {
	s := fixture.NewScene(t, whoAmI, fixture.NobodySignedIn())

	if s.Staff.ID != "" || s.SignedIn != nil {
		t.Fatalf("staff = %+v, signedIn set = %t, want neither", s.Staff, s.SignedIn != nil)
	}
	if n := testdb.Scalar[int](t, s.DB.Admin, `SELECT count(*) FROM practice_memberships WHERE practice_id = $1`, s.Practice.ID); n != 0 {
		t.Fatalf("memberships = %d, want none", n)
	}
}

func TestScene_AddsPeopleAndRoutesToThePractice(t *testing.T) {
	s := fixture.NewScene(t, whoAmI)

	colleague := s.Colleague(fixture.Admin())
	engagement := s.AnotherEngagement(fixture.Kind(fixture.KindPostpartum))
	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, s.As(colleague))

	if colleague.ID == s.Staff.ID || colleague.PracticeID != s.Practice.ID {
		t.Fatalf("colleague = %+v, want another Staff member at the Practice", colleague)
	}
	if engagement.PracticeID != s.Practice.ID || engagement.Kind != fixture.KindPostpartum {
		t.Fatalf("engagement = %+v, want a postpartum Engagement at the Practice", engagement)
	}
	if got, want := s.PracticeURL("/x"), s.Srv.URL+"/api/practices/"+s.Practice.ID+"/x"; got != want {
		t.Fatalf("PracticeURL = %q, want %q", got, want)
	}

	s.Exec(`UPDATE practices SET name = $1 WHERE id = $2`, "Renamed", s.Practice.ID)
	if got := testdb.Scalar[string](t, s.DB.Admin, `SELECT name FROM practices WHERE id = $1`, s.Practice.ID); got != "Renamed" {
		t.Fatalf("name = %q, want Renamed", got)
	}
}

func TestScene_MoveClockToServesLaterRequestsAtTheNewInstant(t *testing.T) {
	at := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)
	s := fixture.NewScene(t, whoAmI, fixture.At(at))
	serve := func(who reqtest.Option) string {
		return string(reqtest.ExpectBody(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusOK, who))
	}
	if got := serve(s.SignedIn); got != at.Format(time.RFC3339) {
		t.Fatalf("served at %q, want %q", got, at.Format(time.RFC3339))
	}

	moved := at.Add(time.Nanosecond)
	s.MoveClockTo(moved)
	if got := serve(s.SignedIn); got != moved.Format(time.RFC3339) {
		t.Fatalf("served at %q after the move, want %q", got, moved.Format(time.RFC3339))
	}

	later := at.AddDate(5, 0, 0)
	s.MoveClockTo(later)
	reqtest.ExpectStatus(t, http.MethodGet, s.PracticeURL("/whoami"), http.StatusUnauthorized, s.SignedIn)
	if got := serve(s.As(s.Staff)); got != later.Format(time.RFC3339) {
		t.Fatalf("served at %q after signing in again, want %q", got, later.Format(time.RFC3339))
	}
}

func TestServeStaff_SignsInAtTheRealOrFixedInstant(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	s := fixture.Staff(t, db, p)

	srv, session := fixture.ServeStaff(t, db, s.UID, fixture.Routes(alsoThere))
	reqtest.ExpectStatus(t, http.MethodGet, srv.URL+"/api/practices/"+p.ID+"/also", http.StatusNoContent, reqtest.Session(session))

	at := time.Now().AddDate(5, 0, 0).UTC()
	srvAt, sessionAt := fixture.ServeStaffAt(t, db, s.UID, whoAmI, at)
	got := reqtest.ExpectBody(t, http.MethodGet, srvAt.URL+"/api/practices/"+p.ID+"/whoami", http.StatusOK, reqtest.Session(sessionAt))
	if string(got) != at.Format(time.RFC3339) {
		t.Fatalf("served at %q, want %q", got, at.Format(time.RFC3339))
	}
}
