package portalinvite

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
)

// ProcessOutboxHandler is the internal endpoint Cloud Scheduler invokes on
// a fixed cadence to run Worker.ProcessPending (ADR-0010). There is no
// Staff or Client session on this path -- no staffauth.Middleware, no
// GatedRouter role declaration -- so it authenticates the caller against
// secret instead, the same shared-secret shape billing's webhook
// endpoints use a signature for. secret must be non-empty: an empty
// configured secret refuses every request rather than accepting an
// unauthenticated one.
func ProcessOutboxHandler(db *sql.DB, worker Worker, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				// coverage:ignore reason: only reached by a DB failure after
				// BeginTx succeeds -- this endpoint takes no request input
				// past the secret check above, so unlike accept.go's
				// rollback (reachable via an ordinary bad-token 400) there
				// is no non-DB way to drive this closed.
				_ = tx.Rollback()
			}
		}()

		// Licenses the client_portal_users/clients SELECT policies
		// 00032_portal_invite_outbox.sql added for this worker -- the
		// X-Internal-Secret check above is what stands in for the
		// membership check staffauth.Middleware would otherwise perform.
		if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := worker.ProcessPending(r.Context(), tx); err != nil {
			// coverage:ignore reason: ProcessPending's only failure mode is
			// a DB failure (outbox.go), not exercised by unit tests
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
