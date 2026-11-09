package plans_test

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pdftext"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
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

// mastheadDoula is the Doula newMastheadEnv puts on every masthead test's
// Engagement, so the Doula row has a person to name (#2355).
const mastheadDoula = "Priya Raman"

// newUnstaffedMastheadEnv is an Engagement at mastheadPractice whose Client
// can sign in and whose legal name (given and family), first name
// (preferred) and full name all differ, with no Doula on it yet.
func newUnstaffedMastheadEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	opts = append([]envOption{withPractice(fixture.PracticeName(mastheadPractice)), withClient(fixture.ClientName("Johanna"), fixture.WithPortalLogin())}, opts...)
	e := newEnv(t, opts...)
	testdb.NameClient(t, e.db, e.Engagement.Client.ID, "Hannah", "Sorensen")
	return e
}

// newMastheadEnv is newUnstaffedMastheadEnv with mastheadDoula on the
// Engagement.
func newMastheadEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	e := newUnstaffedMastheadEnv(t, opts...)
	e.attachDoula(mastheadDoula)
	return e
}

// attachDoula seeds a Doula named name at the Practice and puts her on the
// Engagement with an open, granted Attachment.
func (e *env) attachDoula(name string) {
	e.t.Helper()
	testdb.SeedGrantedAttachment(e.t, e.db, e.Engagement.ID, e.staff(fixture.StaffName(name)).ID)
}

// staff is another Staff member at the Practice.
func (e *env) staff(opts ...fixture.StaffOption) fixture.StaffRecord {
	e.t.Helper()
	return fixture.Staff(e.t, e.db, e.Practice, opts...)
}

// setChangedAt pins the Engagement's seeded Plan Instance's changed_at,
// standing in for the save that last changed it.
func (e *env) setChangedAt(planType string, at time.Time) {
	e.t.Helper()
	e.exec(`UPDATE plan_instances SET changed_at = $3 WHERE engagement_id = $1 AND plan_type = $2`, e.Engagement.ID, planType, at)
}

func (e *env) changedAt(planType string) time.Time {
	e.t.Helper()
	return testdb.Scalar[time.Time](e.t, e.db.Admin, `SELECT changed_at FROM plan_instances WHERE engagement_id = $1 AND plan_type = $2`, e.Engagement.ID, planType)
}

// seedMastheadInstance starts the Engagement's planType Plan, last changed
// at mastheadNow.
func (e *env) seedMastheadInstance(planType string) {
	e.t.Helper()
	e.seedInstance(planType,
		`[{"id":"place","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"place":"Hospital"}`,
	)
	e.setChangedAt(planType, mastheadNow)
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

// wantPDFMasthead asserts a Birth Plan PDF opens with exactly these
// lines: her name, then doulaLines, then the Practice and the day, and
// then its first question. It reads the PDF's text back for the facts a
// stranger in scrubs needs (#2209, #2355): whose it is (her legal name,
// the document name ADR-0017 keeps), who her Doula is, whose Practice,
// and when it last changed, as a day in the Practice's zone.
func wantPDFMasthead(t *testing.T, pdf reqtest.PDF, doulaLines ...string) {
	t.Helper()
	lines := strings.Split(pdftext.Text(pdf.Body), "\n")
	want := append(append([]string{"Birth Plan", "Name: Johanna Sorensen"}, doulaLines...),
		"Practice: "+mastheadPractice, "Last changed: Oct 27, 2026", "Planned birth location")
	if len(lines) < len(want) || !reflect.DeepEqual(lines[:len(want)], want) {
		t.Fatalf("PDF opens with %q, want %q", lines, want)
	}
}

// TestBirthPlanReads_CarryTheirMasthead proves every read of the Birth Plan
// names the plan the way its PDF does: the Staff read the Practice-side
// print page renders, the portal read from the Client's own transaction,
// and the acknowledgment's response, which replaces the page's plan in
// place and so keeps the masthead the page is showing.
func TestBirthPlanReads_CarryTheirMasthead(t *testing.T) {
	reads := []struct {
		name   string
		method string
		url    func(*env) string
		as     func(*env) reqtest.Option
	}{
		{"Staff read", http.MethodGet, func(e *env) string { return e.instanceURL(birthPlanType, "") }, func(e *env) reqtest.Option { return e.signedIn }},
		{"portal read", http.MethodGet, func(e *env) string { return e.portalURL("") }, func(e *env) reqtest.Option { return e.asClient }},
		{"acknowledgment", http.MethodPost, func(e *env) string { return e.portalURL("/acknowledge") }, func(e *env) reqtest.Option { return e.asClient }},
	}
	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			e := newMastheadEnv(t)
			e.seedMastheadInstance(birthPlanType)

			out := reqtest.Expect[plans.InstanceResponse](t, read.method, read.url(e), http.StatusOK, read.as(e))

			wantMasthead(t, out.Masthead)
		})
	}
}

