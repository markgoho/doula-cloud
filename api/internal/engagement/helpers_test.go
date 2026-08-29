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
// cookie is the only credential the middleware reads. List and Create
// moved to package client (#397); this package's own surface is now just
// Engagement detail and completion.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(db.App)(engagement.DetailHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

const doulaRole = "doula"

// seedStaffAtPractice seeds an employee Doula at practiceID.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "employee")
}

// seedContractorAtPractice mirrors seedStaffAtPractice but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "contractor")
}

// seedOwnerContractorAtPractice seeds a Membership holding both the
// owner role and a contractor employment type -- ADR-0017's "solo
// Practice": someone who runs the Practice and also does the work,
// billed as a contractor. clients_insert's WITH CHECK must stay in the
// Owner column for her, not the Doula (contractor) one.
func seedOwnerContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{"owner", doulaRole}, "contractor")
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

// seedStaffWithMembership inserts a new Practice plus a Staff member at
// it, via seedStaffAtPractice.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()

	practiceID, _ = seedStaffWithMembershipID(t, db, identityUID)
	return practiceID
}

// seedStaffWithMembershipID is seedStaffWithMembership widened to also
// return the seeded Staff row's id, for tests that need to set
// app.current_staff_id themselves (clients_insert's contractor check).
func seedStaffWithMembershipID(t *testing.T, db *testdb.DB, identityUID string) (practiceID, staffID string) {
	t.Helper()

	practiceID = testdb.SeedPractice(t, db, "Test Practice")
	staffID = seedStaffAtPractice(t, db, practiceID, identityUID)
	return practiceID, staffID
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection. name is used whole as
// given_name -- #396 split the single name column into three, but no
// caller here asserts on family_name/preferred_name specifically.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email, status string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $3) RETURNING id`,
		practiceID, name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, $3, 'birth') RETURNING id`,
		clientID, practiceID, status,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}
