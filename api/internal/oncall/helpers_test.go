package oncall_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// Named once each so goconst does not see independent literals across
// this package's tests.
const (
	doulaRole    = "doula"
	ownerRole    = "owner"
	adminRole    = "admin"
	employeeType = "employee"
	zone         = "America/New_York"
)

// newServer mounts this package's whole surface through oncall.Mount, the
// same call main.go makes, and seeds a live session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	srv, session, _ = newServerWithNudge(t, db, uid)
	return srv, session
}

// newServerWithNudge is newServer plus the fake enqueuer the gap writes
// nudge, so a test can say whether the worker was nudged.
func newServerWithNudge(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string, enq *tasknudge.FakeEnqueuer) {
	t.Helper()
	enq = &tasknudge.FakeEnqueuer{}
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	oncall.Mount(g, ir, enq)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid), enq
}

func authedGet(t *testing.T, session, url string) *http.Response {
	t.Helper()
	return authedBody(t, session, http.MethodGet, url, nil)
}

func authedBody(t *testing.T, session, method, url string, body any) *http.Response {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", t.Name()+method+url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// decode reads resp's body into T, failing on any status but want.
func decode[T any](t *testing.T, resp *http.Response, want int) T {
	t.Helper()
	defer resp.Body.Close()
	var out T
	if resp.StatusCode != want {
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, want, body.String())
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// seedBirth inserts an active birth Engagement with dueDate (YYYY-MM-DD,
// or "" for none), the shape every window starts from.
func seedBirth(t *testing.T, db *testdb.DB, practiceID, name, dueDate string) (engagementID string) {
	t.Helper()
	clientID := testdb.SeedNamedClient(t, db, practiceID, name, name+"@example.com")
	var due any
	if dueDate != "" {
		due = dueDate
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind, status, due_date)
		 VALUES ($1, $2, 'birth', 'active', $3) RETURNING id`,
		clientID, practiceID, due,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed birth engagement: %v", err)
	}
	return engagementID
}

// exec runs one fixture statement on the superuser connection.
func exec(t *testing.T, db *testdb.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("fixture %q: %v", query, err)
	}
}

func rosterURL(srvURL, practiceID, from, to string) string {
	return srvURL + "/api/practices/" + practiceID + "/on-call?from=" + from + "&to=" + to
}
