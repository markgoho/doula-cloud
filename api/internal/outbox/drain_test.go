package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// The stand-in addresses these tests register outboxes at. Like
// portalInvitePath they are not real endpoints -- what is under test is
// the registry's own behavior, and the addresses the BFF actually serves
// are pinned in api/outboxes_test.go.
const (
	firstPath   = "/first"
	secondPath  = "/second"
	onlyPath    = "/only"
	brokenPath  = "/broken"
	healthyPath = "/healthy"
)

// newDrainServer mounts DrainHandler alone, at its own address, so these
// tests drive the drain rather than the per-outbox endpoints beside it.
func newDrainServer(db *testdb.DB, secret string, registrations []outbox.Registration) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST "+outbox.DrainPath, outbox.DrainHandler(db.App, secret, registrations))
	return httptest.NewServer(mux)
}

// TestDrainHandler_RunsEveryRegistration is the whole point of the
// endpoint: one Cloud Scheduler job is the backstop for all of them, so
// none may be skipped.
func TestDrainHandler_RunsEveryRegistration(t *testing.T) {
	db := testdb.New(t)
	first, second, third := &stubProcessor{}, &stubProcessor{}, &stubProcessor{}

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: firstPath, Door: outbox.NotificationDoor, Worker: first},
		{Path: secondPath, Door: outbox.NotificationDoor, Worker: second},
		{Path: "/third", Worker: third},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	for name, worker := range map[string]*stubProcessor{"first": first, "second": second, "third": third} {
		if !worker.called {
			t.Errorf("%s outbox was not drained", name)
		}
	}
}

// TestDrainHandler_CarriesOnPastAFailingWorker is the failure this shape
// has to survive: thirteen outboxes now share one job, so one that is
// broken must not stop the twelve that are not.
func TestDrainHandler_CarriesOnPastAFailingWorker(t *testing.T) {
	db := testdb.New(t)
	before, after := &stubProcessor{}, &stubProcessor{}

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: "/before", Door: outbox.NotificationDoor, Worker: before},
		{Path: brokenPath, Door: outbox.NotificationDoor, Worker: &stubProcessor{err: errors.New("boom")}},
		{Path: "/after", Door: outbox.NotificationDoor, Worker: after},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "correct-secret")
	defer resp.Body.Close()

	if !before.called {
		t.Error("the outbox before the broken one was not drained")
	}
	if !after.called {
		t.Error("the outbox after the broken one was not drained")
	}
}

// TestDrainHandler_ReportsTheFailingOutboxes is what makes one job's
// last-run status readable: Cloud Scheduler shows the job red, and the
// body says which outboxes made it red.
func TestDrainHandler_ReportsTheFailingOutboxes(t *testing.T) {
	db := testdb.New(t)

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: healthyPath, Door: outbox.NotificationDoor, Worker: &stubProcessor{}},
		{Path: brokenPath, Door: outbox.NotificationDoor, Worker: &stubProcessor{err: errors.New("boom")}},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "/broken") {
		t.Errorf("body = %s, want it to name /broken", body)
	}
	if strings.Contains(string(body), "/healthy") {
		t.Errorf("body = %s, want it not to name the outbox that succeeded", body)
	}
}

// TestDrainHandler_OpensEachRegistrationsOwnDoor proves the drain does
// not hand every worker the same door: #443's site rebuild is not under
// RLS and must be run without one, exactly as its own endpoint runs it.
func TestDrainHandler_OpensEachRegistrationsOwnDoor(t *testing.T) {
	db := testdb.New(t)
	mailing, doorless := &doorReadingProcessor{}, &doorReadingProcessor{}

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: "/mailing", Door: outbox.NotificationDoor, Worker: mailing},
		{Path: "/doorless", Worker: doorless},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if mailing.door != "true" {
		t.Errorf("mailing outbox saw the door as %q, want %q", mailing.door, "true")
	}
	if doorless.door != "" {
		t.Errorf("doorless outbox saw the door as %q, want it unset", doorless.door)
	}
}

// TestDrainHandler_GivesEachWorkerItsOwnTransaction is what makes
// carrying on past a failure safe rather than merely quiet: a worker that
// fails rolls back its own transaction, and the rows every other worker
// wrote are already committed in transactions of their own.
func TestDrainHandler_GivesEachWorkerItsOwnTransaction(t *testing.T) {
	db := testdb.New(t)
	first, second := &txReadingProcessor{}, &txReadingProcessor{}

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: firstPath, Door: outbox.NotificationDoor, Worker: first},
		{Path: brokenPath, Door: outbox.NotificationDoor, Worker: &stubProcessor{err: errors.New("boom")}},
		{Path: secondPath, Door: outbox.NotificationDoor, Worker: second},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "correct-secret")
	defer resp.Body.Close()

	if first.txID == "" || second.txID == "" {
		t.Fatalf("transaction ids = %q and %q, want both read", first.txID, second.txID)
	}
	if first.txID == second.txID {
		t.Errorf("both workers ran in transaction %s, want one each", first.txID)
	}
}

// txReadingProcessor reads the id of the transaction it was handed, which
// is the only way to tell one transaction from another from inside a
// worker.
type txReadingProcessor struct {
	txID string
}

func (p *txReadingProcessor) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	// The virtual transaction id rather than the real one: a real xid is
	// assigned only once a transaction writes, and these workers only
	// read. Every transaction has a virtual id, and no two live at once
	// share it.
	if err := tx.QueryRowContext(ctx,
		`SELECT virtualxid FROM pg_locks WHERE locktype = 'virtualxid' AND pid = pg_backend_pid()`,
	).Scan(&p.txID); err != nil {
		return fmt.Errorf("read the transaction id: %w", err)
	}
	return nil
}

func TestDrainHandler_RefusesTheWrongSecret(t *testing.T) {
	db := testdb.New(t)
	worker := &stubProcessor{}

	srv := newDrainServer(db, "correct-secret", []outbox.Registration{
		{Path: onlyPath, Door: outbox.NotificationDoor, Worker: worker},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "wrong-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if worker.called {
		t.Error("a worker ran despite the wrong secret")
	}
}

// TestDrainHandler_RefusesWhenNoSecretIsConfigured pins the same choice
// ProcessHandler makes: an unset NOTIFICATION_WORKER_SECRET refuses every
// caller rather than accepting an unauthenticated one.
func TestDrainHandler_RefusesWhenNoSecretIsConfigured(t *testing.T) {
	db := testdb.New(t)
	worker := &stubProcessor{}

	srv := newDrainServer(db, "", []outbox.Registration{
		{Path: onlyPath, Door: outbox.NotificationDoor, Worker: worker},
	})
	defer srv.Close()

	resp := postTo(t, srv, outbox.DrainPath, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if worker.called {
		t.Error("a worker ran with no secret configured")
	}
}
