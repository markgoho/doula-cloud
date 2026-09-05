package sitebuild

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Queue records that the published site no longer matches what is
// stored. Called inside the same transaction as the write that made it
// stale, so a publish that rolls back queues no rebuild.
//
// Every reason queues the same row -- a first publish, an edit, and a
// switch away to her own website, which has to prune her page and is
// therefore just as much a reason to rebuild as publishing was.
func Queue(ctx context.Context, tx *sql.Tx, practiceID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO site_build_outbox (practice_id) VALUES ($1)`, practiceID)
	if err != nil {
		// coverage:ignore reason: DB insert failure, not exercised by unit tests
		return fmt.Errorf("sitebuild: queue rebuild: %w", err)
	}
	return nil
}

// Worker turns pending rows into one repository_dispatch.
type Worker struct {
	Dispatcher Dispatcher
	Now        Clock
}

// ProcessPending dispatches at most one deploy, for every pending row at
// once.
//
// The window is what makes "at once" mean anything. Nothing is
// dispatched until the oldest pending row has aged past CoalesceWindow;
// once it has, every pending row is claimed, including ones queued in
// the meantime. So two publishes a minute apart produce one deploy, and
// the nudge that arrives for the second one finds nothing left to do.
//
// A dispatch that fails leaves the rows pending and counts the attempt,
// so Cloud Scheduler's cadence retries it -- the durability half of
// ADR-0013, and the reason a nudge that never arrives loses nothing.
// Rows past MaxAttempts are dead-lettered rather than retried forever.
func (w Worker) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	// The other twelve outboxes claim their rows with FOR UPDATE SKIP
	// LOCKED, which no aggregate over "every pending row" can express.
	// This worker's unit of work is the whole pending set at once, so the
	// lock is taken over the whole worker instead: two runs that overlap
	// -- ADR-0013's nudge and the drain's cadence, which routinely land
	// together -- would otherwise both read the same rows as pending and
	// both fire a repository_dispatch for them.
	//
	// try rather than wait, and nil rather than an error, for the same
	// reason SKIP LOCKED returns no rows: a run that finds another run
	// already holding the set has nothing left to do, and that is a
	// success.
	var locked bool
	if err := tx.QueryRowContext(ctx,
		`SELECT pg_try_advisory_xact_lock($1)`, dispatchLockKey).Scan(&locked); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sitebuild: lock rebuild dispatch: %w", err)
	}
	if !locked {
		return nil
	}

	var (
		oldest   time.Time
		attempts int
	)
	err := tx.QueryRowContext(ctx,
		`SELECT min(created_at), max(attempt_count) FROM site_build_outbox WHERE status = 'pending'`,
	).Scan(&nullTime{&oldest}, &nullInt{&attempts})
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("sitebuild: read pending rebuilds: %w", err)
	}
	if oldest.IsZero() {
		return nil
	}
	if w.Now().Sub(oldest) < CoalesceWindow {
		return nil
	}

	if err := w.Dispatcher.Dispatch(ctx); err != nil {
		return w.recordFailure(ctx, tx, attempts+1, err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE site_build_outbox
		    SET status = 'dispatched', dispatched_at = $1, last_error = NULL
		  WHERE status = 'pending'`, w.Now())
	if err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("sitebuild: mark rebuilds dispatched: %w", err)
	}
	return nil
}

// recordFailure counts the attempt against every pending row and
// dead-letters them once they have had MaxAttempts. It returns nil: the
// failure is recorded, not propagated, so the transaction commits the
// record of it rather than rolling it back and forgetting.
func (w Worker) recordFailure(ctx context.Context, tx *sql.Tx, attempt int, cause error) error {
	status := "pending"
	if attempt >= MaxAttempts {
		status = "dead_lettered"
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE site_build_outbox
		    SET attempt_count = $1, last_error = $2, status = $3::site_build_outbox_status
		  WHERE status = 'pending'`, attempt, cause.Error(), status)
	if err != nil {
		// coverage:ignore reason: DB update failure, not exercised by unit tests
		return fmt.Errorf("sitebuild: record dispatch failure: %w", err)
	}
	return nil
}

// nullTime and nullInt scan an aggregate over no rows, which Postgres
// returns as NULL rather than as no row at all. Local rather than
// sql.NullTime at each call site so the zero value means "nothing
// pending" and the caller reads as one condition.
type nullTime struct{ dst *time.Time }

func (n nullTime) Scan(src any) error {
	if t, ok := src.(time.Time); ok {
		*n.dst = t
	}
	return nil
}

type nullInt struct{ dst *int }

func (n nullInt) Scan(src any) error {
	if i, ok := src.(int64); ok {
		*n.dst = int(i)
	}
	return nil
}
