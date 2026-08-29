package payments

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/outbox"
)

// ProcessPaymentOutboxHandler is the internal endpoint Cloud Scheduler
// invokes on a fixed cadence to run PaymentReceivedWorker.ProcessPending
// -- delegates to outbox.ProcessHandler, which owns the secret check, the
// trusted-worker session var, and the commit/rollback shape every mail
// kind shares (ADR-0010).
func ProcessPaymentOutboxHandler(db *sql.DB, worker PaymentReceivedWorker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}
