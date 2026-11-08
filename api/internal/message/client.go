package message

import (
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
)

// ClientListHandler mirrors ListHandler for the Client-portal population:
// same query, same pagination and ordering, but scoped by
// clientauth.Middleware's already-authorized Engagement rather than a
// Staff practice-tier check -- clientauth.Middleware has already 403'd
// any Engagement the caller doesn't own, so no extra ownership check is
// needed here. Must be mounted behind clientauth.Middleware.
//
// Passes activity.StaffActorDisplayName into listMessages so a thread
// whose sender is a Doula who deleted her own login (#1198) reads "Your
// practice" rather than a blank name: 00111's client_portal_sees_staff
// policy refuses that redacted row to a Client population on purpose
// (ADR-0033), the same gap listPortalVisits already fills for the
// Visits screen. ListHandler, the Staff-side sibling, passes "" and is
// unchanged -- see listMessages' own doc comment for what that side
// actually shows today, which is not what this ticket assumed.
func ClientListHandler() http.Handler {
	return clientauth.Handler(func(w http.ResponseWriter, r *http.Request, c clientauth.Call) {
		writeThreadPage(w, r, c.Tx, c.EngagementID, activity.StaffActorDisplayName)
	})
}

// ClientCreateHandler mirrors CreateHandler for the Client-portal
// population: posts a Message, with an optional single image/PDF
// attachment, as the calling Client into the caller's own Engagement
// thread. On success, notifies the Practice's Staff push subscription(s)
// (#61) before responding. Must be mounted behind clientauth.Middleware.
//
// Mounted through idempotency.Router.PortalReplayable (#1924), which
// wraps it in idempotency.WrapClient: a retry carrying the same
// Idempotency-Key replays the first response, keyed to the calling Client,
// instead of posting the Message twice.
func ClientCreateHandler(store objectstore.ObjectStore, pusher push.Pusher) http.Handler {
	return clientauth.Handler(func(w http.ResponseWriter, r *http.Request, c clientauth.Call) {
		postMessage(w, r, c.Tx, store, pusher, c.EngagementID, senderTypeClient, c.ClientID)
	})
}
