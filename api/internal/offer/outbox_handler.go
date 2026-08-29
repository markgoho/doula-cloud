package offer

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/outbox"
)

// ProcessOutboxHandler is the internal endpoint Cloud Scheduler invokes
// on a fixed cadence to run Worker.ProcessPending -- delegates to
// outbox.ProcessHandler, which owns the secret check, the trusted-worker
// session var, and the commit/rollback shape every mail kind shares
// (ADR-0010).
func ProcessOutboxHandler(db *sql.DB, worker Worker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}
