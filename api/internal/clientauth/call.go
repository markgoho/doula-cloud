package clientauth

import (
	"database/sql"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
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
	ids         map[string]string
}

// ID returns the path id PathID declared under wildcard. It panics on a
// wildcard the handler never declared: that is a wiring mistake the first
// test of the handler meets, not a request a caller can cause.
func (c Call) ID(wildcard string) string {
	id, ok := c.ids[wildcard]
	if !ok {
		panic(fmt.Sprintf("clientauth: Call.ID(%q): no PathID declared for this wildcard", wildcard))
	}
	return id
}

// Option declares one path id a handler needs. It runs after Middleware and
// before the handler, writes the 400 itself when it refuses, and reports
// whether the handler may run.
type Option func(w http.ResponseWriter, r *http.Request, c *Call) (ok bool)

// PathID declares the path wildcard as a UUID, naming it label in the 400
// "invalid <label> id" staffauth.ParseUUID writes, and exposes it as
// Call.ID(wildcard).
func PathID(label, wildcard string) Option {
	return func(w http.ResponseWriter, r *http.Request, c *Call) bool {
		id := r.PathValue(wildcard)
		if !staffauth.ParseUUID(w, label, id) {
			return false
		}
		if c.ids == nil {
			c.ids = map[string]string{}
		}
		c.ids[wildcard] = id
		return true
	}
}

// Handler adapts fn to an http.Handler for a route mounted behind
// Middleware. It reads what Middleware placed on context once, answering
// 500 itself if Middleware never ran, so no handler carries that branch,
// then applies opts and calls fn.
func Handler(fn func(w http.ResponseWriter, r *http.Request, c Call), opts ...Option) http.Handler {
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
		c := Call{Tx: tx, ClientID: clientID, EngagementID: engagementID, IdentityUID: uid}
		for _, opt := range opts {
			if !opt(w, r, &c) {
				return
			}
		}
		fn(w, r, c)
	})
}
