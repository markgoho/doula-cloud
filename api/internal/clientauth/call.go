package clientauth

import (
	"database/sql"
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// Call is what Middleware resolved for one portal request, handed to a
// handler built with Handler as typed values. Middleware has already
// parsed the path's Engagement id and confirmed this Client owns it, so
// nothing here needs a gate of its own.
type Call struct {
	Tx           *sql.Tx
	ClientID     string
	EngagementID string
	// IdentityUID is the signed-in Portal Account, the actor a Client's
	// write records.
	IdentityUID string
}

// Handler adapts fn to an http.Handler for a route mounted behind
// Middleware. It reads what Middleware placed on context once, answering
// 500 itself if Middleware never ran, so no handler carries that branch.
func Handler(fn func(w http.ResponseWriter, r *http.Request, c Call)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tx, hasTx := Tx(ctx)
		clientID, hasClient := ClientID(ctx)
		engagementID, hasEngagement := EngagementID(ctx)
		uid, hasUID := IdentityUID(ctx)
		if !hasTx || !hasClient || !hasEngagement || !hasUID {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		fn(w, r, Call{Tx: tx, ClientID: clientID, EngagementID: engagementID, IdentityUID: uid})
	})
}
