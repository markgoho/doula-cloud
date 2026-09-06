// Package sessionevict is #610's cross-population eviction in the shape
// a mint seam wants it: one call, on the caller's own transaction, that
// either refuses with a warning or leaves the other population's session
// deleted and its notice queued.
//
// It exists as its own package because three seams mint a session and
// all three need the same order -- refuse, then delete, then queue --
// and neither package that owns half of it can host the whole. authn
// owns the session store and the tier test but must not queue mail;
// sessionnotice owns the outbox but must not write HTTP refusals. This
// is the join, and it is the only thing in it.
//
// See authn/eviction.go for why an eviction is disclosed rather than
// prevented, and what the refusal is allowed to say.
package sessionevict

import (
	"database/sql"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionnotice"
)

// msgInternalError is the body a caller sees when the failure is the
// BFF's own -- an unreachable database, not a bad credential.
const msgInternalError = "internal error"

// Apply refuses, or evicts, the live session in the population other
// than minting.
//
// ok=false means Apply has already written the response and the caller
// must return without minting: either the caller holds a session in the
// other population and has not confirmed losing it (409, see
// authn.RefuseUnconfirmed), or the database could not be read. The
// caller's own deferred rollback is what undoes anything it did before
// this point -- which is why a magic-link redeem may safely spend its
// token first and ask afterwards.
//
// ok=true with queued=true means a session was evicted and a notice for
// it is now pending, so the caller should nudge the session-notice
// outbox once its own commit succeeds. queued=false means either nothing
// was evicted or the evicted session was a Client's, which sends no mail
// -- sessionnotice.QueueSessionEvicted records why.
func Apply(w http.ResponseWriter, r *http.Request, tx *sql.Tx, minting authn.Tier, now time.Time) (queued bool, ok bool) {
	ev, found, err := authn.EvictionFor(r.Context(), tx, r, minting, now)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false, false
	}
	if !authn.RefuseUnconfirmed(w, r, ev, found) {
		return false, false
	}
	if !found {
		return false, true
	}

	// Deleted, not left to expire: the cookie is being overwritten
	// either way, and a live row behind it is a token that still
	// verifies (#610's own AC).
	if err := authn.EndSession(r.Context(), tx, ev.Token); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false, false
	}
	queued, err = sessionnotice.QueueSessionEvicted(r.Context(), tx, ev)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, msgInternalError, http.StatusInternalServerError)
		return false, false
	}
	return queued, true
}
