package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

const statusSent = "sent"

// sendPushPayload is the content-free payload delivered to the Client's
// push subscription(s) on Send -- per the ticket's "no Contract content
// leaking through the notification channel" rule, it carries only the
// Engagement id, the same shape message.pushPayload uses for a Client
// recipient (no practiceId, since the Client-portal thread route needs
// only the Engagement id).
type sendPushPayload struct {
	EngagementID string `json:"engagementId"`
}

// PostSendContractHandler transitions the Contract for :engagementId from
// 'draft' to 'sent' -- the only transition it permits; any other current
// status 409s. On success it notifies the Client's registered push
// subscription(s) with sendPushPayload via pusher, the #61 Pusher
// interface. Must be mounted behind staffauth.Middleware.
func PostSendContractHandler(pusher push.Pusher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, values, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if status != statusDraft {
			http.Error(w, "contract is no longer a draft", http.StatusConflict)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
			statusSent, id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		notifyClient(r.Context(), tx, pusher, engagementID)

		w.Header().Set("Content-Type", "application/json")
		out := ContractResponse{
			EngagementID: engagementID,
			Status:       statusSent,
			Prose:        prose,
			MergeFields:  extractMergeFields(prose),
			Values:       values.nonEmpty(),
		}
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// notifyClient sends sendPushPayload to every push subscription
// registered by engagementID's Client, resolved via
// push_subscriptions_for_message_recipient (00010_push_subscriptions_message_recipient.sql)
// the same way message.notifyRecipient does -- Send's recipient is always
// the Client (the Staff member sending it already knows), so this needs
// neither a senderType parameter nor the Practice-id branch
// message.pushPayload carries for a Staff recipient. Failures are logged
// and swallowed: the Contract's status has already been written to tx by
// the time this runs, and push delivery is a best-effort notification
// layer on top of it, not part of Send's own success criteria.
func notifyClient(ctx context.Context, tx *sql.Tx, pusher push.Pusher, engagementID string) {
	body, err := json.Marshal(sendPushPayload{EngagementID: engagementID})
	// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
		log.Printf("contracts: notify client: marshal payload: %v", err)
		// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
		return
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT endpoint, p256dh_key, auth_key FROM push_subscriptions_for_message_recipient($1, 'client')`,
		engagementID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		log.Printf("contracts: notify client: query subscriptions: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var subs []push.Subscription
	for rows.Next() {
		var s push.Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dhKey, &s.AuthKey); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			log.Printf("contracts: notify client: scan subscription row: %v", err)
			return
		}
		subs = append(subs, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		log.Printf("contracts: notify client: iterate subscription rows: %v", err)
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return
	}

	for _, sub := range subs {
		if err := pusher.Send(ctx, sub, body); err != nil {
			log.Printf("contracts: notify client: send push: %v", err)
		}
	}
}
