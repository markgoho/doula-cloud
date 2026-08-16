package pushsub

import (
	"net/http"

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
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		clientID, _ := clientauth.ClientID(r.Context())

		req, ok := decodeSubscribeRequest(w, r)
		if !ok {
			return
		}

		if err := upsertSubscription(r, tx, ownerTypeClient, clientID, req); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ClientUnregisterHandler mirrors UnregisterHandler for the Client-portal
// population. Must be mounted behind clientauth.Middleware.
func ClientUnregisterHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		clientID, _ := clientauth.ClientID(r.Context())

		endpoint, ok := requireEndpointQueryParam(w, r)
		if !ok {
			return
		}

		if err := deleteSubscription(r, tx, ownerTypeClient, clientID, endpoint); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
