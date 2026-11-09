package sessionnotice_test

import (
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/testdb"
)

// queueEvicted runs QueueSessionEvicted for ev on its own committed
// transaction, the way a mint seam does inside its own.
func queueEvicted(t *testing.T, db *testdb.DB, ev authn.Eviction) (queued bool) {
	t.Helper()
	tx := db.BeginAs(t)
	queued, err := sessionnotice.QueueSessionEvicted(t.Context(), tx, ev)
	if err != nil {
		t.Fatalf("QueueSessionEvicted: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return queued
}

func TestQueueSessionEvicted_ReportsQueuedForStaff(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-evicted"

	if !queueEvicted(t, db, authn.Eviction{IdentityUID: uid, Tier: authn.TierStaff}) {
		t.Fatal("queued = false, want true for an evicted Staff session")
	}
	if got := countOutboxRows(t, db, uid, "session_evicted"); got != 1 {
		t.Fatalf("outbox rows = %d, want 1", got)
	}
}

// #610's recorded decision: an evicted Client sends no mail at all. See
// QueueSessionEvicted's own comment for the two reasons.
func TestQueueSessionEvicted_QueuesNothingForAPortalAccount(t *testing.T) {
	db := testdb.New(t)
	uid := portalaccount.NewIdentifier()

	if queueEvicted(t, db, authn.Eviction{IdentityUID: uid, Tier: authn.TierPortal}) {
		t.Fatal("queued = true, want false for an evicted portal session")
	}
	if got := countOutboxRows(t, db, uid, "session_evicted"); got != 0 {
		t.Fatalf("outbox rows = %d, want 0", got)
	}
}
