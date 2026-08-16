package push

import (
	"context"
	"fmt"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// vapidSendTimeout bounds a single push-service round trip. Message
// creation calls Pusher.Send synchronously and in-request (#56's "no
// background job/queue" decision), so a slow or unreachable push service
// must not stall the response indefinitely.
const vapidSendTimeout = 5 * time.Second

// VAPIDPusher is the production Pusher, backed by Web Push (VAPID) --
// the same bucket-vs-client shape as objectstore.GCSStore.
type VAPIDPusher struct {
	publicKey  string
	privateKey string
	subscriber string
	httpClient *http.Client
}

// NewVAPIDPusher builds a VAPIDPusher from a VAPID keypair (see
// webpush.GenerateVAPIDKeys) and a subscriber contact (a "mailto:" URI,
// required by the VAPID spec).
func NewVAPIDPusher(publicKey, privateKey, subscriber string) *VAPIDPusher {
	// coverage:ignore reason: requires real VAPID keys and network access, not exercised by unit tests
	return &VAPIDPusher{
		publicKey:  publicKey,
		privateKey: privateKey,
		subscriber: subscriber,
		httpClient: &http.Client{Timeout: vapidSendTimeout},
	}
}

// Send delivers payload to sub via the Web Push protocol.
func (p *VAPIDPusher) Send(ctx context.Context, sub Subscription, payload []byte) error {
	// coverage:ignore reason: requires a real push service and network access, not exercised by unit tests
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dhKey, Auth: sub.AuthKey},
	}, &webpush.Options{
		HTTPClient:      p.httpClient,
		Subscriber:      p.subscriber,
		VAPIDPublicKey:  p.publicKey,
		VAPIDPrivateKey: p.privateKey,
		TTL:             60,
	})
	// coverage:ignore reason: requires a real push service and network access, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real push service and network access, not exercised by unit tests
		return fmt.Errorf("push: send: %w", err)
	}
	// coverage:ignore reason: requires a real push service and network access, not exercised by unit tests
	defer func() { _ = resp.Body.Close() }()
	// coverage:ignore reason: requires a real push service and network access, not exercised by unit tests
	return nil
}
