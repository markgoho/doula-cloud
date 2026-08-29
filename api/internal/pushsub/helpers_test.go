package pushsub_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	return testdb.SeedPractice(t, db, name)
}

// seedStaffAtPractice seeds an employee Doula at practiceID.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) {
	t.Helper()
	testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{"doula"}, "employee")
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'intake', 'birth') RETURNING id`,
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
