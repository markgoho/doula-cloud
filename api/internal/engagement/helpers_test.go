package engagement_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
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
		staffauth.Middleware(db.App)(engagement.CreateHandler(db.App, &tasknudge.FakeEnqueuer{})))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(db.App)(engagement.DetailHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row linking them to an existing practiceID, using
// the superuser Admin connection (which bypasses RLS) so fixture setup
// isn't gated by the policies under test.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

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
	return staffID
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

	practiceID, _ = seedStaffWithMembershipID(t, db, identityUID)
	return practiceID
}

// seedStaffWithMembershipID is seedStaffWithMembership widened to also
// return the seeded Staff row's id, for tests that need to set
// app.current_staff_id themselves (clients_insert's contractor check).
func seedStaffWithMembershipID(t *testing.T, db *testdb.DB, identityUID string) (practiceID, staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Test Practice') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
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

// seedPortalUser inserts a client_portal_users row for clientID -- #346's
// join target. When accepted is true, identity_uid is set the way
// accept.go leaves it; otherwise the row stays pending (identity_uid
// null), same as right after portalinvite.InviteHandler runs.
func seedPortalUser(t *testing.T, db *testdb.DB, clientID string, accepted bool) (portalUserID string) {
	t.Helper()

	var identityUID sql.NullString
	if accepted {
		identityUID = sql.NullString{String: "identity-" + clientID, Valid: true}
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, identity_uid) VALUES ($1, $2) RETURNING id`,
		clientID, identityUID,
	).Scan(&portalUserID); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}
	return portalUserID
}

// seedOutboxRow inserts a portal_invite_outbox row for portalUserID at an
// explicit createdAt, rather than relying on now(), so a "latest row
// wins" test can seed two rows in a known order without depending on
// clock resolution between two sequential inserts.
func seedOutboxRow(t *testing.T, db *testdb.DB, portalUserID, status string, createdAt time.Time) {
	t.Helper()

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO portal_invite_outbox (client_portal_user_id, status, created_at) VALUES ($1, $2, $3)`,
		portalUserID, status, createdAt,
	); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
}
