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
