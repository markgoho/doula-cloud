// Package pushsub owns the register/unregister endpoints for
// push_subscriptions (00008_messaging.sql), for both the Staff-side
// (staff.go) and Client-portal-side (client.go) populations -- the
// generic subscription-management layer #61 (Push notifications on new
// message) builds so a later notification type (Invoice-due,
// Contract-ready) can reuse it without rework. The Staff handlers rely on
// staffauth.Middleware having already resolved the caller's Staff id and
// opened a request-scoped *sql.Tx, the same way message's Staff handlers
// do; the Client-portal handlers rely on clientauth.Middleware the same
// way. Resolving a Message recipient's subscription(s) to actually send a
// push is a separate concern, handled by internal/message and
// internal/push, not this package.
package pushsub

import (
	"database/sql"
	"doula-cloud/api/internal/apierr"
	"encoding/json"
	"fmt"
	"net/http"
)

// SubscribeRequest is the body of a register-push-subscription request,
// matching the shape a browser's PushSubscription.toJSON() produces.
type SubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// decodeSubscribeRequest decodes and validates a SubscribeRequest body,
// shared by the Staff and Client-portal register handlers. Writes its own
// error response and returns ok=false on failure.
func decodeSubscribeRequest(w http.ResponseWriter, r *http.Request) (SubscribeRequest, bool) {
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
		return SubscribeRequest{}, false
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		apierr.WriteError(w, "endpoint and keys.p256dh and keys.auth are required", http.StatusBadRequest)
		return SubscribeRequest{}, false
	}
	return req, true
}

// upsertSubscription registers ownerType/ownerID's subscription, or
// updates it in place if endpoint was already registered (e.g. the
// browser re-subscribed the same device) -- endpoint is UNIQUE
// (00008_messaging.sql), so ON CONFLICT keys off it directly.
func upsertSubscription(r *http.Request, tx *sql.Tx, ownerType, ownerID string, req SubscribeRequest) error {
	_, err := tx.ExecContext(r.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (endpoint) DO UPDATE SET
			owner_type = excluded.owner_type,
			owner_id = excluded.owner_id,
			p256dh_key = excluded.p256dh_key,
			auth_key = excluded.auth_key`,
		ownerType, ownerID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("pushsub: upsert subscription: %w", err)
	}
	return nil
}

// deleteSubscription unregisters ownerType/ownerID's subscription at
// endpoint. A no-op (no error) if no such row exists -- unregistering an
// already-unregistered device isn't a failure.
func deleteSubscription(r *http.Request, tx *sql.Tx, ownerType, ownerID, endpoint string) error {
	_, err := tx.ExecContext(r.Context(),
		`DELETE FROM push_subscriptions WHERE owner_type = $1 AND owner_id = $2 AND endpoint = $3`,
		ownerType, ownerID, endpoint,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("pushsub: delete subscription: %w", err)
	}
	return nil
}

// requireEndpointQueryParam reads the "endpoint" query parameter a
// DELETE-unregister request carries, shared by the Staff and
// Client-portal unregister handlers. Writes its own error response and
// returns ok=false if missing.
func requireEndpointQueryParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		apierr.WriteError(w, "endpoint is required", http.StatusBadRequest)
		return "", false
	}
	return endpoint, true
}
