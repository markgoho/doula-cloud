package outbox

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
)

// DrainPath is the one endpoint Cloud Scheduler is provisioned against:
// a single job on a fixed cadence that runs every registered outbox.
//
// ADR-0010 asked for "a separate internal endpoint, invoked by Cloud
// Scheduler on a fixed cadence", and this repository read that as one job
// per outbox. Provisioned by hand against a console, that shape drifts:
// #481 found ten of thirteen outboxes with no job at all, four of which
// were added after the ticket was filed and never provisioned. Draining
// from the registry instead means a fourteenth outbox gets its backstop
// by being registered, with no console change at all.
//
// The per-outbox endpoints stay mounted beside it -- they are ADR-0013's
// nudge targets, and the address a person invokes by hand.
const DrainPath = "/api/internal/outboxes/drain"

// DrainWorkerTimeout bounds one worker's turn, so a single wedged outbox
// cannot eat the whole attempt deadline and starve every outbox after
// it. Cloud Scheduler's job allows 180 seconds and thirteen workers at 30
// apiece could exceed that, which is the intended trade: the request's
// own context ends the drain, every worker that finished has committed
// its own transaction, and the ones that never ran are picked up on the
// next tick.
const DrainWorkerTimeout = 30 * time.Second

// runOutbox runs one worker inside its own transaction, with its own
// door set for the length of it, and commits.
//
// door is the Postgres session variable that licenses the outbox's own
// staff/practice_memberships (or equivalent) SELECT policies for reading
// recipients at send time -- each kind's own migration adds the policy,
// this sets the variable every one of them checks. Empty for an outbox
// that is not under RLS, which should not be handed a door it has no use
// for.
//
// door is a compile-time constant a Registration names, never request
// data, so building it into SQL carries no injection risk -- the same
// reasoning Worker.Table already rests on.
func runOutbox(ctx context.Context, db *sql.DB, worker Processor, door string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("outbox: begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if door != "" {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`SELECT set_config('%s', 'true', true)`, door)); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("outbox: set door %s: %w", door, err)
		}
	}

	if err := worker.ProcessPending(ctx, tx); err != nil {
		return fmt.Errorf("outbox: process pending: %w", err)
	}

	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: DB commit failure, not exercised by unit tests
		return fmt.Errorf("outbox: commit: %w", err)
	}
	committed = true
	return nil
}

// DrainHandler is ADR-0013's durability backstop: the endpoint one Cloud
// Scheduler job calls on a fixed cadence to run every registered outbox
// in turn. It authenticates the caller against secret rather than a
// session, exactly as the per-outbox endpoints beside it do.
//
// Every outbox gets its turn regardless of what the ones before it did.
// Each runs in its own transaction, so a worker that fails rolls back
// only its own rows and the drain carries on to the next one, and each
// failure is logged against the outbox it belongs to -- one job's
// last-run status is now the only place thirteen outboxes report from,
// and a broken one must not disappear into a green tick.
//
// The response tells Cloud Scheduler what the log tells a person: 200
// only when every outbox succeeded, and 500 naming the ones that did not,
// so the job shows red and its retry policy fires.
func DrainHandler(db *sql.DB, secret string, registrations []Registration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			apierr.WriteError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var failed []string
		for _, reg := range registrations {
			ctx, cancel := context.WithTimeout(r.Context(), DrainWorkerTimeout)
			err := runOutbox(ctx, db, reg.Worker, reg.Door)
			cancel()
			if err != nil {
				log.Printf("outbox: drain %s: %v", reg.Path, err)
				failed = append(failed, reg.Path)
			}
		}

		if len(failed) > 0 {
			apierr.WriteError(w, "drain failed: "+strings.Join(failed, ", "), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
