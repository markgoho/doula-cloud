package push

import (
	"context"
	"sync"
)

// FakeCall records one Send invocation, for tests to assert against.
type FakeCall struct {
	Subscription Subscription
	Payload      []byte
}

// FakePusher is an in-memory Pusher, injected into handler tests instead
// of a real VAPID/push service, per #56's testing decisions. Err, when
// set, is returned by every Send call (after still recording it) so
// tests can exercise a handler's push-failure handling.
type FakePusher struct {
	mu    sync.Mutex
	calls []FakeCall
	Err   error
}

// NewFakePusher returns a FakePusher with no recorded calls.
func NewFakePusher() *FakePusher {
	return &FakePusher{}
}

// Send records sub and payload, then returns Err (nil unless a test set
// it).
func (p *FakePusher) Send(_ context.Context, sub Subscription, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, FakeCall{Subscription: sub, Payload: payload})
	return p.Err
}

// Calls returns a copy of every Send call recorded so far.
func (p *FakePusher) Calls() []FakeCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]FakeCall(nil), p.calls...)
}
