package mail

import (
	"context"
	"sync"
)

// FakeSender is the in-memory Sender tests inject instead of the real
// Mailgun-backed one -- see docs/testing.md's "no real vendor reachable
// from api/ tests" rule, mirroring push.FakePusher.
type FakeSender struct {
	mu   sync.Mutex
	sent []Message
	// Err, when set, is returned by every call to Send instead of
	// recording the message -- how outbox retry/dead-letter tests
	// simulate a Mailgun failure.
	Err error
	// deleted records every address handed to DeleteBounce, and
	// DeleteErr, when set, makes DeleteBounce fail instead -- how #744's
	// "Mailgun refused, so change nothing locally" case is exercised.
	deleted   []string
	DeleteErr error
}

// Send records msg, or returns Err if set.
func (f *FakeSender) Send(_ context.Context, msg Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.sent = append(f.sent, msg)
	return nil
}

// Sent returns every Message recorded by Send so far.
func (f *FakeSender) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.sent...)
}

// DeleteBounce records address instead of calling Mailgun, or returns
// DeleteErr if set -- the same seam Send is, for #744's suppression
// clear.
func (f *FakeSender) DeleteBounce(_ context.Context, address string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	f.deleted = append(f.deleted, address)
	return nil
}

// Deleted returns every address DeleteBounce was called with so far.
func (f *FakeSender) Deleted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}
