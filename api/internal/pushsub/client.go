package pushsub

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
)

// ownerTypeClient is the push_subscriptions.owner_type value
// (00008_messaging.sql's actor_type enum) for the Client-portal
// population.
const ownerTypeClient = "client"

// ClientRegisterHandler mirrors RegisterHandler for the Client-portal
// population: registers or updates the calling Client's push subscription
// for the current device/browser. EngagementID is only present in the URL
// because clientauth.Middleware requires it, the same as every other
// Client-portal endpoint -- subscriptions are scoped to the Client
// identity, not a single Engagement. Must be mounted behind
// clientauth.Middleware.
func ClientRegisterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, clientID, ok := clientauth.RequireClientTx(w, r)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}

		req, ok := decodeSubscribeRequest(w, r)
		if !ok {
			return
		}

		if err := upsertSubscription(r, tx, ownerTypeClient, clientID, req); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ClientUnregisterHandler mirrors UnregisterHandler for the Client-portal
// population. Must be mounted behind clientauth.Middleware.
func ClientUnregisterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, clientID, ok := clientauth.RequireClientTx(w, r)
		if !ok {
			// coverage:ignore reason: the helper already wrote the failure response; reached only on a DB failure or a missing Middleware value, not exercised by unit tests
			return
		}

		endpoint, ok := requireEndpointQueryParam(w, r)
		if !ok {
			return
		}

		if err := deleteSubscription(r, tx, ownerTypeClient, clientID, endpoint); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
