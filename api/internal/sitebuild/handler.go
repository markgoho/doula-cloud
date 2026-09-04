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

// VerifyHandler is the internal endpoint that probes every published
// page and records whether it resolved.
//
// Two callers, both authenticated by secret rather than a session, and
// deliberately the same behavior for both: the deploy workflow POSTs it
// once as its last step, and Cloud Scheduler calls it on a cadence. The
// cadence is what makes AC5 hold -- a build that fails produces no deploy
// and no callback at all, so only something that runs anyway can notice
// that a page never went live.
//
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
//
// This package's rebuild outbox used to sit beside it on this same shape.
// It is now one entry in the BFF's outbox list and runs through
// outbox.ProcessHandler like every other, which is why what remains here
// serves one endpoint rather than two.
func VerifyHandler(db *sql.DB, verifier Verifier, secret string) http.Handler {
	return internalHandler(db, secret, verifier.Verify)
}

// internalHandler is the secret-checked, one-transaction shape the verify
// endpoint runs on. It always opens the site worker door, which licenses
// 00049's policies on practice_websites -- the probe is the only thing
// left here and it is the only thing that ever needed it.
func internalHandler(db *sql.DB, secret string, run func(context.Context, *sql.Tx) error) http.Handler {
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

		if _, err := tx.ExecContext(r.Context(),
			`SELECT set_config('app.site_worker_trusted', 'true', true)`); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := run(r.Context(), tx); err != nil {
			// coverage:ignore reason: the verifier's only failure mode is a DB error, not exercised by unit tests
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
