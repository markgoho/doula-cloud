package authmail

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/outbox"
)

// ProcessTokenMailOutboxHandler is the internal endpoint Cloud Scheduler
// invokes on a fixed cadence to run TokenMailWorker.ProcessPending.
func ProcessTokenMailOutboxHandler(db *sql.DB, worker TokenMailWorker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}

// ProcessEmailChangeOutboxHandler is the internal endpoint Cloud
// Scheduler invokes on a fixed cadence to run
// EmailChangeWorker.ProcessPending.
func ProcessEmailChangeOutboxHandler(db *sql.DB, worker EmailChangeWorker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}
