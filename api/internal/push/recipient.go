package push

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"log"
)

// NotifyMessageRecipient sends payload to every push subscription the
// recipientType party ("staff" or "client") holds for engagementID,
// synchronously and in-request. Every failure is logged under label and
// swallowed: a push is a best-effort layer on top of a write the caller
// has already made to tx, never part of that write's success.
//
// The subscriptions come from the SECURITY DEFINER function added by
// 00010_push_subscriptions_message_recipient.sql, which -- unlike every
// other query on push_subscriptions -- deliberately bypasses the table's
// own RLS (00008_messaging.sql scoped it to self-identity only, per #57's
// AC) so the other party's subscriptions can be resolved without widening
// that surface. Safe only because the caller's middleware has already
// established that it owns engagementID.
func NotifyMessageRecipient(ctx context.Context, tx *sql.Tx, pusher Pusher, label, engagementID, recipientType string, payload any) {
	body, err := json.Marshal(payload)
	// coverage:ignore reason: the payloads callers pass always marshal cleanly, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: the payloads callers pass always marshal cleanly, not exercised by unit tests
		log.Printf("%s: marshal payload: %v", label, err)
		// coverage:ignore reason: the payloads callers pass always marshal cleanly, not exercised by unit tests
		return
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT endpoint, p256dh_key, auth_key FROM push_subscriptions_for_message_recipient($1, $2)`,
		engagementID, recipientType,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		log.Printf("%s: query subscriptions: %v", label, err)
		return
	}
	defer func() { _ = rows.Close() }()

	var subs []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dhKey, &s.AuthKey); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			log.Printf("%s: scan subscription row: %v", label, err)
			return
		}
		subs = append(subs, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		log.Printf("%s: iterate subscription rows: %v", label, err)
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return
	}

	for _, sub := range subs {
		if err := pusher.Send(ctx, sub, body); err != nil {
			log.Printf("%s: send push: %v", label, err)
		}
	}
}
