package tasknudge

import (
	"context"
	"fmt"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// endpointPath maps each OutboxType to the process-* endpoint main.go
// mounts it at (ADR-0010). Every nudge task hits one of these eleven paths.
var endpointPath = map[OutboxType]string{
	SiteBuild:         "/api/internal/site/process-build-outbox",
	EngagementOffer:   "/api/internal/notifications/process-offer-outbox",
	EngagementRequest: "/api/internal/notifications/process-engagement-request-outbox",
	PortalInvite:      "/api/internal/notifications/process-outbox",
	LowCredit:         "/api/internal/notifications/process-low-credit-outbox",
	Payout:            "/api/internal/notifications/process-payout-outbox",
	PaymentReceived:   "/api/internal/notifications/process-payment-outbox",
	SessionNotice:     "/api/internal/notifications/process-session-notice-outbox",
	StaffInvite:       "/api/internal/notifications/process-staff-invite-outbox",
	MFARecoveryCode:   "/api/internal/notifications/process-mfa-recovery-outbox",
	ClientErasure:     "/api/internal/clients/process-erasure-outbox",
}

// CloudTasksEnqueuer is the production Enqueuer, backed by one Cloud
// Tasks queue shared by all eight outbox types (ADR-0013) -- the same
// X-Internal-Secret shape the process-* endpoints already accept from
// Cloud Scheduler, so no endpoint needs to change to accept a nudge too.
type CloudTasksEnqueuer struct {
	client        *cloudtasks.Client
	queue         string
	targetBaseURL string
	secret        string
}

// NewCloudTasksEnqueuer wraps an existing *cloudtasks.Client -- callers
// construct the client once at startup and share it, the same pattern
// objectstore.NewGCSStore uses for *storage.Client. queue is the queue's
// full resource name (projects/PROJECT_ID/locations/LOCATION_ID/queues/QUEUE_ID).
// targetBaseURL is the Cloud Run service's own base URL (e.g.
// https://doula-api-xyz.a.run.app); secret is NOTIFICATION_WORKER_SECRET,
// reused rather than a second credential.
func NewCloudTasksEnqueuer(client *cloudtasks.Client, queue, targetBaseURL, secret string) *CloudTasksEnqueuer {
	// coverage:ignore reason: requires a real Cloud Tasks client, not exercised by unit tests
	return &CloudTasksEnqueuer{client: client, queue: queue, targetBaseURL: targetBaseURL, secret: secret}
}

// Enqueue creates a Cloud Task that POSTs outboxType's process-* endpoint
// with the same X-Internal-Secret header Cloud Scheduler sends. No task
// name is set, so Cloud Tasks assigns a random one -- de-duplication by
// name isn't wanted here: a burst of writes to the same outbox should
// nudge every time, not collapse into a single task.
func (e *CloudTasksEnqueuer) Enqueue(ctx context.Context, outboxType OutboxType) error {
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	path, ok := endpointPath[outboxType]
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	if !ok {
		// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
		return fmt.Errorf("tasknudge: unknown outbox type %q", outboxType)
	}
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	task := &cloudtaskspb.Task{
		MessageType: &cloudtaskspb.Task_HttpRequest{
			HttpRequest: &cloudtaskspb.HttpRequest{
				Url:        e.targetBaseURL + path,
				HttpMethod: cloudtaskspb.HttpMethod_POST,
				Headers:    map[string]string{"X-Internal-Secret": e.secret},
			},
		},
	}
	// Zero for every type but #443's site rebuild, whose worker can only
	// collapse queued rows that have had a moment to accumulate. Left
	// unset when the delay is zero, which is what "as soon as you can"
	// has always meant here.
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	if d := Delay(outboxType); d > 0 {
		// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
		task.ScheduleTime = timestamppb.New(time.Now().Add(d))
	}
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	_, err := e.client.CreateTask(ctx, &cloudtaskspb.CreateTaskRequest{Parent: e.queue, Task: task})
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
		return fmt.Errorf("tasknudge: enqueue nudge for %s: %w", outboxType, err)
	}
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	return nil
}
