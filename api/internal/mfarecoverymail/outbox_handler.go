package mfarecoverymail

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/outbox"
)

// ProcessOutboxHandler is the internal endpoint Cloud Scheduler (and
// tasknudge's Cloud Tasks nudge) invoke to run Worker.ProcessPending.
func ProcessOutboxHandler(db *sql.DB, worker Worker, secret string) http.Handler {
	return outbox.ProcessHandler(db, worker, secret)
}
