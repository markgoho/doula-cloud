package plans_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/pdftext"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// mastheadPractice is the Practice every masthead test seeds, named once so
// each assertion reads the same name the seed wrote.
const mastheadPractice = "Rooted Birth Collective"

// mastheadNow is the request clock the masthead tests run on: 02:30 UTC on
// Oct 28 is still Oct 27 in the seeded Practice's zone (America/New_York),
// so a day read in UTC, or in the reader's zone, would say the 28th.
var mastheadNow = time.Date(2026, time.October, 28, 2, 30, 0, 0, time.UTC)

const mastheadDay = "2026-10-27"

// newClockedServer mounts the plans routes behind clock.Middleware fixed at
// now, so a write's changed_at is the request clock's instant, not the
// database's.
func newClockedServer(t *testing.T, db *testdb.DB, uid string, now time.Time) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	plans.Mount(g, ir, db.App)
	return httptest.NewServer(clock.Middleware(func() time.Time { return now })(mux)), authntest.SeedSession(t, db.App, uid)
}

// seedMastheadClient seeds a Client whose legal name (given and family),
// first name (preferred) and full name all differ, at mastheadPractice.
func seedMastheadClient(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()
	clientID, engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Johanna", "hannah@example.com")
	testdb.NameClient(t, db, clientID, "Hannah", "Sorensen")
	return clientID, engagementID
}

// setChangedAt pins a seeded Plan Instance's changed_at, standing in for
// the save that last changed it.
func setChangedAt(t *testing.T, db *testdb.DB, engagementID, planType string, at time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE plan_instances SET changed_at = $3 WHERE engagement_id = $1 AND plan_type = $2`,
		engagementID, planType, at,
	); err != nil {
		t.Fatalf("set changed_at: %v", err)
	}
}

func changedAt(t *testing.T, db *testdb.DB, engagementID, planType string) time.Time {
	t.Helper()
	var at time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT changed_at FROM plan_instances WHERE engagement_id = $1 AND plan_type = $2`,
		engagementID, planType,
	).Scan(&at); err != nil {
		t.Fatalf("read changed_at: %v", err)
	}
	return at
}

func decodeMastheadBody(t *testing.T, resp *http.Response) plans.InstanceResponse {
	t.Helper()
	var out plans.InstanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func wantMasthead(t *testing.T, got *plans.Masthead) {
	t.Helper()
	want := plans.Masthead{ClientName: "Johanna Sorensen", PracticeName: mastheadPractice, ChangedOn: mastheadDay}
	if got == nil || *got != want {
		t.Fatalf("masthead = %+v, want %+v", got, want)
	}
}

// wantPDFNamesThePlan reads a Birth Plan PDF's text back and asserts the
// three facts a stranger in scrubs needs (#2209): whose it is (her legal
// name, the document name ADR-0017 keeps), whose Practice, and when it last
// changed, as a day in the Practice's zone.
func wantPDFNamesThePlan(t *testing.T, resp *http.Response) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	text := pdftext.Text(body)
	for _, want := range []string{"Name: Johanna Sorensen", "Doula: " + mastheadPractice, "Last changed: Oct 27, 2026"} {
		if !strings.Contains(text, want) {
			t.Errorf("PDF text does not contain %q:\n%s", want, text)
		}
	}
}

func seedMastheadInstance(t *testing.T, db *testdb.DB, engagementID, planType string) {
	t.Helper()
	seedInstance(t, db, engagementID, planType,
		`[{"id":"place","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"place":"Hospital"}`,
	)
	setChangedAt(t, db, engagementID, planType, mastheadNow)
}

// TestGetInstanceHandler_BirthPlanCarriesItsMasthead proves the Staff read
// the Practice-side print page renders names the plan the way its PDF does.
func TestGetInstanceHandler_BirthPlanCarriesItsMasthead(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-staff-get"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantMasthead(t, decodeMastheadBody(t, resp).Masthead)
}

// TestGetInstanceHandler_CarePlanHasNoMasthead proves the masthead stays a
// Birth Plan fact (#306's scope): the Care Plan is never printed for anyone.
func TestGetInstanceHandler_CarePlanHasNoMasthead(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-staff-care-plan"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	seedMastheadInstance(t, db, engagementID, carePlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, carePlanType)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := decodeMastheadBody(t, resp).Masthead; got != nil {
		t.Fatalf("masthead = %+v, want none on a Care Plan", got)
	}
}

