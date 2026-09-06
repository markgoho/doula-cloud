package visit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

// seedStaffAtPracticeWithRoles seeds an employee Staff member holding
// roles at practiceID.
func seedStaffAtPracticeWithRoles(t *testing.T, db *testdb.DB, practiceID, identityUID string, roles []string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, roles, "employee")
}

// seedContractorAtPractice mirrors seedStaffAtPracticeWithRoles but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{"doula"}, "contractor")
}

// seedGrantedAttachment inserts an open, granted-origin
// engagement_attachments row directly -- no handler in this codebase
// writes one yet (#317 builds that).
func seedGrantedAttachment(t *testing.T, db *testdb.DB, engagementID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $2)`,
		engagementID, staffID,
	); err != nil {
		t.Fatalf("seed granted attachment: %v", err)
	}
}

// seedPractice inserts a bare Practice row using the superuser Admin
// connection.
func seedPractice(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	return testdb.SeedPractice(t, db, "Test Practice")
}

// seedDoulaWithMembership inserts a new Practice plus a Staff member
// holding the Doula role there.
func seedDoulaWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID, staffID string) {
	t.Helper()

	practiceID = seedPractice(t, db)
	staffID = seedStaffAtPracticeWithRoles(t, db, practiceID, identityUID, []string{doulaRole})
	return practiceID, staffID
}

// seedEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID string) (engagementID string) {
	t.Helper()

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
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
