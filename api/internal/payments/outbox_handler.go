package payments

import (
	"crypto/subtle"
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/staffauth"
)

// ProcessOutboxHandler is the internal endpoint Cloud Scheduler invokes on
// a fixed cadence to run Worker.ProcessPending, mirroring
// billing.ProcessOutboxHandler's shape -- a separate endpoint per
// ADR-0010's accepted per-Notification-type cost, since this worker
// processes a table of its own. There is no Staff or Client session on
// this path, so it authenticates the caller against secret instead.
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
func ProcessOutboxHandler(db *sql.DB, worker Worker, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
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

		// Licenses ownerEmails' staff/practice_memberships reads via
		// 00033's trusted-worker policies -- the same session var
		// billing.ProcessOutboxHandler sets for its own worker.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := worker.ProcessPending(r.Context(), tx); err != nil {
			// coverage:ignore reason: ProcessPending's only failure mode is
			// a DB failure (outbox.go), not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true
		w.WriteHeader(http.StatusOK)
	})
}
