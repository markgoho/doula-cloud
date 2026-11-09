package pushsub

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// ownerTypeStaff is the push_subscriptions.owner_type value
// (00008_messaging.sql's actor_type enum) for the Staff population.
const ownerTypeStaff = "staff"

// RegisterHandler registers or updates the calling Staff member's push
// subscription for the current device/browser. Subscriptions are scoped
// to the Staff identity, not the current Practice -- PracticeID is only
// present in the URL because staffauth.Middleware requires it, the same
// as every other Staff-side endpoint. Must be mounted behind
// staffauth.Middleware.
func RegisterHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		req, ok := decodeSubscribeRequest(w, r)
		if !ok {
			return
		}

		if err := upsertSubscription(r, c.Tx, ownerTypeStaff, c.StaffID, req); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// UnregisterHandler removes the calling Staff member's push subscription
// for a given endpoint. Must be mounted behind staffauth.Middleware.
func UnregisterHandler() http.Handler {
	return staffauth.Handler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call) {
		endpoint, ok := requireEndpointQueryParam(w, r)
		if !ok {
			return
		}

		if err := deleteSubscription(r, c.Tx, ownerTypeStaff, c.StaffID, endpoint); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