// TestGetInstanceHandler_CarePlanHasNoMasthead proves the masthead stays a
// Birth Plan fact (#306's scope): the Care Plan is never printed for anyone.
func TestGetInstanceHandler_CarePlanHasNoMasthead(t *testing.T) {
	e := newMastheadEnv(t)
	e.seedMastheadInstance(carePlanType)

	if got := e.instance(carePlanType).Masthead; got != nil {
		t.Fatalf("masthead = %+v, want none on a Care Plan", got)
	}
}

// TestBirthPlanPDF_NamesThePlan is the PDF half of #2209, from the
// Practice side and from the portal.
func TestBirthPlanPDF_NamesThePlan(t *testing.T) {
	for _, who := range []string{"Staff", "Client"} {
		t.Run(who, func(t *testing.T) {
			e := newMastheadEnv(t)
			e.seedMastheadInstance(birthPlanType)
			url, session := e.instanceURL(birthPlanType, "/pdf"), e.signedIn
			if who == "Client" {
				url, session = e.portalURL("/pdf"), e.asClient
			}

			wantPDFMasthead(t, reqtest.ExpectPDF(t, http.MethodGet, url, session), "Doula: "+mastheadDoula)
		})
	}
}

// TestPostInstanceHandler_ChangedAtIsTheRequestClock proves a new plan's
// "last changed" is the instant it was started, read through clock (#773),
// not the database's now().
func TestPostInstanceHandler_ChangedAtIsTheRequestClock(t *testing.T) {
	e := newMastheadEnv(t, atInstant(mastheadNow))
	e.seedTemplate(birthPlanType, `[{"id":"place","type":"short_text","label":"Planned birth location","order":0}]`)

	out := e.start(birthPlanType, http.StatusCreated)

	if got := e.changedAt(birthPlanType); !got.Equal(mastheadNow) {
		t.Fatalf("changed_at = %v, want %v", got, mastheadNow)
	}
	wantMasthead(t, out.Masthead)
}

// TestPutInstanceHandler_ChangedAtMovesOnlyWhenAnAnswerChanges proves the
// "last changed" date tracks the answers, by the same test that clears the
// Client's acknowledgment (#301): a real change moves it to the request
// clock, and a save that changes nothing leaves it where it was.
func TestPutInstanceHandler_ChangedAtMovesOnlyWhenAnAnswerChanges(t *testing.T) {
	e := newMastheadEnv(t, atInstant(mastheadNow))
	e.seedMastheadInstance(birthPlanType)
	earlier := mastheadNow.Add(-72 * time.Hour)
	e.setChangedAt(birthPlanType, earlier)

	e.putAnswers(birthPlanType, plans.Answers{"place": hospital})
	if got := e.changedAt(birthPlanType); !got.Equal(earlier) {
		t.Fatalf("after a save that changed nothing, changed_at = %v, want %v", got, earlier)
	}

	changed := e.putAnswers(birthPlanType, plans.Answers{"place": "Home"})
	if got := e.changedAt(birthPlanType); !got.Equal(mastheadNow) {
		t.Fatalf("after a real change, changed_at = %v, want %v", got, mastheadNow)
	}
	wantMasthead(t, changed.Masthead)
}

// TestMasthead_WithNoDoulaNamesOnlyThePractice proves an Engagement nobody
// is on yet names no Doula (#2355): the Practice row is the masthead's only
// answer to "whose plan is this, from where", on the Staff read and, with
// no Doula line at all, on the portal PDF.
func TestMasthead_WithNoDoulaNamesOnlyThePractice(t *testing.T) {
	e := newUnstaffedMastheadEnv(t)
	e.seedMastheadInstance(birthPlanType)

	wantMastheadDoulas(t, e.instance(birthPlanType).Masthead)
	wantPDFMasthead(t, reqtest.ExpectPDF(t, http.MethodGet, e.portalURL("/pdf"), e.asClient))
}

// TestMasthead_NamesEveryDoulaOnTheEngagement proves the portal names who
// the Engagement hub names (#2355): every Doula holding an open, granted
// Attachment, in the order she was put on it. One whose Attachment ended,
// or whose only Attachment is accrued, is not on it. The Practice-side PDF
// carries them as one "Doulas" line, the way the Engagement hub labels
// them.
func TestMasthead_NamesEveryDoulaOnTheEngagement(t *testing.T) {
	e := newMastheadEnv(t)
	e.attachDoula("Lena Vasquez")
	testdb.SeedAttachment(e.t, e.db, e.Engagement.ID, e.staff(fixture.StaffName("Ended Doula")).ID, "granted", true)
	testdb.SeedAttachment(e.t, e.db, e.Engagement.ID, e.staff(fixture.StaffName("Accrued Doula")).ID, "accrued", false)
	e.seedMastheadInstance(birthPlanType)

	wantMastheadDoulas(t, e.birthPlan().Masthead, mastheadDoula, "Lena Vasquez")
	wantPDFMasthead(t, reqtest.ExpectPDF(t, http.MethodGet, e.instanceURL(birthPlanType, "/pdf"), e.signedIn), "Doulas: "+mastheadDoula+", Lena Vasquez")
}
