package idempotency

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/clock"
)

// Sweeper deletes every idempotency_keys row past ttl, at every Practice
// (#1987). It is an outbox.Processor so the outbox drain -- the one Cloud
// Scheduler tick, every five minutes (ADR-0010 as amended by #481) --
// runs it with the rest, and no Scheduler job of its own is needed.
//
// The delete is sweep_expired_idempotency_keys (00130), which refuses a
// transaction without the worker door the drain sets: no single caller's
// policy branch can see another Practice's rows, and the function lets
// the sweep delete them without letting it read one.
type Sweeper struct{}

// ProcessPending deletes every row created before clock.Now(ctx) less
// ttl -- expiredBefore, the same line lookup draws (#1991).
func (Sweeper) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	cutoff := expiredBefore(clock.Now(ctx))
	if _, err := tx.ExecContext(ctx, `SELECT sweep_expired_idempotency_keys($1)`, cutoff); err != nil {
		return fmt.Errorf("idempotency: sweep expired keys: %w", err)
	}
	return nil
}
