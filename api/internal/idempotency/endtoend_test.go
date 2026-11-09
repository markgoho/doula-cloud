package idempotency_test

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// endToEnd is a Practice with one Doula signed in and a mutating handler
// mounted behind staffauth.Middleware(...)(idempotency.Wrap(...)) exactly
// as main.go does, so a test proves the helper is usable against a real
// endpoint and not only the synthetic handler in wrap_test.go.
type endToEnd struct {
	DB           *testdb.DB
	PracticeID   string
	EngagementID string
	ClientID     string
	Session      string
	base         string
}

// newEndToEnd mounts handler at pattern, which may name {practiceId} and
// {engagementId}. withEngagement seeds a Client and an Engagement for the
// Practice.
func newEndToEnd(t *testing.T, uid, pattern string, handler http.Handler, withEngagement bool) *endToEnd {
	t.Helper()
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	e := &endToEnd{DB: db, PracticeID: practiceID, Session: authntest.SeedSession(t, db.App, uid)}
	if withEngagement {
		e.ClientID, e.EngagementID = testdb.SeedNamedEngagement(t, db, practiceID, "E2E Client", uid+"@example.com")
	}
	mux := http.NewServeMux()
	mux.Handle("POST "+pattern, staffauth.Middleware(db.App)(idempotency.Wrap(handler)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	e.base = srv.URL + "/practices/" + practiceID
	return e
}

// post answers 201 or fails, so a test reads only what the response held.
func post[T any](t *testing.T, e *endToEnd, path, key string, opts ...reqtest.Option) T {
	t.Helper()
	return postExpecting[T](t, e, path, key, http.StatusCreated, opts...)
}

func postExpecting[T any](t *testing.T, e *endToEnd, path, key string, want int, opts ...reqtest.Option) T {
	t.Helper()
	opts = append(opts, reqtest.Session(e.Session), reqtest.HeaderIf(key != "", "Idempotency-Key", key))
	return reqtest.Expect[T](t, http.MethodPost, e.base+path, want, opts...)
}

// requireRetryReplays calls post with a key, again with the same key and
// once with none. The retry must answer what the first call did and run
// nothing a second time; the call with no key must run the handler again,
// which is what tells a real replay from a handler that happens to run
// once. Each counter reads one side effect: 1 after the first call, 1
// after the retry, 2 after the call with no key. It returns the first
// response for any further assertion on it.
func requireRetryReplays[T any](t *testing.T, post func(key string) T, id func(T) string, counters ...func() int) T {
	t.Helper()
	requireCounts := func(stage string, want int) {
		t.Helper()
		for _, count := range counters {
			if got := count(); got != want {
				t.Fatalf("side effects %s = %d, want %d", stage, got, want)
			}
		}
	}

	first := post("retry-key")
	requireCounts("after the first call", 1)
	retried := post("retry-key")
	if id(retried) != id(first) {
		t.Fatalf("retried response = %+v, want identical stored response %+v -- business logic must not re-run on replay", retried, first)
	}
	requireCounts("after the retry", 1)
	noKey := post("")
	if id(noKey) == id(first) {
		t.Fatalf("call with no Idempotency-Key returned the same response as the earlier call -- expected the handler to re-run")
	}
	requireCounts("after the call with no key", 2)
	return first
}

var errCloudTasksDown = errors.New("cloud tasks unavailable")

// postInvite posts the Engagement's portal-invite. A call with no key
// re-runs the handler against the invite that call already made, which
// answers 200 and rotates the token, where the first call answers 201.
func postInvite(t *testing.T, e *endToEnd) func(key string) portalinvite.InviteResponse {
	return func(key string) portalinvite.InviteResponse {
		want := http.StatusCreated
		if key == "" {
			want = http.StatusOK
		}
		return postExpecting[portalinvite.InviteResponse](t, e, "/engagements/"+e.EngagementID+"/portal-invite", key, want)
	}
}

func tokenOf(r portalinvite.InviteResponse) string { return r.InviteToken }

// TestEndToEnd_PortalInviteReplaysOnRetry proves a retried portal-invite
// with the same Idempotency-Key returns the stored token (#126's AC),
// and that a call with no key rotates it.
func TestEndToEnd_PortalInviteReplaysOnRetry(t *testing.T) {
	e := newEndToEnd(t, "e2e-portal-invite-staff", "/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		portalinvite.InviteHandler(&tasknudge.FakeEnqueuer{}), true)
	requireRetryReplays(t, postInvite(t, e), tokenOf)
}

// TestEndToEnd_PortalInviteReplaysOnRetryEvenWhenNudgeEnqueueFails is
// ADR-0013's correctness constraint proven at InviteHandler: a Cloud
// Tasks enqueue failure must never surface as a 500, because
// idempotency.Wrap only skips caching a response below status 500 --
// staffauth.Middleware drains the registered nudge (tasknudge.Register/
// Drain) only after its own tx.Commit() succeeds, by which point
// idempotency.Wrap has already cached the 201 this test asserts survives
// unchanged.
func TestEndToEnd_PortalInviteReplaysOnRetryEvenWhenNudgeEnqueueFails(t *testing.T) {
	e := newEndToEnd(t, "e2e-portal-invite-nudge-fail-staff", "/practices/{practiceId}/engagements/{engagementId}/portal-invite",
		portalinvite.InviteHandler(&tasknudge.FakeEnqueuer{Err: errCloudTasksDown}), true)
	requireRetryReplays(t, postInvite(t, e), tokenOf)
}

// TestEndToEnd_CreateClientNoDuplicateRowOnRetry wires client.CreateHandler
// behind the same composition, proving a retried create-Client request
// with the same Idempotency-Key returns the identical Client rather than
// a second row. #397 split the old combined create-Client-and-Engagement
// handler (#128) -- saving a Client is free now, so there is no credit to
// double-spend here; that risk moves to the Engagement Request approval
// endpoint.
func TestEndToEnd_CreateClientNoDuplicateRowOnRetry(t *testing.T) {
	e := newEndToEnd(t, "e2e-create-client-staff", "/practices/{practiceId}/clients", client.CreateHandler(), false)
	requireRetryReplays(t, func(key string) client.Record {
		// Override: true -- this test is about idempotency replay, not the
		// match-refusal path (covered in the client package's own tests), and
		// without it the no-key retry would collide with the first call's
		// identical GivenName and 409 instead of creating its own row.
		return post[client.Record](t, e, "/clients", key, reqtest.JSON(client.CreateRequest{GivenName: "E2E Client", Override: true}))
	}, func(r client.Record) string { return r.ID },
		func() int {
			return testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM clients WHERE practice_id = $1`, e.PracticeID)
		})
}

// TestEndToEnd_MessageCreateNoDuplicateRowOnRetry wires message.CreateHandler
// behind the same composition, proving a retried create-Message request with
// the same Idempotency-Key returns the identical stored Message, inserts
// exactly one messages row and notifies the Client once -- the concrete
// duplicate-post risk #129 exists to close.
func TestEndToEnd_MessageCreateNoDuplicateRowOnRetry(t *testing.T) {
	pusher := push.NewFakePusher()
	e := newEndToEnd(t, "e2e-message-create-staff", "/practices/{practiceId}/engagements/{engagementId}/messages",
		message.CreateHandler(objectstore.NewMemoryStore(), pusher), true)
	testdb.SeedPushSubscription(t, e.DB, "client", e.ClientID, "https://push.example.com/message-create-retry")

	first := requireRetryReplays(t, func(key string) message.Message {
		return post[message.Message](t, e, "/engagements/"+e.EngagementID+"/messages", key,
			reqtest.JSON(message.CreateRequest{Body: "hello from e2e"}))
	}, func(m message.Message) string { return m.MessageID },
		func() int {
			return testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM messages WHERE engagement_id = $1`, e.EngagementID)
		},
		func() int { return len(pusher.Calls()) })
	if first.SenderID == e.ClientID {
		t.Fatalf("first.SenderID = %q, want the calling Staff's id, not the seeded Client's -- staff-side create must not stamp the Client identity", first.SenderID)
	}
}

// TestEndToEnd_MessageCreateMultipartNoDuplicateUploadOnRetry covers the
// multipart/form-data path (an attachment upload alongside the row insert)
// that makes message.CreateHandler different from #128's /clients and
// #126's portal-invite: a retried multipart create with the same
// Idempotency-Key must replay the stored response without a second
// ObjectStore.Put, not just without a second messages row.
func TestEndToEnd_MessageCreateMultipartNoDuplicateUploadOnRetry(t *testing.T) {
	store := newCountingPutStore()
	e := newEndToEnd(t, "e2e-message-multipart-staff", "/practices/{practiceId}/engagements/{engagementId}/messages",
		message.CreateHandler(store, push.NewFakePusher()), true)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("body", "photo attached"); err != nil {
		t.Fatalf("write body field: %v", err)
	}
	part, err := writer.CreateFormFile("attachment", "photo.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(pngBytes); err != nil {
		t.Fatalf("write attachment content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	first := requireRetryReplays(t, func(key string) message.Message {
		return post[message.Message](t, e, "/engagements/"+e.EngagementID+"/messages", key,
			reqtest.Body(writer.FormDataContentType(), buf.Bytes()))
	}, func(m message.Message) string { return m.MessageID },
		func() int { return int(store.puts.Load()) })
	if first.AttachmentFilename != "photo.png" {
		t.Fatalf("first response AttachmentFilename = %q, want %q", first.AttachmentFilename, "photo.png")
	}
}
