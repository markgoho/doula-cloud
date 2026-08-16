package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"doula-cloud/api/internal/push"
)

// pushPayload is the content-free payload delivered to a Message
// recipient's push subscription(s) -- per #56's "no sender name, no
// message text" rule, it carries only enough to route a notification tap
// to the right thread. PracticeID is empty for a Client recipient (the
// Client-portal thread route needs only an Engagement id); it's set for a
// Staff recipient, whose thread route is Practice-scoped.
type pushPayload struct {
	EngagementID string `json:"engagementId"`
	PracticeID   string `json:"practiceId,omitempty"`
}

// notifyRecipient resolves the *other* party's (the recipient's)
// registered push subscription(s) for engagementID and sends each one a
// content-free notification, synchronously and in-request -- #56's "no
// background job/queue" decision. senderType is the population that
// authored the Message (senderTypeStaff or senderTypeClient, as passed to
// insertMessage) -- the recipient is always the other one; taking the
// sender's type here, rather than the recipient's, keeps call sites
// self-explanatory (CreateHandler passes senderTypeStaff because a Staff
// member is sending, not because it's notifying Staff). Any failure along
// the way (resolving the Practice id, marshaling the payload, resolving
// subscriptions, delivering a push) is logged and swallowed rather than
// surfaced as a 500: the Message row has already been written to tx by
// the time this runs (durability itself still depends on the
// staffauth/clientauth Middleware's deferred commit succeeding, same as
// every other write in this package), and push delivery is a best-effort
// notification layer on top of it, not part of create's own success
// criteria.
func notifyRecipient(ctx context.Context, tx *sql.Tx, pusher push.Pusher, engagementID, senderType string) {
	recipientType := senderTypeStaff
	if senderType == senderTypeStaff {
		recipientType = senderTypeClient
	}

	payload := pushPayload{EngagementID: engagementID}
	if recipientType == senderTypeStaff {
		practiceID, err := fetchEngagementPracticeID(ctx, tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			log.Printf("message: notify recipient: resolve practice id: %v", err)
			return
		}
		payload.PracticeID = practiceID
	}

	body, err := json.Marshal(payload)
	// coverage:ignore reason: pushPayload always marshals cleanly, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: pushPayload always marshals cleanly, not exercised by unit tests
		log.Printf("message: notify recipient: marshal payload: %v", err)
		// coverage:ignore reason: pushPayload always marshals cleanly, not exercised by unit tests
		return
	}

	subs, err := resolveRecipientSubscriptions(ctx, tx, engagementID, recipientType)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		log.Printf("message: notify recipient: resolve subscriptions: %v", err)
		return
	}

	for _, sub := range subs {
		if err := pusher.Send(ctx, sub, body); err != nil {
			log.Printf("message: notify recipient: send push: %v", err)
		}
	}
}

// fetchEngagementPracticeID looks up engagementID's Practice, for a Staff
// recipient's payload. Callable from a Client-scoped tx (app.current_client_id
// set, app.current_practice_id unset) because the caller already owns
// this Engagement -- clientauth.Middleware verified it before
// ClientCreateHandler ever runs -- so engagements' client-tier RLS policy
// (00005_client_engagement.sql) grants visibility without needing
// app.current_practice_id to be set.
func fetchEngagementPracticeID(ctx context.Context, tx *sql.Tx, engagementID string) (string, error) {
	var practiceID string
	err := tx.QueryRowContext(ctx, `SELECT practice_id FROM engagements WHERE id = $1`, engagementID).Scan(&practiceID)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("message: fetch engagement practice id: %w", err)
	}
	return practiceID, nil
}

// resolveRecipientSubscriptions calls the SECURITY DEFINER function added
// by 00010_push_subscriptions_message_recipient.sql, which -- unlike
// every other query in this package -- deliberately bypasses
// push_subscriptions' own RLS (00008_messaging.sql scoped it to
// self-identity only, never cross-identity readable, per #57's AC) so a
// Message's *other* party's subscriptions can be resolved without
// widening that table's RLS surface. Safe only because engagementID's
// ownership by the caller was already established by staffauth's or
// clientauth's middleware before this runs.
func resolveRecipientSubscriptions(ctx context.Context, tx *sql.Tx, engagementID, recipientType string) ([]push.Subscription, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT endpoint, p256dh_key, auth_key FROM push_subscriptions_for_message_recipient($1, $2)`,
		engagementID, recipientType)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, fmt.Errorf("message: query recipient subscriptions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var subs []push.Subscription
	for rows.Next() {
		var s push.Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dhKey, &s.AuthKey); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("message: scan recipient subscription row: %w", err)
		}
		subs = append(subs, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("message: iterate recipient subscription rows: %w", err)
	}
	return subs, nil
}
