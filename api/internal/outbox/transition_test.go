package outbox_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// TestWorker_ARowLeavesPendingOnce proves every Mark* method is a move
// from its declared state, not a write that lands whatever the row is
// in: a second move off pending, on a row the first one already moved,
// is refused with ErrNotInState rather than overwriting what the first
// recorded.
func TestWorker_ARowLeavesPendingOnce(t *testing.T) {
	second := map[string]func(w outbox.Worker, tx *sql.Tx) error{
		"sent again": func(w outbox.Worker, tx *sql.Tx) error {
			return w.MarkSent(t.Context(), tx, rowID, time.Now())
		},
		"skipped": func(w outbox.Worker, tx *sql.Tx) error {
			return w.MarkSkipped(t.Context(), tx, rowID, outbox.SkipAlreadyResolved, time.Now())
		},
		"dead-lettered": func(w outbox.Worker, tx *sql.Tx) error {
			return w.MarkDeadLetteredNow(t.Context(), tx, rowID, testDeadReason)
		},
		"retried": func(w outbox.Worker, tx *sql.Tx) error {
			return w.MarkFailed(t.Context(), tx, rowID, 0, errTransient, time.Now())
		},
	}
	for name, mark := range second {
		t.Run(name, func(t *testing.T) {
			env := newRowEnv(t, 0)
			w := newTestWorker(&mail.FakeSender{})
			env.inTx(func(tx *sql.Tx) error { return w.MarkSent(t.Context(), tx, rowID, time.Now()) })

			tx := env.DB.BeginAdmin(t)
			if err := mark(w, tx); !errors.Is(err, outbox.ErrNotInState) {
				t.Fatalf("err = %v, want ErrNotInState", err)
			}
			_ = tx.Rollback()
			env.expect(rowWant{status: testStatusSent})
		})
	}
}

// TestWorker_MarkReported covers the one move off sent: Mailgun's report
// of a bounce or a complaint, and nothing else.
func TestWorker_MarkReported(t *testing.T) {
	env := newRowEnv(t, 0)
	w := newTestWorker(&mail.FakeSender{})

	tx := env.DB.BeginAdmin(t)
	if err := w.MarkReported(t.Context(), tx, rowID, outbox.TransitionBounce); !errors.Is(err, outbox.ErrNotInState) {
		t.Fatalf("report on a pending row: err = %v, want ErrNotInState", err)
	}
	_ = tx.Rollback()

	env.inTx(func(tx *sql.Tx) error { return w.MarkSent(t.Context(), tx, rowID, time.Now()) })
	tx = env.DB.BeginAdmin(t)
	if err := w.MarkReported(t.Context(), tx, rowID, outbox.TransitionSend); err == nil || errors.Is(err, outbox.ErrNotInState) {
		t.Fatalf("report as a send: err = %v, want the refusal of a transition that is no delivery report", err)
	}
	_ = tx.Rollback()

	env.inTx(func(tx *sql.Tx) error { return w.MarkReported(t.Context(), tx, rowID, outbox.TransitionComplain) })
	env.expect(rowWant{status: string(outbox.StatusComplained)})
}

// pendingIDs is the selection an act outside a worker hands LockPending:
// every pending row, locked.
const pendingIDs = `SELECT id FROM outbox_test_rows WHERE status = 'pending' ORDER BY id FOR UPDATE`

// TestWorker_ResolvingPendingRowsFromOutsideAWorker proves the two ways an
// act that is not a worker -- a login deletion, a revoked invitation --
// moves the rows queued for it: each selected row goes through the same
// declared move a worker makes, exit duty and all, and a row the
// selection reaches that is no longer pending stops the act rather than
// being written over.
func TestWorker_ResolvingPendingRowsFromOutsideAWorker(t *testing.T) {
	setup := func(t *testing.T) *testdb.DB {
		t.Helper()
		db := testdb.New(t)
		createTestTable(t, db)
		insertTestRow(t, db, "a", 0, 0)
		insertTestRow(t, db, "b", 0, time.Hour)
		return db
	}

	t.Run("skip", func(t *testing.T) {
		db := setup(t)
		w := newTestWorker(&mail.FakeSender{}, secretA)
		(&rowEnv{T: t, DB: db}).inTx(func(tx *sql.Tx) error {
			return w.SkipPending(t.Context(), tx, pendingIDs, outbox.SkipLoginDeleted, time.Now())
		})
		for _, id := range []string{"a", "b"} {
			got := readTestRow(t, db, id)
			if got.status != testStatusSkipped || got.skipReason.String != string(outbox.SkipLoginDeleted) || got.secretA.Valid {
				t.Errorf("%s = %+v, want skipped for login_deleted with secret_a cleared", id, got)
			}
		}
	})

	t.Run("dead-letter", func(t *testing.T) {
		db := setup(t)
		w := newTestWorker(&mail.FakeSender{}, secretB)
		(&rowEnv{T: t, DB: db}).inTx(func(tx *sql.Tx) error {
			return w.DeadLetterPending(t.Context(), tx, pendingIDs, testDeadReason)
		})
		for _, id := range []string{"a", "b"} {
			got := readTestRow(t, db, id)
			if got.status != testStatusDead || got.reason.String != string(outbox.OwnData) || got.secretB.Valid {
				t.Errorf("%s = %+v, want dead-lettered for own_data with secret_b cleared", id, got)
			}
		}
	})

	t.Run("a selected row no longer pending stops the act", func(t *testing.T) {
		db := setup(t)
		w := newTestWorker(&mail.FakeSender{})
		(&rowEnv{T: t, DB: db}).inTx(func(tx *sql.Tx) error { return w.MarkSent(t.Context(), tx, "a", time.Now()) })
		const everyRow = `SELECT id FROM outbox_test_rows ORDER BY id FOR UPDATE`
		for name, act := range map[string]func(*sql.Tx) error{
			outbox.TransitionSkip.Name: func(tx *sql.Tx) error {
				return w.SkipPending(t.Context(), tx, everyRow, outbox.SkipLoginDeleted, time.Now())
			},
			outbox.TransitionDeadLetter.Name: func(tx *sql.Tx) error { return w.DeadLetterPending(t.Context(), tx, everyRow, testDeadReason) },
		} {
			tx := db.BeginAdmin(t)
			if err := act(tx); !errors.Is(err, outbox.ErrNotInState) {
				t.Errorf("%s: err = %v, want ErrNotInState", name, err)
			}
			_ = tx.Rollback()
		}
	})

	t.Run("a selection that is not one id per row fails", func(t *testing.T) {
		db := setup(t)
		w := newTestWorker(&mail.FakeSender{})
		const twoColumns = `SELECT id, status FROM outbox_test_rows FOR UPDATE`
		for name, act := range map[string]func(*sql.Tx) error{
			outbox.TransitionSkip.Name: func(tx *sql.Tx) error {
				return w.SkipPending(t.Context(), tx, twoColumns, outbox.SkipLoginDeleted, time.Now())
			},
			outbox.TransitionDeadLetter.Name: func(tx *sql.Tx) error { return w.DeadLetterPending(t.Context(), tx, twoColumns, testDeadReason) },
		} {
			tx := db.BeginAdmin(t)
			if err := act(tx); err == nil {
				t.Errorf("%s: err = nil, want the scan failure", name)
			}
			_ = tx.Rollback()
		}
	})
}
