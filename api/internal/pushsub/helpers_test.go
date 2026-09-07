package pushsub_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

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
