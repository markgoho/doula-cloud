package outbox

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"doula-cloud/api/internal/apierr"
	"net/http"
)

// MsgInternalError is the body a caller sees for a failure that carries
// no more specific detail. Defined here rather than borrowed from
// staffauth so this package can serve every outbox regardless of whether
// staffauth already imports that outbox's own package (staffinvite,
// sessionnotice) -- an import cycle this package, importing neither,
// never risks.
const MsgInternalError = "internal error"

// Processor is what ProcessHandler needs from an outbox's Worker: the
// method every one of them already exposes.
type Processor interface {
	ProcessPending(ctx context.Context, tx *sql.Tx) error
}

// ProcessHandler is the internal endpoint ADR-0013's nudge invokes on
// demand, and the address a person invokes by hand, to run
// worker.ProcessPending -- one instance per outbox, mirroring the
// per-Notification-type cost ADR-0010 accepted, since each worker
// processes its own outbox table. There is no Staff or Client session on
// this path, so it authenticates the caller against secret instead.
// secret must be non-empty: an empty configured secret refuses every
// request rather than accepting an unauthenticated one.
//
// Cloud Scheduler no longer calls these one by one: DrainPath's single
// job runs every registration, for the reason recorded there and on
// ADR-0010.
//
// door is the Postgres session variable to set for the length of the
// transaction, or empty for none -- see Registration.Door. Callers
// normally reach this through Register rather than directly.
func ProcessHandler(db *sql.DB, worker Processor, secret, door string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			apierr.WriteError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if err := runOutbox(r.Context(), db, worker, door); err != nil {
			apierr.WriteError(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
