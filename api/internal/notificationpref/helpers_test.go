package notificationpref_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

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
	engagementID = seedEngagement(t, db, practiceID, clientID)
	return clientID, engagementID
}

// seedEngagement inserts one more Engagement for an already-seeded
// clientID -- used to prove muting one Engagement leaves a sibling
// Engagement's own push preference untouched.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID, clientID string) (engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'intake', 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
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

// readPreferenceRow reads notification_preferences' ground truth for
// identityUID/engagementID, via the superuser Admin connection (bypassing
// RLS -- used only to assert on ground truth, not to observe RLS in
// effect). found is false when no row exists yet.
func readPreferenceRow(t *testing.T, db *testdb.DB, identityUID, engagementID string) (muted bool, found bool) {
	t.Helper()
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT muted FROM notification_preferences WHERE identity_uid = $1 AND engagement_id = $2 AND channel = 'push'`,
		identityUID, engagementID,
	).Scan(&muted)
	if err != nil {
		return false, false
	}
	return muted, true
}

// latestActivityAction reads the most recent activity row's action for
// engagementID, via the superuser Admin connection.
func latestActivityAction(t *testing.T, db *testdb.DB, engagementID string) (action, actorKind string) {
	t.Helper()
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text FROM activity
		 WHERE subject_kind = 'engagement' AND subject_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&action, &actorKind)
	if err != nil {
		t.Fatalf("read latest activity for %q: %v", engagementID, err)
	}
	return action, actorKind
}
