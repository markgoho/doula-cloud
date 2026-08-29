package outbox

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
)

// MsgInternalError is the body a caller sees for a failure that carries
// no more specific detail. Defined here rather than borrowed from
// staffauth so every mail kind's ProcessOutboxHandler can delegate to
// ProcessHandler regardless of whether staffauth already imports that
// kind's package (staffinvite, sessionnotice) -- an import cycle this
// package, importing neither, never risks.
const MsgInternalError = "internal error"

// Processor is what ProcessHandler needs from a mail kind's Worker: the
// method every one of them already exposes.
type Processor interface {
	ProcessPending(ctx context.Context, tx *sql.Tx) error
}

// ProcessHandler is the internal endpoint Cloud Scheduler invokes on a
// fixed cadence (and ADR-0013's nudge on demand) to run
// worker.ProcessPending -- one instance per mail kind, mirroring the
// per-Notification-type-cost ADR-0010 accepted, since each worker
// processes its own outbox table. There is no Staff or Client session on
// this path, so it authenticates the caller against secret instead.
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
func ProcessHandler(db *sql.DB, worker Processor, secret string) http.Handler {
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

		// Licenses every mail kind's own staff/practice_memberships (or
		// equivalent) SELECT policies for reading recipients at send time --
		// each kind's own migration adds the policy, this sets the session
		// var every one of them checks.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
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
