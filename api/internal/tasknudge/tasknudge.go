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

// OutboxType names which of ADR-0010's process-* endpoints a nudge task
// should call.
type OutboxType string

// The seven outbox types a nudge can target, one per process-* endpoint
// main.go mounts (ADR-0010, ADR-0013).
const (
	PortalInvite    OutboxType = "portal-invite"
	LowCredit       OutboxType = "low-credit"
	Payout          OutboxType = "payout"
	PaymentReceived OutboxType = "payment-received"
	SessionNotice   OutboxType = "session-notice"
	StaffInvite     OutboxType = "staff-invite"
	EngagementOffer OutboxType = "engagement-offer"
)

// Enqueuer enqueues a Cloud Task that nudges outboxType's process-*
// endpoint to run immediately. An error means the caller should log and
// swallow it (ADR-0013's correctness constraint) -- Cloud Scheduler's own
// cadence for that outbox is still coming regardless.
type Enqueuer interface {
	Enqueue(ctx context.Context, outboxType OutboxType) error
}

// NoOpEnqueuer is the Enqueuer main() wires up wherever no Cloud Tasks
// queue is configured (NOTIFICATION_TASKS_QUEUE unset -- local dev, CI's
// boot smoke test, and the e2e stack, none of which have GCP credentials
// available for a real *cloudtasks.Client). Every call succeeds without
// doing anything: the outbox row a write site just queued still gets
// picked up by Cloud Scheduler's cadence regardless, so there is nothing
// to log either.
type NoOpEnqueuer struct{}

// Enqueue does nothing and always succeeds.
func (NoOpEnqueuer) Enqueue(context.Context, OutboxType) error {
	return nil
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
