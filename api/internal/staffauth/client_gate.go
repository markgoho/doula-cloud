package staffauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// MsgClientNotFound is the one thing a caller is told when a Client is not
// hers to reach: it does not exist at the Practice, or ADR-0017's rule
// keeps it from a contractor Doula who holds no attachment to it.
const MsgClientNotFound = "client not found"

// RequireClientReach is the preamble of every route under
// /clients/{clientId}: the request-scoped tx, the Practice, the caller's
// Reader, and the path's Client id, parsed and confirmed reachable by that
// Reader (Reader.CanAccessClient). It writes the 400, 404 or 500 itself.
func RequireClientReach(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, practiceID, clientID string, reader Reader, ok bool) {
	tx, practiceID, reader, ok = RequireTxReader(w, r)
	// coverage:ignore reason: Middleware always sets a tx and a Reader before this handler runs
	if !ok {
		return nil, "", "", Reader{}, false
	}
	clientID = r.PathValue("clientId")
	if !ParseUUID(w, "client", clientID) {
		return nil, "", "", Reader{}, false
	}
	if !RequireClientAccess(w, r, tx, reader, clientID) {
		return nil, "", "", Reader{}, false
	}
	return tx, practiceID, clientID, reader, true
}

// RequireClientAccess confirms reader may reach clientID, writing the 404
// MsgClientNotFound when she may not.
func RequireClientAccess(w http.ResponseWriter, r *http.Request, tx *sql.Tx, reader Reader, clientID string) bool {
	canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return false
	}
	if !canAccess {
		apierr.WriteError(w, MsgClientNotFound, http.StatusNotFound)
		return false
	}
	return true
}
