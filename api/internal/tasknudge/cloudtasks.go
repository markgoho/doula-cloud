package tasknudge

import (
	"context"
	"fmt"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CallerAuth is how a nudge task identifies itself to the process-*
// endpoint it POSTs -- the same boundary Cloud Scheduler crosses, so no
// endpoint needs to change to accept a nudge too (ADR-0037).
//
// ServiceAccount set is the production shape: Cloud Tasks mints a
// Google-signed OIDC token for that account, for Audience, and the
// endpoint checks it against internalauth's allowlist. Nothing durable
// holds a credential anybody can replay. Secret is the fallback the
// end-to-end stack and a local run use, where there is no metadata
// server to mint a token with.
type CallerAuth struct {
	ServiceAccount string
	Audience       string
	Secret         string
}

// CloudTasksEnqueuer is the production Enqueuer, backed by one Cloud
// Tasks queue shared by every nudged outbox type (ADR-0013).
type CloudTasksEnqueuer struct {
	client        *cloudtasks.Client
	queue         string
	targetBaseURL string
	auth          CallerAuth
	endpointPath  map[OutboxType]string
}

// NewCloudTasksEnqueuer wraps an existing *cloudtasks.Client -- callers
// construct the client once at startup and share it, the same pattern
// objectstore.NewGCSStore uses for *storage.Client. queue is the queue's
// full resource name (projects/PROJECT_ID/locations/LOCATION_ID/queues/QUEUE_ID).
// targetBaseURL is the Cloud Run service's own base URL (e.g.
// https://doula-api-xyz.a.run.app); auth is how each task identifies
// itself to the endpoint it calls.
//
// endpointPath is where each nudged OutboxType is served, supplied by the
// caller rather than held here. This package used to keep its own copy of
// every path, which made a route pattern a fact written in two files that
// agreed only by hand -- so a renamed endpoint could leave a nudge
// POSTing an address the mux no longer served. The BFF's outbox list is
// now the single source for both (outbox.NudgePaths).
func NewCloudTasksEnqueuer(client *cloudtasks.Client, queue, targetBaseURL string, auth CallerAuth, endpointPath map[OutboxType]string) *CloudTasksEnqueuer {
	// coverage:ignore reason: requires a real Cloud Tasks client, not exercised by unit tests
	return &CloudTasksEnqueuer{client: client, queue: queue, targetBaseURL: targetBaseURL, auth: auth, endpointPath: endpointPath}
}

// task builds the Cloud Task that nudges outboxType's process-*
// endpoint. Separated from Enqueue because everything it decides -- the
// URL, how the task identifies itself, and #443's delay -- is worth
// asserting on, and none of it needs a queue to assert.
//
// No task name is set, so Cloud Tasks assigns a random one:
// de-duplication by name isn't wanted here, since a burst of writes to
// the same outbox should nudge every time rather than collapse into a
// single task.
func (e *CloudTasksEnqueuer) task(outboxType OutboxType) (*cloudtaskspb.Task, error) {
	path, ok := e.endpointPath[outboxType]
	if !ok {
		return nil, fmt.Errorf("tasknudge: unknown outbox type %q", outboxType)
	}

	request := &cloudtaskspb.HttpRequest{
		Url:        e.targetBaseURL + path,
		HttpMethod: cloudtaskspb.HttpMethod_POST,
	}
	// The production shape: Cloud Tasks mints the token itself at
	// dispatch, so nothing durable carries a credential. The header is
	// what a stack with no metadata server falls back to.
	if e.auth.ServiceAccount != "" {
		request.AuthorizationHeader = &cloudtaskspb.HttpRequest_OidcToken{
			OidcToken: &cloudtaskspb.OidcToken{
				ServiceAccountEmail: e.auth.ServiceAccount,
				Audience:            e.auth.Audience,
			},
		}
	} else {
		request.Headers = map[string]string{"X-Internal-Secret": e.auth.Secret}
	}

	task := &cloudtaskspb.Task{MessageType: &cloudtaskspb.Task_HttpRequest{HttpRequest: request}}
	// Zero for every type but #443's site rebuild, whose worker can only
	// collapse queued rows that have had a moment to accumulate. Left
	// unset when the delay is zero, which is what "as soon as you can"
	// has always meant here.
	if d := Delay(outboxType); d > 0 {
		task.ScheduleTime = timestamppb.New(time.Now().Add(d))
	}
	return task, nil
}

// Enqueue creates the task built above on the shared queue.
func (e *CloudTasksEnqueuer) Enqueue(ctx context.Context, outboxType OutboxType) error {
	task, err := e.task(outboxType)
	if err != nil {
		return err
	}
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	_, err = e.client.CreateTask(ctx, &cloudtaskspb.CreateTaskRequest{Parent: e.queue, Task: task})
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
		return fmt.Errorf("tasknudge: enqueue nudge for %s: %w", outboxType, err)
	}
	// coverage:ignore reason: requires a real Cloud Tasks queue and network access, not exercised by unit tests
	return nil
}
