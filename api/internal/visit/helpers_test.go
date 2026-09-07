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
