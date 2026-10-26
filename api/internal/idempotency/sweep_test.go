package idempotency_test

import (
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// sweepNow is the instant the sweep tests seed clock.Now with, so a row's
// age is the test's own number and not the wall clock's.
var sweepNow = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

// seedKeyAt inserts one row through the superuser, created at the given
// age before sweepNow. Exactly one of staffID and clientID is set.
func seedKeyAt(t *testing.T, db *testdb.DB, key, practiceID, staffID, clientID string, age time.Duration) {
	t.Helper()
	var staff, client any
	if staffID != "" {
		staff = staffID
	}
	if clientID != "" {
		client = clientID
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO idempotency_keys (key, practice_id, staff_id, client_id, status_code, response_body, created_at)
		 VALUES ($1, $2, $3, $4, 201, '{"text":"her own words"}', $5)`,
		key, practiceID, staff, client, sweepNow.Add(-age),
	); err != nil {
		t.Fatalf("seed key %s: %v", key, err)
	}
}

// remainingKeys reads every row's key through the superuser, sorted.
func remainingKeys(t *testing.T, db *testdb.DB) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(), `SELECT key FROM idempotency_keys ORDER BY key`)
	if err != nil {
		t.Fatalf("read keys: %v", err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan key: %v", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate keys: %v", err)
	}
	return keys
}

// runSweep runs the Sweeper in one app_runtime transaction, the way the
// drain runs a registration, with the worker door set when door is true.
func runSweep(t *testing.T, db *testdb.DB, door bool) error {
	t.Helper()
	ctx := clock.Into(t.Context(), func() time.Time { return sweepNow })
	tx, err := db.App.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if door {
		if _, err := tx.ExecContext(ctx, `SELECT set_config($1, 'true', true)`, outbox.NotificationDoor); err != nil {
			t.Fatalf("set door: %v", err)
		}
	}
	if err := (idempotency.Sweeper{}).ProcessPending(ctx, tx); err != nil {
		return err //nolint:wrapcheck // test helper; the caller only checks nil-ness
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

// TestSweeper_DeletesEveryExpiredRowAndKeepsTheLiveOnes is #1987's sweep
// AC: a row past the 48-hour TTL is deleted at every Practice, Staff and
// Client rows alike, and a row inside it stays -- the age read from
// clock.Now, not from the database's own clock.
func TestSweeper_DeletesEveryExpiredRowAndKeepsTheLiveOnes(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Sweep Practice A")
	practiceB := testdb.SeedPractice(t, db, "Sweep Practice B")
	staffA := seedStaffMember(t, db, "sweep-staff-a")
	staffB := seedStaffMember(t, db, "sweep-staff-b")
	clientA, _ := testdb.SeedEngagement(t, db, practiceA)

	seedKeyAt(t, db, "expired-staff-a", practiceA, staffA, "", 49*time.Hour)
	seedKeyAt(t, db, "expired-staff-b", practiceB, staffB, "", 72*time.Hour)
	seedKeyAt(t, db, "expired-client-a", practiceA, "", clientA, 49*time.Hour)
	seedKeyAt(t, db, "live-staff-a", practiceA, staffA, "", time.Hour)
	seedKeyAt(t, db, "live-client-a", practiceA, "", clientA, 47*time.Hour)

	if err := runSweep(t, db, true); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	want := []string{"live-client-a", "live-staff-a"}
	if got := remainingKeys(t, db); !slices.Equal(got, want) {
		t.Fatalf("keys after the sweep = %v, want %v", got, want)
	}
}

// TestSweeper_RefusedOutsideTheWorkerDoor proves the delete is the
// drain's alone: sweep_expired_idempotency_keys sees every Practice's
// rows, so a transaction without the worker door -- a Staff or a Client
// request -- is refused and deletes nothing.
func TestSweeper_RefusedOutsideTheWorkerDoor(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Sweep No Door")
	staffID := seedStaffMember(t, db, "sweep-no-door")
	seedKeyAt(t, db, "expired", practiceID, staffID, "", 49*time.Hour)

	if err := runSweep(t, db, false); err == nil {
		t.Fatal("the sweep ran without the worker door, want it refused")
	}
	if got := remainingKeys(t, db); !slices.Equal(got, []string{"expired"}) {
		t.Fatalf("keys after a refused sweep = %v, want [expired]", got)
	}
}
