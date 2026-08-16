// Package message holds the BFF handlers for a Message thread: list and
// create, for both the Staff-side (#58) and Client-portal-side (#59)
// populations. The Staff handlers rely on staffauth.Middleware having
// already resolved the caller's Staff/Practice ids and opened a
// request-scoped *sql.Tx with app.current_practice_id set, the same way
// the engagement and visit packages' handlers do; the Client-portal
// handlers (client.go) rely on clientauth.Middleware the same way, with
// app.current_client_id set instead. Both populations share the same
// thread per Engagement -- one continuous conversation, not split by
// sender. A Message may carry a single image/PDF attachment (attachment.go,
// #60), stored via an objectstore.ObjectStore rather than inline; there is
// no update or delete endpoint -- Messages are immutable (push
// notifications are a separate ticket, per #58).
package message

import (
	"context"
	"database/sql"
	"fmt"
)

// senderTypeStaff and senderTypeClient are the two values messages.sender_type
// (the actor_type Postgres enum from 00008_messaging.sql) can hold. Shared
// across create.go and list.go so the two populations' names live in one
// place rather than as repeated string literals.
const (
	senderTypeStaff  = "staff"
	senderTypeClient = "client"
)

// requireEngagementAtPractice confirms engagementID exists and belongs to
// practiceID, returning sql.ErrNoRows if not -- callers translate that into
// a 404, mirroring visit.requireEngagementAtPractice. Shared by
// ListHandler and CreateHandler so a thread can never be listed or posted
// to under an Engagement at a different Practice, even one an attacker
// guesses the id of.
func requireEngagementAtPractice(ctx context.Context, tx *sql.Tx, engagementID, practiceID string) error {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE id = $1 AND practice_id = $2)`,
		engagementID, practiceID,
	).Scan(&exists)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("message: check engagement at practice: %w", err)
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}
