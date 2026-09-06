package sessionnotice_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/testdb"
)

// queueEvicted runs QueueSessionEvicted for ev on its own committed
// transaction, the way a mint seam does inside its own.
func queueEvicted(t *testing.T, db *testdb.DB, evictions ...authn.Eviction) (queued bool) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	for _, ev := range evictions {
		queued, err = sessionnotice.QueueSessionEvicted(t.Context(), tx, ev)
		if err != nil {
			t.Fatalf("QueueSessionEvicted: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return queued
}

func TestQueueSessionEvicted_InsertsPendingRowForStaff(t *testing.T) {
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

func TestQueueSessionEvicted_ConflictOnExistingPendingRowIsNoop(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-double-eviction"
	ev := authn.Eviction{IdentityUID: uid, Tier: authn.TierStaff}

	queueEvicted(t, db, ev, ev)

	if got := countOutboxRows(t, db, uid, "session_evicted"); got != 1 {
		t.Fatalf("outbox rows after two rapid calls = %d, want 1", got)
	}
}

func TestWorker_ProcessPending_MailsSessionEvicted(t *testing.T) {
	db := testdb.New(t)
	const uid = "staff-worker-evicted"
	seedStaff(t, db, uid)
	outboxID := seedOutboxRow(t, db, uid, "session_evicted", 0, time.Now().Add(-time.Minute), time.Now())

	sender := &mail.FakeSender{}
	runWorker(t, db, newTestWorker(sender))

	status, _ := outboxRowState(t, db, outboxID)
	if status != testStatusSent {
		t.Fatalf("status = %q, want %s", status, testStatusSent)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	// Its own subject and body, not session_revoked's: an eviction ends
	// one browser's session, and her other devices stay signed in.
	if sent[0].Subject != "Doula Cloud: you were signed out in one browser" {
		t.Fatalf("subject = %q", sent[0].Subject)
	}
}
