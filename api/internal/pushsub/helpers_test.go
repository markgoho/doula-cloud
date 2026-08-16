package pushsub_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

// seedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row linking them to an existing practiceID, using
// the superuser Admin connection so fixture setup isn't gated by the
// policies under test.
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
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{doula}')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Test Client', 'client@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, 'intake') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPortalUser links identityUID to clientID via client_portal_users,
// using the superuser Admin connection.
func seedPortalUser(t *testing.T, db *testdb.DB, identityUID, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
}

// countSubscriptions counts push_subscriptions rows for endpoint, via the
// superuser Admin connection (bypassing RLS -- used only to assert on
// ground truth, not to observe RLS in effect).
func countSubscriptions(t *testing.T, db *testdb.DB, endpoint string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM push_subscriptions WHERE endpoint = $1`, endpoint).Scan(&count); err != nil {
		t.Fatalf("count push_subscriptions for %q: %v", endpoint, err)
	}
	return count
}
