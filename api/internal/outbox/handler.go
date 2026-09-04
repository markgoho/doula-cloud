package outbox

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
)

// MsgInternalError is the body a caller sees for a failure that carries
// no more specific detail. Defined here rather than borrowed from
// staffauth so this package can serve every outbox regardless of whether
// staffauth already imports that outbox's own package (staffinvite,
// sessionnotice) -- an import cycle this package, importing neither,
// never risks.
const MsgInternalError = "internal error"

// Processor is what ProcessHandler needs from an outbox's Worker: the
// method every one of them already exposes.
type Processor interface {
	ProcessPending(ctx context.Context, tx *sql.Tx) error
}

// ProcessHandler is the internal endpoint Cloud Scheduler invokes on a
// fixed cadence (and ADR-0013's nudge on demand) to run
// worker.ProcessPending -- one instance per outbox, mirroring the
// per-Notification-type-cost ADR-0010 accepted, since each worker
// processes its own outbox table. There is no Staff or Client session on
// this path, so it authenticates the caller against secret instead.
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
//
// door is the Postgres session variable to set for the length of the
// transaction, or empty for none -- see Registration.Door. Callers
// normally reach this through Register rather than directly.
func ProcessHandler(db *sql.DB, worker Processor, secret, door string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				// coverage:ignore reason: only reached by a DB failure after
				// BeginTx succeeds -- this endpoint takes no request input
				// past the secret check above.
				_ = tx.Rollback()
			}
		}()

		// Licenses the outbox's own staff/practice_memberships (or
		// equivalent) SELECT policies for reading recipients at send time --
		// each one's own migration adds the policy, this sets the session
		// var it checks. Skipped entirely for an outbox that is not under
		// RLS, which should not be handed a door it has no use for.
		//
		// door is a compile-time constant a Registration names, never
		// request data, so building it into SQL carries no injection risk
		// -- the same reasoning Worker.Table already rests on.
		if door != "" {
			if _, err := tx.ExecContext(r.Context(), `SELECT set_config('`+door+`', 'true', true)`); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		if err := worker.ProcessPending(r.Context(), tx); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true
		w.WriteHeader(http.StatusOK)
	})
}
