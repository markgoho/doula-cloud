package visit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
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
	// seededPracticeZoneName is the zone testdb.SeedPractice states for
	// every fixture Practice -- the value 00110_practice_timezone.sql
	// carried as its column default before #1166 dropped it.
	seededPracticeZoneName = "America/New_York"
)

// newServer mounts this package's whole surface through visit.Mount, the
// same call main.go makes on the real GatedRouter and idempotency.Router,
// and seeds a live session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	visit.Mount(g, ir)
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

	loc, err := time.LoadLocation(seededPracticeZoneName)
	if err != nil {
		t.Fatalf("load the seeded Practice's zone %q: %v", seededPracticeZoneName, err)
	}
	return loc
}

// setPracticeTimezone writes a Practice's zone directly on the Admin
// connection. practicetimezone.PutHandler is the real write (#1166);
// these tests need the fixture, not that handler's own behavior.
func setPracticeTimezone(t *testing.T, db *testdb.DB, practiceID, zone string) {
	t.Helper()

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET timezone = $1 WHERE id = $2`, zone, practiceID,
	); err != nil {
		t.Fatalf("set practice timezone: %v", err)
	}
}
