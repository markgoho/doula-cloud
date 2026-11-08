package clientauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// RequireTx resolves the request-scoped tx Middleware placed on context,
// writing an internal error response if it is missing. Shared by the
// downstream BFF packages mounted behind Middleware.
func RequireTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, ok bool) {
	tx, has := Tx(r.Context())
	if !has {
		// coverage:ignore reason: Middleware always sets a tx before this handler runs
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return nil, false
	}
	return tx, true
}

// RequireEngagementTx is RequireTx and the Engagement id Middleware
// resolved from the path, for the routes keyed on one Engagement.
func RequireEngagementTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, engagementID string, ok bool) {
	tx, ok = RequireTx(w, r)
	// coverage:ignore reason: Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", false
	}
	engagementID, _ = EngagementID(r.Context())
	return tx, engagementID, true
}

// RequireClientTx is RequireTx and the Client id Middleware resolved, for
// the routes keyed on the Client rather than on one Engagement.
func RequireClientTx(w http.ResponseWriter, r *http.Request) (tx *sql.Tx, clientID string, ok bool) {
	tx, ok = RequireTx(w, r)
	// coverage:ignore reason: Middleware always sets a tx before this handler runs
	if !ok {
		return nil, "", false
	}
	clientID, _ = ClientID(r.Context())
	return tx, clientID, true
}
