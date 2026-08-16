// Package push is the seam over Web Push delivery that #61 (Push
// notifications on new message) puts in front of the Go BFF's handlers so
// a create-Message request never calls the VAPID/web-push library
// directly. VAPIDPusher is the real, Web-Push-backed implementation;
// FakePusher is the in-memory fake injected by tests, per #56's "no real
// VAPID/push service reachable from api/ tests" testing decision.
// Deliberately generic (a bare endpoint + keys + opaque payload) so a
// future non-Message notification type can reuse it without a new
// interface, mirroring internal/objectstore's shape for attachments.
package push

import "context"

// Subscription is one registered Web Push subscription's delivery
// details -- the same three fields push_subscriptions
// (00008_messaging.sql) stores per row.
type Subscription struct {
	Endpoint  string
	P256dhKey string
	AuthKey   string
}

// Pusher sends payload -- a content-free notification per #56's
// no-PHI-to-non-BAA-vendor rule -- to a single registered subscription.
type Pusher interface {
	Send(ctx context.Context, sub Subscription, payload []byte) error
}
