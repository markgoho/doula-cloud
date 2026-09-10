package tasknudge

import (
	"testing"

	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

const (
	testBaseURL     = "https://doula-api.example"
	testNudgePath   = "/api/internal/notifications/process-outbox"
	testRuntimeSA   = "doula-api-runtime@doula-cloud.iam.gserviceaccount.com"
	testNudgeTarget = testBaseURL + testNudgePath
	// #nosec G101 -- a test fixture, not a credential
	testSecret = "e2e-worker-secret"
)

func testEnqueuer(auth CallerAuth) *CloudTasksEnqueuer {
	return &CloudTasksEnqueuer{
		targetBaseURL: testBaseURL,
		auth:          auth,
		endpointPath:  map[OutboxType]string{PortalInvite: testNudgePath},
	}
}

// The production shape (ADR-0037): the task carries an OIDC token minted
// for the runtime service account, and no header at all -- nothing
// durable holds a credential a reader could replay.
func TestTask_CarriesAnOIDCTokenWhenAServiceAccountIsNamed(t *testing.T) {
	task, err := testEnqueuer(CallerAuth{
		ServiceAccount: testRuntimeSA,
		Audience:       testBaseURL,
		Secret:         "should-not-be-sent",
	}).task(PortalInvite)
	if err != nil {
		t.Fatalf("task() error = %v", err)
	}

	request := task.GetHttpRequest()
	if got := request.GetUrl(); got != testNudgeTarget {
		t.Errorf("url = %q, want %q", got, testNudgeTarget)
	}
	token := request.GetOidcToken()
	if token == nil {
		t.Fatal("task carries no OIDC token, want one")
	}
	if token.GetServiceAccountEmail() != testRuntimeSA {
		t.Errorf("service account = %q, want %q", token.GetServiceAccountEmail(), testRuntimeSA)
	}
	// The audience has to be the same value internalauth checks, not
	// Cloud Tasks' default of the full target URI -- they differ by the
	// path, and a mismatch is a 401 on the first nudge.
	if token.GetAudience() != testBaseURL {
		t.Errorf("audience = %q, want %q", token.GetAudience(), testBaseURL)
	}
	if len(request.GetHeaders()) != 0 {
		t.Errorf("headers = %v, want none alongside an OIDC token", request.GetHeaders())
	}
}

// The local and end-to-end shape: no service account to mint a token
// for, so the task falls back to the header the stack configures.
func TestTask_FallsBackToTheHeaderWithNoServiceAccount(t *testing.T) {
	task, err := testEnqueuer(CallerAuth{Secret: testSecret}).task(PortalInvite)
	if err != nil {
		t.Fatalf("task() error = %v", err)
	}

	request := task.GetHttpRequest()
	if request.GetOidcToken() != nil {
		t.Error("task carries an OIDC token with no service account named, want none")
	}
	if got := request.GetHeaders()["X-Internal-Secret"]; got != testSecret {
		t.Errorf("X-Internal-Secret = %q, want the configured secret", got)
	}
	if request.GetHttpMethod() != cloudtaskspb.HttpMethod_POST {
		t.Errorf("method = %v, want POST", request.GetHttpMethod())
	}
}

// #443's site rebuild is the one type whose nudge waits, so its worker
// has rows to collapse; every other type is dispatched as soon as Cloud
// Tasks can.
func TestTask_DelaysOnlyTheSiteRebuild(t *testing.T) {
	enqueuer := &CloudTasksEnqueuer{
		targetBaseURL: testBaseURL,
		auth:          CallerAuth{Secret: testSecret},
		endpointPath: map[OutboxType]string{
			PortalInvite: testNudgePath,
			SiteBuild:    "/api/internal/site/process-build-outbox",
		},
	}

	prompt, err := enqueuer.task(PortalInvite)
	if err != nil {
		t.Fatalf("task(PortalInvite) error = %v", err)
	}
	if prompt.GetScheduleTime() != nil {
		t.Error("PortalInvite nudge carries a schedule time, want none")
	}

	delayed, err := enqueuer.task(SiteBuild)
	if err != nil {
		t.Fatalf("task(SiteBuild) error = %v", err)
	}
	if delayed.GetScheduleTime() == nil {
		t.Error("SiteBuild nudge carries no schedule time, want one")
	}
}

// A type with no registered path is a programming error, and it is
// reported rather than POSTed at an address the mux does not serve.
// Enqueue reports it without reaching for the queue at all, which is
// what lets this run with no client: an unbuildable task is not a task
// worth trying to create.
func TestEnqueue_RefusesAnUnregisteredOutboxType(t *testing.T) {
	if err := testEnqueuer(CallerAuth{}).Enqueue(t.Context(), SiteBuild); err == nil {
		t.Error("Enqueue() error = nil for an unregistered type, want an error")
	}
}
