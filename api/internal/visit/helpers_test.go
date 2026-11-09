package visit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const (
	doulaRole = "doula"
	adminRole = "admin"
	// grantedOrigin is named once so golangci-lint's goconst check doesn't
	// see three independent "granted" literals across this package's tests.
	grantedOrigin = "granted"
	// notAUUID is the malformed id every "reject a bad path segment or
	// field" case sends, named here for the same goconst reason.
	notAUUID = "not-a-uuid"
)

// newServer mounts this package's whole surface through visit.Mount, the
// same call main.go makes on the real GatedRouter and idempotency.Router,
// and seeds a live session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	visit.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedVisit inserts a Visit under engagementID assigned to staffID, using
// the superuser Admin connection.
func seedVisit(t *testing.T, db *testdb.DB, engagementID, staffID string) (visitID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id`,
		engagementID, staffID,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	return visitID
}

// seedScheduledVisit is seedVisit plus an already-set scheduled_at, for a
// test that needs to prove ScheduleHandler changes or clears an existing
// value rather than only ever setting one from nothing.
func seedScheduledVisit(t *testing.T, db *testdb.DB, engagementID, staffID string, scheduledAt time.Time) (visitID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3) RETURNING id`,
		engagementID, staffID, scheduledAt,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed scheduled visit: %v", err)
	}
	return visitID
}

// seedVisitWithNotes is seedVisit plus an already-written notes value, for
// a test that needs to prove NotesHandler changes or clears an existing
// value rather than only ever setting one from nothing (#251).
func seedVisitWithNotes(t *testing.T, db *testdb.DB, engagementID, staffID, notes string) (visitID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, notes) VALUES ($1, $2, $3) RETURNING id`,
		engagementID, staffID, notes,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed visit with notes: %v", err)
	}
	return visitID
}

// seedVisitCreatedAt is seedVisit with an explicit created_at and no
// scheduled_at, for #281's fallback case: DeriveType must read a Visit
// that was never scheduled by when it was logged.
func seedVisitCreatedAt(t *testing.T, db *testdb.DB, engagementID, staffID string, createdAt time.Time) (visitID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, created_at) VALUES ($1, $2, $3) RETURNING id`,
		engagementID, staffID, createdAt,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed visit with created_at: %v", err)
	}
	return visitID
}

// setPregnancyEnded writes an Engagement's birth outcome and
// pregnancy-end date directly on the Admin connection, bypassing
// engagement.RecordBirthOutcomeHandler's own frozen-write rules -- this
// package's tests need the fixture, not that handler's own behavior,
// which engagement_test already covers. endedOn is YYYY-MM-DD text,
// matching the shape DeriveType compares against.
//
// 00093's engagements_freeze_outcome trigger fires for any writer, Admin
// included -- a superuser bypasses Row-Level Security, not a BEFORE
// UPDATE trigger -- so a second call correcting an already-recorded
// outcome opens the trigger's own door (app.allow_outcome_correction)
// first, inside the one transaction the setting has to survive in.
func setPregnancyEnded(t *testing.T, db *testdb.DB, engagementID, outcome, endedOn string) {
	t.Helper()

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("set pregnancy ended: begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		t.Fatalf("set pregnancy ended: open correction door: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = $1, pregnancy_ended_on = $2 WHERE id = $3`,
		outcome, endedOn, engagementID,
	); err != nil {
		t.Fatalf("set pregnancy ended: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("set pregnancy ended: commit: %v", err)
	}
}

// seededPracticeZone loads the zone a freshly seeded Practice holds --
// the one testdb.SeedPractice states. Named here rather than repeated as
// a literal so the day these tests type Visits against follows the
// fixture rather than a copy of it.
func seededPracticeZone(t *testing.T) *time.Location {
	t.Helper()

	loc, err := time.LoadLocation(testdb.SeededPracticeZoneName)
	if err != nil {
		t.Fatalf("load the seeded Practice's zone %q: %v", testdb.SeededPracticeZoneName, err)
	}
	return loc
}

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs and withEngagement.
type env struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server

	Practice   fixture.PracticeRecord
	Staff      fixture.StaffRecord
	Engagement fixture.EngagementRecord

	signedIn reqtest.Option
}

type envSpec struct {
	staff      []fixture.StaffOption
	engagement []fixture.EngagementOption
}

type envOption func(*envSpec)

// signedInAs shapes the Staff member who is at the screen.
func signedInAs(opts ...fixture.StaffOption) envOption {
	return func(s *envSpec) { s.staff = append(s.staff, opts...) }
}

// withEngagement shapes the Engagement the test is about.
func withEngagement(opts ...fixture.EngagementOption) envOption {
	return func(s *envSpec) { s.engagement = append(s.engagement, opts...) }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	var spec envSpec
	for _, opt := range opts {
		opt(&spec)
	}
	db := testdb.New(t)
	e := &env{t: t, db: db, Practice: fixture.Practice(t, db)}
	e.Staff = fixture.Staff(t, db, e.Practice, spec.staff...)
	e.Engagement = fixture.Engagement(t, db, e.Practice, spec.engagement...)
	e.srv = e.mount()
	e.signedIn = e.as(e.Staff)
	return e
}

// mount mounts this package's whole surface through visit.Mount, the same
// call main.go makes on the real GatedRouter and idempotency.Router.
func (e *env) mount() *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, e.db.App)
	ir := idempotency.NewRouter(g, e.db.App)
	visit.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	srv := httptest.NewServer(mux)
	e.t.Cleanup(srv.Close)
	return srv
}

// as signs a request in as who.
func (e *env) as(who fixture.StaffRecord) reqtest.Option {
	e.t.Helper()
	return reqtest.Session(fixture.Session(e.t, e.db, who))
}

// staff is another Staff member at the Practice.
func (e *env) staff(opts ...fixture.StaffOption) fixture.StaffRecord {
	e.t.Helper()
	return fixture.Staff(e.t, e.db, e.Practice, opts...)
}

// visitsURL is the Engagement's Visits route plus path.
func (e *env) visitsURL(path string) string {
	return e.srv.URL + "/api/practices/" + e.Practice.ID + "/engagements/" + e.Engagement.ID + "/visits" + path
}
