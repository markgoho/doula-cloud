package oncall_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// Named once each so goconst does not see independent literals across
// this package's tests.
const (
	doulaRole    = "doula"
	ownerRole    = "owner"
	adminRole    = "admin"
	employeeType = "employee"
	zone         = "America/New_York"
	backupName   = "Bo Backup"
	statusSent   = "sent"
	fieldWeek    = "startWeek"
)

// The fixture calendar, named once. Every date test in this package
// works against one birth due 2026-10-30, whose default window under a
// 37-week rule and a 14-day grace runs windowStart..windowEnd.
const (
	windowStart    = "2026-10-09"
	windowEnd      = "2026-11-13"
	octFirst       = "2026-10-01"
	octTwelfth     = "2026-10-12"
	octFifteen     = "2026-10-15"
	octTwenty      = "2026-10-20"
	octTwentyTwo   = "2026-10-22"
	octTwentyThree = "2026-10-23"
	octLast        = "2026-10-31"
	grantedOn      = "2026-06-01"
	birthDue       = "2026-10-30"
)

// env is what almost every test in this package starts from: a Practice
// with this package's whole surface mounted on a server, its Owner, a
// primary Doula and one active birth due 2026-10-30 that the Doula is
// attached to. A test reads as what it changes from that, and signs each
// request in with asOwner or as(someone).
type env struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server
	enq *tasknudge.FakeEnqueuer

	Practice fixture.PracticeRecord
	Owner    fixture.StaffRecord
	Doula    fixture.StaffRecord
	Birth    fixture.EngagementRecord

	asOwner reqtest.Option

	nudgesBefore int
}

type envSpec struct {
	practiceName string
	dueDate      string
}

type envOption func(*envSpec)

func dueOn(date string) envOption {
	return func(s *envSpec) { s.dueDate = date }
}

func practiceNamed(name string) envOption {
	return func(s *envSpec) { s.practiceName = name }
}

// newBareEnv is a Practice and a server with nobody working there yet.
func newBareEnv(t *testing.T, practiceOpts ...fixture.PracticeOption) *env {
	t.Helper()
	db := testdb.New(t)
	e := &env{t: t, db: db, enq: &tasknudge.FakeEnqueuer{}, Practice: fixture.Practice(t, db, practiceOpts...)}
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	oncall.Mount(g, ir, e.enq)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{practiceName: "Test Practice", dueDate: birthDue}
	for _, opt := range opts {
		opt(&spec)
	}
	e := newBareEnv(t, fixture.PracticeName(spec.practiceName))
	e.Owner = fixture.Staff(t, e.db, e.Practice, fixture.Owner())
	e.Doula = fixture.Staff(t, e.db, e.Practice, fixture.StaffName("Maya Primary"))
	e.Birth = fixture.Engagement(t, e.db, e.Practice,
		fixture.NewClient(fixture.ClientName("Ada Whitfield")),
		fixture.Status(fixture.StatusActive), fixture.DueDate(spec.dueDate), fixture.WithDoula(e.Doula))
	e.asOwner = e.as(e.Owner)
	return e
}

// as signs a request in as who.
func (e *env) as(who fixture.StaffRecord) reqtest.Option {
	e.t.Helper()
	return reqtest.Session(fixture.Session(e.t, e.db, who))
}

// backup is a colleague on the birth beside the primary.
func (e *env) backup() fixture.StaffRecord {
	e.t.Helper()
	b := fixture.Staff(e.t, e.db, e.Practice, fixture.StaffName(backupName))
	testdb.SeedGrantedAttachment(e.t, e.db, e.Birth.ID, b.ID)
	return b
}

// attachNewBirth is another active birth with no due date, for a Client
// named name, that who is attached to.
func (e *env) attachNewBirth(name string, who fixture.StaffRecord) {
	e.t.Helper()
	fixture.Engagement(e.t, e.db, e.Practice,
		fixture.NewClient(fixture.ClientName(name)), fixture.Status(fixture.StatusActive), fixture.WithDoula(who))
}

// stranger is a Doula at the Practice who is not on the birth.
func (e *env) stranger() fixture.StaffRecord {
	e.t.Helper()
	return fixture.Staff(e.t, e.db, e.Practice, fixture.StaffName("Not On It"))
}

func (e *env) practiceURL(path string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + path
}

func (e *env) birthURL(path string) string {
	return e.practiceURL("/engagements/" + e.Birth.ID + path)
}

func (e *env) rosterURL(from, to string) string {
	return e.practiceURL("/on-call?from=" + from + "&to=" + to)
}

func (e *env) settingsURL() string { return e.practiceURL("/on-call-settings") }

func (e *env) panelURL() string { return e.birthURL("/on-call") }

func (e *env) ruleURL() string { return e.birthURL("/on-call-rule") }

func (e *env) gapsURL() string { return e.birthURL("/coverage-gaps") }

func (e *env) narrowURL(staffID string) string {
	return e.birthURL("/attachments/" + staffID + "/on-call")
}

// exec runs one fixture statement on the superuser connection.
func (e *env) exec(query string, args ...any) {
	e.t.Helper()
	if _, err := e.db.Admin.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("fixture %q: %v", query, err)
	}
}

// nudges is how many times a write has nudged the worker since the test
// last called forgetNudges.
func (e *env) nudges() int { return len(e.enq.Calls()) - e.nudgesBefore }

func (e *env) forgetNudges() { e.nudgesBefore = len(e.enq.Calls()) }

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	return testdb.Scalar[int](e.t, e.db.Admin, query, args...)
}

// roster is the Owner's read of the roster over from..to.
func (e *env) roster(from, to string) oncall.RosterResponse {
	e.t.Helper()
	return reqtest.Expect[oncall.RosterResponse](e.t, http.MethodGet, e.rosterURL(from, to), http.StatusOK, e.asOwner)
}

// panel is the Owner's read of the birth's on-call panel.
func (e *env) panel() oncall.EngagementOnCall {
	e.t.Helper()
	return reqtest.Expect[oncall.EngagementOnCall](e.t, http.MethodGet, e.panelURL(), http.StatusOK, e.asOwner)
}

// activityActors lists who performed action on the birth, oldest first.
func (e *env) activityActors(action string) []string {
	e.t.Helper()
	return testdb.Column[string](e.t, e.db.Admin,
		`SELECT actor_staff_id FROM activity WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		e.Birth.ID, action)
}

// refusal is the error envelope a 4xx answers with.
type refusal struct {
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func emailOf(s fixture.StaffRecord) string { return s.UID + "@example.com" }
