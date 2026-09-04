package client

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/outbox"
)

// ProcessErasureOutboxHandler is the internal endpoint Cloud Scheduler
// invokes on a fixed cadence -- and Cloud Tasks nudges immediately after
// an erasure -- to run ErasureWorker.ProcessPending. Delegates to
// outbox.ProcessHandler, which owns the secret check, the trusted-worker
// session var, and the commit/rollback shape every outbox shares
// (ADR-0010).
func ProcessErasureOutboxHandler(db *sql.DB, worker ErasureWorker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}
