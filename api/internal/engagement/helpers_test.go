package engagement_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware, and seeds a live session for uid --
// returning the token its __session cookie carries, since #151 the
// cookie is the only credential the middleware reads.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/clients",
		staffauth.Middleware(db.App)(engagement.ListHandler()))
	mux.Handle("POST /practices/{practiceId}/clients",
		staffauth.Middleware(db.App)(engagement.CreateHandler(db.App)))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(db.App)(engagement.DetailHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row linking them to an existing practiceID, using
// the superuser Admin connection (which bypasses RLS) so fixture setup
// isn't gated by the policies under test.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) {
	t.Helper()

	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedContractorAtPractice mirrors seedStaffAtPractice but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
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

// seedSignupBonus grants practiceID the same +3 signup-bonus credit_ledger
// row staffauth.signup writes for a real Practice, giving CreateHandler
// tests a balance to spend without going through the signup flow.
func seedSignupBonus(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		t.Fatalf("seed signup bonus: %v", err)
	}
}

// seedStaffWithMembership inserts a new Practice plus a Staff member at
// it, via seedStaffAtPractice.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	seedStaffAtPractice(t, db, practiceID, identityUID)
	return practiceID
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email, status string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`,
		name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, $3) RETURNING id`,
		clientID, practiceID, status,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}
