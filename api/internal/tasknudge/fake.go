package tasknudge

import (
	"context"
	"sync"
)

// FakeEnqueuer is the in-memory Enqueuer tests inject instead of the real
// Cloud-Tasks-backed one, mirroring mail.FakeSender/push.FakePusher.
type FakeEnqueuer struct {
	mu    sync.Mutex
	calls []OutboxType
	// Err, when set, is returned by every call to Enqueue instead of
	// recording the call -- how a write site's test simulates a Cloud
	// Tasks enqueue failure to prove ADR-0013's correctness constraint.
	Err error
}

// Enqueue records outboxType, or returns Err if set.
func (f *FakeEnqueuer) Enqueue(_ context.Context, outboxType OutboxType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.calls = append(f.calls, outboxType)
	return nil
}

// Calls returns every OutboxType recorded by Enqueue so far.
func (f *FakeEnqueuer) Calls() []OutboxType {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]OutboxType(nil), f.calls...)
}
