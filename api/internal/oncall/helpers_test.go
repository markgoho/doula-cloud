package oncall_test

import (
	"database/sql"
	"net/http"
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
	doulaRole     = "doula"
	ownerRole     = "owner"
	adminRole     = "admin"
	employeeType  = "employee"
	zone          = "America/New_York"
	backupName    = "Bo Backup"
	statusSent    = "sent"
	statusSkipped = "skipped"
	fieldWeek     = "startWeek"
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
// with this package's whole surface mounted on a server, its Owner at the
// screen (the Scene's Staff), a primary Doula and one active birth due
// 2026-10-30 that the Doula is attached to. A test reads as what it
// changes from that, and signs each request in with SignedIn or
// As(someone).
type env struct {
	*fixture.Scene
	enq *tasknudge.FakeEnqueuer

	Doula fixture.StaffRecord
	Birth fixture.EngagementRecord

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
func newBareEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	e := &env{enq: &tasknudge.FakeEnqueuer{}}
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		oncall.Mount(g, ir, e.enq)
	}
	e.Scene = fixture.NewScene(t, mount, opts...)
	return e
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	spec := envSpec{practiceName: "Test Practice", dueDate: birthDue}
	for _, opt := range opts {
		opt(&spec)
	}
	e := newBareEnv(t, fixture.InPractice(fixture.PracticeName(spec.practiceName)), fixture.SignedInAs(fixture.Owner()))
	e.Doula = e.Colleague(fixture.StaffName("Maya Primary"))
	e.Birth = e.AnotherEngagement(
		fixture.NewClient(fixture.ClientName("Ada Whitfield")),
		fixture.Status(fixture.StatusActive), fixture.DueDate(spec.dueDate), fixture.WithDoula(e.Doula))
	return e
}

// backup is a colleague on the birth beside the primary.
func (e *env) backup() fixture.StaffRecord {
	e.T.Helper()
	b := fixture.Staff(e.T, e.DB, e.Practice, fixture.StaffName(backupName))
	testdb.SeedGrantedAttachment(e.T, e.DB, e.Birth.ID, b.ID)
	return b
}

// attachNewBirth is another active birth with no due date, for a Client
// named name, that who is attached to.
func (e *env) attachNewBirth(name string, who fixture.StaffRecord) {
	e.T.Helper()
	fixture.Engagement(e.T, e.DB, e.Practice,
		fixture.NewClient(fixture.ClientName(name)), fixture.Status(fixture.StatusActive), fixture.WithDoula(who))
}

// stranger is a Doula at the Practice who is not on the birth.
func (e *env) stranger() fixture.StaffRecord {
	e.T.Helper()
	return fixture.Staff(e.T, e.DB, e.Practice, fixture.StaffName("Not On It"))
}

func (e *env) birthURL(path string) string {
	return e.PracticeURL("/engagements/" + e.Birth.ID + path)
}

func (e *env) rosterURL(from, to string) string {
	return e.PracticeURL("/on-call?from=" + from + "&to=" + to)
}

func (e *env) settingsURL() string { return e.PracticeURL("/on-call-settings") }

func (e *env) panelURL() string { return e.birthURL("/on-call") }

func (e *env) ruleURL() string { return e.birthURL("/on-call-rule") }

func (e *env) gapsURL() string { return e.birthURL("/coverage-gaps") }

func (e *env) narrowURL(staffID string) string {
	return e.birthURL("/attachments/" + staffID + "/on-call")
}

// nudges is how many times a write has nudged the worker since the test
// last called forgetNudges.
func (e *env) nudges() int { return len(e.enq.Calls()) - e.nudgesBefore }

func (e *env) forgetNudges() { e.nudgesBefore = len(e.enq.Calls()) }

func (e *env) count(query string, args ...any) int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin, query, args...)
}

// roster is the Owner's read of the roster over from..to.
func (e *env) roster(from, to string) oncall.RosterResponse {
	e.T.Helper()
	return reqtest.Expect[oncall.RosterResponse](e.T, http.MethodGet, e.rosterURL(from, to), http.StatusOK, e.SignedIn)
}

// panel is the Owner's read of the birth's on-call panel.
func (e *env) panel() oncall.EngagementOnCall {
	e.T.Helper()
	return reqtest.Expect[oncall.EngagementOnCall](e.T, http.MethodGet, e.panelURL(), http.StatusOK, e.SignedIn)
}

// activityActors lists who performed action on the birth, oldest first.
func (e *env) activityActors(action string) []string {
	e.T.Helper()
	return testdb.Column[string](e.T, e.DB.Admin,
		`SELECT actor_staff_id FROM activity WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		e.Birth.ID, action)
}

// refusal is the error envelope a 4xx answers with.
type refusal struct {
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func emailOf(s fixture.StaffRecord) string { return s.UID + "@example.com" }
