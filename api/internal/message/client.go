package message

import (
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var after *messageCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeCursor(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		items, hasMore, err := listMessages(r.Context(), tx, engagementID, after, activity.StaffActorDisplayName)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeCursor(items[len(items)-1].CreatedAt, items[len(items)-1].MessageID)
			resp.NextCursor = &next
		}

		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}

// ClientCreateHandler mirrors CreateHandler for the Client-portal
// population: posts a Message, with an optional single image/PDF
// attachment, as the calling Client into the caller's own Engagement
// thread. On success, notifies the Practice's Staff push subscription(s)
// (#61) before responding. Must be mounted behind clientauth.Middleware.
//
// Not wrapped in idempotency.Wrap (#129): idempotency_keys
// (00027_idempotency_keys.sql) scopes a key by (practice_id, staff_id),
// both NOT NULL -- a Client caller has neither. Wiring this endpoint would
// need a schema change adding ClientID-scoped keys, which #129 left as a
// follow-up rather than resolving here.
func ClientCreateHandler(store objectstore.ObjectStore, pusher push.Pusher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		clientID, _ := clientauth.ClientID(r.Context())
		engagementID, _ := clientauth.EngagementID(r.Context())

		messageID := uuid.NewString()
		body, attachment, ok := decodeCreate(w, r, store, engagementID, messageID, apierr.MsgInternalError)
		if !ok {
			return
		}

		item, err := insertMessage(r.Context(), tx, messageID, engagementID, senderTypeClient, clientID, body, attachment)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		notifyRecipient(r.Context(), tx, pusher, engagementID, senderTypeClient)
		writeCreated(w, item)
	})
}
