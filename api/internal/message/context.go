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
// no update or delete endpoint -- Messages are immutable. On successful
// creation, the recipient's push subscription(s) are notified via a
// push.Pusher (push.go, #61).
package message

import ()

// senderTypeStaff and senderTypeClient are the two values messages.sender_type
// (the actor_type Postgres enum from 00008_messaging.sql) can hold. Shared
// across create.go and list.go so the two populations' names live in one
// place rather than as repeated string literals.
const (
	senderTypeStaff  = "staff"
	senderTypeClient = "client"
)
