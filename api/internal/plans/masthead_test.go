package plans_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// mastheadDoula is the Doula seedMastheadClient puts on every masthead
// test's Engagement, so the Doula row has a person to name (#2355).
const mastheadDoula = "Priya Raman"

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
// first name (preferred) and full name all differ, at mastheadPractice,
// with mastheadDoula on her Engagement.
func seedMastheadClient(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()
	clientID, engagementID = seedUnstaffedMastheadClient(t, db, practiceID)
	attachMastheadDoula(t, db, practiceID, engagementID, "masthead-doula-priya", mastheadDoula)
	return clientID, engagementID
}

// seedUnstaffedMastheadClient is seedMastheadClient with no Doula on the
// Engagement yet.
func seedUnstaffedMastheadClient(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()
	clientID, engagementID = testdb.SeedNamedEngagement(t, db, practiceID, "Johanna", "hannah@example.com")
	testdb.NameClient(t, db, clientID, "Hannah", "Sorensen")
	return clientID, engagementID
}

// attachMastheadDoula seeds a Doula named name at practiceID and puts her
// on engagementID with an open, granted Attachment.
func attachMastheadDoula(t *testing.T, db *testdb.DB, practiceID, engagementID, uid, name string) {
	t.Helper()
	staffID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, uid, name, []string{doulaRole}, "employee")
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
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
	wantMastheadDoulas(t, got, mastheadDoula)
}

// wantMastheadDoulas is wantMasthead for an Engagement with these Doulas
// on it, in the order they were put on it, or none.
func wantMastheadDoulas(t *testing.T, got *plans.Masthead, doulas ...string) {
	t.Helper()
	if doulas == nil {
		doulas = []string{}
	}
	want := plans.Masthead{ClientName: "Johanna Sorensen", Doulas: doulas, PracticeName: mastheadPractice, ChangedOn: mastheadDay}
	if got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("masthead = %+v, want %+v", got, want)
	}
}

// wantPDFNamesThePlan reads a Birth Plan PDF's text back and asserts the
// facts a stranger in scrubs needs (#2209, #2355): whose it is (her legal
// name, the document name ADR-0017 keeps), who her Doula is, whose
// Practice, and when it last changed, as a day in the Practice's zone.
func wantPDFNamesThePlan(t *testing.T, resp *http.Response) {
	t.Helper()
	wantPDFMasthead(t, resp, "Doula: "+mastheadDoula)
}

// wantPDFMasthead asserts a Birth Plan PDF opens with exactly these
// lines: her name, then doulaLines, then the Practice and the day, and
// then its first question.
func wantPDFMasthead(t *testing.T, resp *http.Response, doulaLines ...string) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	lines := strings.Split(pdftext.Text(body), "\n")
	want := append(append([]string{"Birth Plan", "Name: Johanna Sorensen"}, doulaLines...),
		"Practice: "+mastheadPractice, "Last changed: Oct 27, 2026", "Planned birth location")
	if len(lines) < len(want) || !reflect.DeepEqual(lines[:len(want)], want) {
		t.Fatalf("PDF opens with %q, want %q", lines, want)
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

// TestGetInstanceHandler_MastheadWithNoDoulaNamesOnlyThePractice proves an
// Engagement nobody is on yet names no Doula (#2355): the Practice row is
// the masthead's only answer to "whose plan is this, from where".
func TestGetInstanceHandler_MastheadWithNoDoulaNamesOnlyThePractice(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-staff-no-doula"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedUnstaffedMastheadClient(t, db, practiceID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstance(t, srv, session, practiceID, engagementID, birthPlanType)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantMastheadDoulas(t, decodeMastheadBody(t, resp).Masthead)
}

// TestClientGetBirthPlanPDFHandler_MastheadWithNoDoulaNamesOnlyThePractice
// is the portal PDF with nobody on the Engagement: no Doula line at all.
func TestClientGetBirthPlanPDFHandler_MastheadWithNoDoulaNamesOnlyThePractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "masthead-client-pdf-no-doula"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	clientID, engagementID := seedUnstaffedMastheadClient(t, db, practiceID)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, http.DefaultClient, srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantPDFMasthead(t, resp)
}

// TestClientGetBirthPlanHandler_MastheadNamesEveryDoulaOnTheEngagement
// proves the portal names who the Engagement hub names (#2355): every Doula
// holding an open, granted Attachment, in the order she was put on it. One
// whose Attachment ended, or whose only Attachment is accrued, is not on
// it.
func TestClientGetBirthPlanHandler_MastheadNamesEveryDoulaOnTheEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "masthead-client-two-doulas"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	clientID, engagementID := seedMastheadClient(t, db, practiceID)
	attachMastheadDoula(t, db, practiceID, engagementID, "masthead-doula-lena", "Lena Vasquez")
	ended := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "masthead-doula-ended", "Ended Doula", []string{doulaRole}, "employee")
	testdb.SeedAttachment(t, db, engagementID, ended, "granted", true)
	accrued := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "masthead-doula-accrued", "Accrued Doula", []string{doulaRole}, "employee")
	testdb.SeedAttachment(t, db, engagementID, accrued, "accrued", false)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantMastheadDoulas(t, decodeMastheadBody(t, resp).Masthead, mastheadDoula, "Lena Vasquez")
}

// TestGetBirthPlanPDFHandler_MastheadNamesEveryDoulaOnTheEngagement is the
// Practice-side PDF with two Doulas on the Engagement: one "Doulas" line,
// the way the Engagement hub labels them.
func TestGetBirthPlanPDFHandler_MastheadNamesEveryDoulaOnTheEngagement(t *testing.T) {
	db := testdb.New(t)
	const uid = "masthead-staff-pdf-two-doulas"
	practiceID := testdb.SeedPractice(t, db, mastheadPractice)
	testdb.SeedStaffAtPractice(t, db, practiceID, uid, []string{doulaRole}, "employee")
	_, engagementID := seedMastheadClient(t, db, practiceID)
	attachMastheadDoula(t, db, practiceID, engagementID, "masthead-doula-lena", "Lena Vasquez")
	seedMastheadInstance(t, db, engagementID, birthPlanType)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	wantPDFMasthead(t, resp, "Doulas: "+mastheadDoula+", Lena Vasquez")
}
