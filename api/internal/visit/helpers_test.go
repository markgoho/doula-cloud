package visit_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

const (
	doulaRole = "doula"
	adminRole = "admin"
)

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware, and seeds a live session for uid --
// returning the token its __session cookie carries, since #151 the
// cookie is the only credential the middleware reads.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(db.App)(visit.ListHandler()))
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/visits",
		staffauth.Middleware(db.App)(visit.CreateHandler()))
	mux.Handle("PATCH /practices/{practiceId}/engagements/{engagementId}/visits/{visitId}",
		staffauth.Middleware(db.App)(visit.ReassignHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedStaffAtPracticeWithRoles inserts a Staff row bound to identityUID and
// a practice_memberships row linking them to an existing practiceID with
// the given roles, using the superuser Admin connection (which bypasses
// RLS) so fixture setup isn't gated by the policies under test.
func seedStaffAtPracticeWithRoles(t *testing.T, db *testdb.DB, practiceID, identityUID string, roles []string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, $2, $3) RETURNING id`,
		identityUID, "Test Staff "+identityUID, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	literal := "{" + strings.Join(roles, ",") + "}"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], 'employee')`,
		practiceID, staffID, literal,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}

// seedContractorAtPractice mirrors seedStaffAtPracticeWithRoles but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, $2, $3) RETURNING id`,
		identityUID, "Test Staff "+identityUID, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'contractor')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed contractor membership: %v", err)
	}
	return staffID
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

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
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
