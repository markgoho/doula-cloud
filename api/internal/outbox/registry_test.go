package outbox_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// portalInvitePath is a stand-in address, not the real one -- these
// tests are about the registry's own behavior, and the endpoints the BFF
// actually serves are pinned in api/outboxes_test.go.
const portalInvitePath = "/portal-invite"

// recordingMux stands in for *http.ServeMux so a test can see exactly
// which patterns Register mounted, without going through routing.
type recordingMux struct {
	patterns []string
}

func (m *recordingMux) Handle(pattern string, _ http.Handler) {
	m.patterns = append(m.patterns, pattern)
}

func TestRegister_MountsEveryRegistrationAsAPOST(t *testing.T) {
	db := testdb.New(t)
	mux := &recordingMux{}

	paths := outbox.Register(mux, db.App, "secret", []outbox.Registration{
		{Path: "/api/internal/notifications/process-outbox", Door: outbox.NotificationDoor, Worker: &stubProcessor{}},
		{Path: "/api/internal/site/process-build-outbox", Worker: &stubProcessor{}},
	})

	wantPatterns := []string{
		"POST /api/internal/notifications/process-outbox",
		"POST /api/internal/site/process-build-outbox",
	}
	if !reflect.DeepEqual(mux.patterns, wantPatterns) {
		t.Errorf("mounted patterns = %v, want %v", mux.patterns, wantPatterns)
	}

	// Sorted, so a caller asserting the contract Cloud Scheduler is
	// provisioned against does not have to care what order the list is in.
	wantPaths := []string{
		"/api/internal/notifications/process-outbox",
		"/api/internal/site/process-build-outbox",
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("returned paths = %v, want %v", paths, wantPaths)
	}
}

// TestRegister_MountsHandlersThatRunTheirOwnWorker proves Register wires
// each registration to its own worker rather than to a shared one -- the
// bug a list of near-identical entries invites.
func TestRegister_MountsHandlersThatRunTheirOwnWorker(t *testing.T) {
	db := testdb.New(t)
	first, second := &stubProcessor{}, &stubProcessor{}

	mux := http.NewServeMux()
	outbox.Register(mux, db.App, "correct-secret", []outbox.Registration{
		{Path: "/first", Door: outbox.NotificationDoor, Worker: first},
		{Path: "/second", Door: outbox.NotificationDoor, Worker: second},
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := postTo(t, srv, "/second", "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if first.called {
		t.Error("posting /second ran /first's worker")
	}
	if !second.called {
		t.Error("posting /second did not run its own worker")
	}
}

// TestRegister_CarriesTheSecretToEveryHandler proves the secret check is
// not lost on the way through the registry.
func TestRegister_CarriesTheSecretToEveryHandler(t *testing.T) {
	db := testdb.New(t)
	worker := &stubProcessor{}

	mux := http.NewServeMux()
	outbox.Register(mux, db.App, "correct-secret", []outbox.Registration{
		{Path: "/only", Door: outbox.NotificationDoor, Worker: worker},
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp := postTo(t, srv, "/only", "wrong-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if worker.called {
		t.Error("worker ran despite the wrong secret")
	}
}

// TestProcessHandler_NoDoorRunsWithoutSettingOne is #443's site rebuild
// shape: an outbox whose table is not under RLS is handed no door, and
// still processes normally.
func TestProcessHandler_NoDoorRunsWithoutSettingOne(t *testing.T) {
	db := testdb.New(t)
	worker := &doorReadingProcessor{}
	srv := newHandlerServerWithDoor(db, worker, "correct-secret", "")
	defer srv.Close()

	resp := postProcess(t, srv, "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if worker.door != "" {
		t.Errorf("app.notification_worker_trusted = %q, want it unset", worker.door)
	}
}

// TestProcessHandler_DoorIsOpenForTheWorker is the same read on the other
// side: a registration naming NotificationDoor gets it set for the length
// of its transaction, which is what licenses each outbox's own recipient
// SELECT policies.
func TestProcessHandler_DoorIsOpenForTheWorker(t *testing.T) {
	db := testdb.New(t)
	worker := &doorReadingProcessor{}
	srv := newHandlerServerWithDoor(db, worker, "correct-secret", outbox.NotificationDoor)
	defer srv.Close()

	resp := postProcess(t, srv, "correct-secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if worker.door != "true" {
		t.Errorf("app.notification_worker_trusted = %q, want %q", worker.door, "true")
	}
}

func TestNudgePaths_MapsEveryNudgedRegistration(t *testing.T) {
	got := outbox.NudgePaths([]outbox.Registration{
		{Path: portalInvitePath, Nudge: tasknudge.PortalInvite, Worker: &stubProcessor{}},
		{Path: "/site-build", Nudge: tasknudge.SiteBuild, Worker: &stubProcessor{}},
	})

	want := map[tasknudge.OutboxType]string{
		tasknudge.PortalInvite: portalInvitePath,
		tasknudge.SiteBuild:    "/site-build",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NudgePaths = %v, want %v", got, want)
	}
}

// TestNudgePaths_OmitsARegistrationWithNoNudge is #613's two Staff auth
// mail outboxes: they ride Cloud Scheduler's cadence alone, so they must
// be absent from the map rather than present under an empty key -- which
// would make every un-nudged outbox collide on one entry.
func TestNudgePaths_OmitsARegistrationWithNoNudge(t *testing.T) {
	got := outbox.NudgePaths([]outbox.Registration{
		{Path: "/staff-token-mail", Worker: &stubProcessor{}},
		{Path: "/staff-email-change", Worker: &stubProcessor{}},
		{Path: portalInvitePath, Nudge: tasknudge.PortalInvite, Worker: &stubProcessor{}},
	})

	want := map[tasknudge.OutboxType]string{tasknudge.PortalInvite: portalInvitePath}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NudgePaths = %v, want %v", got, want)
	}
}

// doorReadingProcessor reads back the session variable the handler is
// supposed to have set, from inside the very transaction the worker runs
// in -- the only place the answer is observable.
type doorReadingProcessor struct {
	door string
}

func (p *doorReadingProcessor) ProcessPending(ctx context.Context, tx *sql.Tx) error {
	// COALESCE because current_setting's missing_ok form answers NULL for
	// a variable that was never set, which is exactly the no-door case.
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(current_setting('app.notification_worker_trusted', true), '')`,
	).Scan(&p.door); err != nil {
		return fmt.Errorf("read the door: %w", err)
	}
	return nil
}

func postTo(t *testing.T, srv *httptest.Server, path, headerSecret string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", headerSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}
