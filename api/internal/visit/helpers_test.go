package visit_test

import (
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
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
	return fixture.ServeStaff(t, db, uid, func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		visit.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	})
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
	db.ExecCorrectingOutcome(t, `UPDATE engagements SET birth_outcome = $1, pregnancy_ended_on = $2 WHERE id = $3`, outcome, endedOn, engagementID)
}

// env is what a test in this package starts from: a Practice with this
// package's whole surface mounted on a server, one Staff member at the
// screen and one Engagement. With no option she is an employee Doula and
// the Engagement is an intake birth for a new Client; a test says only
// what it changes, with signedInAs and withEngagement.
type env struct {
	*fixture.Scene
}

type envOption = fixture.SceneOption

// The option names this package's tests read as; each is the Scene's own.
var (
	signedInAs     = fixture.SignedInAs
	withEngagement = fixture.WithEngagement
)

// newEnv mounts this package's whole surface through visit.Mount, the same
// call main.go makes on the real GatedRouter and idempotency.Router.
func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) {
		visit.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	}
	return &env{fixture.NewScene(t, mount, append([]envOption{withEngagement()}, opts...)...)}
}

// visitsURL is the Engagement's Visits route plus path.
func (e *env) visitsURL(path string) string {
	return e.PracticeURL("/engagements/" + e.Engagement.ID + "/visits" + path)
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
