package sitebuild

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
)

// MsgInternalError is this package's response body for a failure the
// caller can't act on.
const MsgInternalError = "internal error"

// ProcessOutboxHandler is the internal endpoint that turns queued
// rebuilds into one deploy, on the shape ADR-0010's seven notification
// workers already use. Two callers, both authenticated by secret rather
// than a session: Cloud Tasks nudges it CoalesceWindow after a publish
// (ADR-0013), and Cloud Scheduler polls it on a cadence as the
// durability backstop.
//
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
func ProcessOutboxHandler(db *sql.DB, worker Worker, secret string) http.Handler {
	return internalHandler(db, secret, false, worker.ProcessPending)
}

// VerifyHandler is the internal endpoint that probes every published
// page and records whether it resolved.
//
// The same two shapes of caller again, and deliberately the same
// behavior for both: the deploy workflow POSTs it once as its last step,
// and Cloud Scheduler calls it on a cadence. The cadence is what makes
// AC5 hold -- a build that fails produces no deploy and no callback at
// all, so only something that runs anyway can notice that a page never
// went live.
func VerifyHandler(db *sql.DB, verifier Verifier, secret string) http.Handler {
	return internalHandler(db, secret, true, verifier.Verify)
}

// internalHandler is the secret-checked, one-transaction shape both
// endpoints share. siteWorkerDoor licenses 00049's policies on
// practice_websites, which only the probe needs -- the outbox is not
// under RLS at all, and a worker should not be handed a door it has no
// use for.
func internalHandler(db *sql.DB, secret string, siteWorkerDoor bool, run func(context.Context, *sql.Tx) error) http.Handler {
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
				// coverage:ignore reason: only reached by a DB failure after BeginTx succeeds -- these endpoints take no request input past the secret check
				_ = tx.Rollback()
			}
		}()

		if siteWorkerDoor {
			if _, err := tx.ExecContext(r.Context(),
				`SELECT set_config('app.site_worker_trusted', 'true', true)`); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				http.Error(w, MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		if err := run(r.Context(), tx); err != nil {
			// coverage:ignore reason: both workers' only failure mode is a DB error, not exercised by unit tests
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
