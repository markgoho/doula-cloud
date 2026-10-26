package idempotency_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// portalClient is one signed-in Client on her own Engagement, the caller
// a portal Idempotency-Key is scoped to (#1924).
type portalClient struct {
	session      string
	engagementID string
}

// seedPortalClient seeds a Client on a new Engagement at practiceID, gives
// her a Portal Account, and signs her in.
func seedPortalClient(t *testing.T, db *testdb.DB, practiceID, uid, name, email string) portalClient {
	t.Helper()
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, name, email)
	testdb.SeedPortalUser(t, db, testdb.PortalUID(uid), clientID)
	return portalClient{session: authntest.SeedSession(t, db.App, testdb.PortalUID(uid)), engagementID: engagementID}
}

// newPortalMessageServer mounts message.Mount exactly as routes() does, so
// the portal create goes through idempotency.Router.PortalReplayable --
// clientauth.Middleware, then WrapClient, then ClientCreateHandler.
func newPortalMessageServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	message.Mount(g, ir, db.App, objectstore.NewMemoryStore(), push.NewFakePusher())
	return httptest.NewServer(mux)
}

func postPortalMessage(t *testing.T, srv *httptest.Server, c portalClient, key, text string) (status int, body []byte) {
	t.Helper()
	payload, err := json.Marshal(message.CreateRequest{Body: text})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/portal/engagements/"+c.engagementID+"/messages", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, c.session)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, body
}

func countMessages(t *testing.T, db *testdb.DB, engagementID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM messages WHERE engagement_id = $1`, engagementID,
	).Scan(&n); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	return n
}

// TestEndToEnd_PortalMessageReplaysOnRetry is #1924's end-to-end proof: a
// Client's portal Message retried with the same Idempotency-Key creates no
// second messages row and gets the first response back, keyed to her own
// Client -- while the same request with no key (the control) still posts
// every time.
func TestEndToEnd_PortalMessageReplaysOnRetry(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Portal Retry Practice")
	client := seedPortalClient(t, db, practiceID, "portal-retry-client", "Retry Client", "retry@example.com")
	srv := newPortalMessageServer(t, db)
	defer srv.Close()

	firstStatus, firstBody := postPortalMessage(t, srv, client, "portal-message-retry-key", "Is Tuesday still on?")
	if firstStatus != http.StatusCreated {
		t.Fatalf("first call status = %d, want %d: %s", firstStatus, http.StatusCreated, firstBody)
	}
	retriedStatus, retriedBody := postPortalMessage(t, srv, client, "portal-message-retry-key", "Is Tuesday still on?")
	if retriedStatus != http.StatusCreated {
		t.Fatalf("retried call status = %d, want %d", retriedStatus, http.StatusCreated)
	}
	if !bytes.Equal(firstBody, retriedBody) {
		t.Fatalf("retried body = %s, want the first response replayed: %s", retriedBody, firstBody)
	}
	if n := countMessages(t, db, client.engagementID); n != 1 {
		t.Fatalf("messages after a same-key retry = %d, want 1", n)
	}

	// The control: no key, so nothing to replay -- both posts land.
	for range 2 {
		if status, body := postPortalMessage(t, srv, client, "", "Is Tuesday still on?"); status != http.StatusCreated {
			t.Fatalf("no-key call status = %d, want %d: %s", status, http.StatusCreated, body)
		}
	}
	if n := countMessages(t, db, client.engagementID); n != 3 {
		t.Fatalf("messages after two no-key posts = %d, want 3", n)
	}
}

// TestEndToEnd_PortalKeyIsScopedToItsOwnClient proves a key is one
// Client's, not a global value: a second Client at the same Practice
// sending the same key value posts her own Message rather than receiving
// the first Client's stored response.
func TestEndToEnd_PortalKeyIsScopedToItsOwnClient(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Portal Scope Practice")
	first := seedPortalClient(t, db, practiceID, "portal-scope-first", "First Client", "first@example.com")
	second := seedPortalClient(t, db, practiceID, "portal-scope-second", "Second Client", "second@example.com")
	srv := newPortalMessageServer(t, db)
	defer srv.Close()

	_, firstBody := postPortalMessage(t, srv, first, "shared-key", "From the first Client")
	status, secondBody := postPortalMessage(t, srv, second, "shared-key", "From the second Client")
	if status != http.StatusCreated {
		t.Fatalf("second Client status = %d, want %d: %s", status, http.StatusCreated, secondBody)
	}
	if bytes.Equal(firstBody, secondBody) {
		t.Fatal("second Client received the first Client's stored response for the same key value")
	}
	if n := countMessages(t, db, second.engagementID); n != 1 {
		t.Fatalf("second Client's messages = %d, want 1", n)
	}
}
