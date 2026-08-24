// Package tasknudge is ADR-0013's latency nudge on top of ADR-0010's
// outbox: after a write site queues an outbox row, it enqueues a Cloud
// Task that calls the same process-* endpoint Cloud Scheduler already
// polls, normally delivered within seconds instead of on the next
// five-minute tick. Cloud Scheduler's cadence is untouched and stays the
// durability backstop.
package tasknudge

import (
	"context"
	"log"
)

// OutboxType names which of ADR-0010's five process-* endpoints a nudge
// task should call.
type OutboxType string

// The five outbox types a nudge can target, one per process-* endpoint
// main.go mounts (ADR-0010, ADR-0013).
const (
	PortalInvite    OutboxType = "portal-invite"
	LowCredit       OutboxType = "low-credit"
	Payout          OutboxType = "payout"
	PaymentReceived OutboxType = "payment-received"
	SessionNotice   OutboxType = "session-notice"
)

// Enqueuer enqueues a Cloud Task that nudges outboxType's process-*
// endpoint to run immediately. An error means the caller should log and
// swallow it (ADR-0013's correctness constraint) -- Cloud Scheduler's own
// cadence for that outbox is still coming regardless.
type Enqueuer interface {
	Enqueue(ctx context.Context, outboxType OutboxType) error
}

// Fire returns a closure that enqueues a nudge for outboxType via enq,
// logging and swallowing any error rather than propagating it -- the one
// piece of behavior every write site in ADR-0013 needs identically. The
// three write sites that commit their own write directly call the
// returned closure immediately after that commit succeeds; the two that
// run inside staffauth.Middleware's request-scoped transaction pass it to
// Register instead, so Middleware runs it only after its own commit
// succeeds.
func Fire(enq Enqueuer, outboxType OutboxType) func(context.Context) {
	return func(ctx context.Context) {
		if err := enq.Enqueue(ctx, outboxType); err != nil {
			log.Printf("tasknudge: enqueue nudge for %s outbox: %v", outboxType, err)
		}
	}
}