// TestClientGetBirthPlanHandler_CarriesItsMasthead proves the portal read
// names the plan the same way, from the Client's own transaction.
func TestClientGetBirthPlanHandler_CarriesItsMasthead(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "masthead-client-get"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	clientID, engagementID := seedMastheadClient(t, db, practiceID)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantMasthead(t, decodeMastheadBody(t, resp).Masthead)
}

// TestClientAcknowledgeBirthPlanHandler_CarriesItsMasthead proves the
// acknowledgment's response, which replaces the page's plan in place, keeps
// the masthead the page is showing.
func TestClientAcknowledgeBirthPlanHandler_CarriesItsMasthead(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "masthead-client-ack"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	clientID, engagementID := seedMastheadClient(t, db, practiceID)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := postAcknowledgeBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantMasthead(t, decodeMastheadBody(t, resp).Masthead)
}

// TestGetBirthPlanPDFHandler_NamesThePlan is the PDF half of #2209 from the
// Practice side.
func TestGetBirthPlanPDFHandler_NamesThePlan(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-staff-pdf"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantPDFNamesThePlan(t, resp)
}

// TestClientGetBirthPlanPDFHandler_NamesThePlan is the PDF half of #2209
// from the portal.
func TestClientGetBirthPlanPDFHandler_NamesThePlan(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "masthead-client-pdf"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	clientID, engagementID := seedMastheadClient(t, db, practiceID)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, http.DefaultClient, srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantPDFNamesThePlan(t, resp)
}

// TestPostInstanceHandler_ChangedAtIsTheRequestClock proves a new plan's
// "last changed" is the instant it was started, read through clock (#773),
// not the database's now().
func TestPostInstanceHandler_ChangedAtIsTheRequestClock(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-post-clock"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	seedPlanTemplate(t, db, practiceID, birthPlanType,
		`[{"id":"place","type":"short_text","label":"Planned birth location","order":0}]`)

	srv, session := newClockedServer(t, db, uid, mastheadNow)
	defer srv.Close()

	resp := postInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if got := changedAt(t, db, engagementID, birthPlanType); !got.Equal(mastheadNow) {
		t.Fatalf("changed_at = %v, want %v", got, mastheadNow)
	}
	wantMasthead(t, decodeMastheadBody(t, resp).Masthead)
}

// TestPutInstanceHandler_ChangedAtMovesOnlyWhenAnAnswerChanges proves the
// "last changed" date tracks the answers, by the same test that clears the
// Client's acknowledgment (#301): a real change moves it to the request
// clock, and a save that changes nothing leaves it where it was.
func TestPutInstanceHandler_ChangedAtMovesOnlyWhenAnAnswerChanges(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-put-clock"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)
	earlier := mastheadNow.Add(-72 * time.Hour)
	setChangedAt(t, db, engagementID, birthPlanType, earlier)

	srv, session := newClockedServer(t, db, uid, mastheadNow)
	defer srv.Close()

	unchanged := putInstance(t, srv, session, practiceID, engagementID, birthPlanType,
		plans.PutInstanceRequest{Answers: plans.Answers{"place": "Hospital"}})
	defer unchanged.Body.Close()
	if unchanged.StatusCode != http.StatusOK {
		t.Fatalf("unchanged save status = %d, want %d", unchanged.StatusCode, http.StatusOK)
	}
	if got := changedAt(t, db, engagementID, birthPlanType); !got.Equal(earlier) {
		t.Fatalf("after a save that changed nothing, changed_at = %v, want %v", got, earlier)
	}

	changed := putInstance(t, srv, session, practiceID, engagementID, birthPlanType,
		plans.PutInstanceRequest{Answers: plans.Answers{"place": "Home"}})
	defer changed.Body.Close()
	if changed.StatusCode != http.StatusOK {
		t.Fatalf("changed save status = %d, want %d", changed.StatusCode, http.StatusOK)
	}
	if got := changedAt(t, db, engagementID, birthPlanType); !got.Equal(mastheadNow) {
		t.Fatalf("after a real change, changed_at = %v, want %v", got, mastheadNow)
	}
	wantMasthead(t, decodeMastheadBody(t, changed).Masthead)
}
