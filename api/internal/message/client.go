package message

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/clientauth"
)

// ClientListHandler mirrors ListHandler for the Client-portal population:
// same query, same pagination and ordering, but scoped by
// clientauth.Middleware's already-authorized Engagement rather than a
// Staff practice-tier check -- clientauth.Middleware has already 403'd
// any Engagement the caller doesn't own, so no extra ownership check is
// needed here. Must be mounted behind clientauth.Middleware.
func ClientListHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		var after *messageCursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := decodeCursor(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		items, hasMore, err := listMessages(r.Context(), tx, engagementID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := ListResponse{Items: items, HasMore: hasMore}
		if hasMore {
			next := encodeCursor(items[len(items)-1].CreatedAt, items[len(items)-1].MessageID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// ClientCreateHandler mirrors CreateHandler for the Client-portal
// population: posts a text-only Message as the calling Client into the
// caller's own Engagement thread. Must be mounted behind
// clientauth.Middleware.
func ClientCreateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		clientID, _ := clientauth.ClientID(r.Context())
		engagementID, _ := clientauth.EngagementID(r.Context())

		req, ok := decodeCreateRequest(w, r)
		if !ok {
			return
		}

		messageID := uuid.NewString()
		item, err := insertMessage(r.Context(), tx, messageID, engagementID, senderTypeClient, clientID, req.Body)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeCreated(w, item, clientauth.MsgInternalError)
	})
}
