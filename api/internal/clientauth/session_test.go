package clientauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/testdb"
)

// newSessionServer mounts the portal session route and seeds a live
// session for uid, returning the token its __session cookie carries.
func newSessionServer(t *testing.T, db *testdb.DB, uid string) (*httptest.Server, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /portal/session", clientauth.SessionHandler(db.App))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestSessionHandler_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := getSession(t, srv, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestSessionHandler_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestSessionHandler_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "irrelevant")
	defer srv.Close()

	resp := getSession(t, srv, "never-issued")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSessionHandler_UnknownClient(t *testing.T) {
	db := testdb.New(t)
	srv, session := newSessionServer(t, db, "no-such-client")
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSessionHandler_SingleEngagement(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "single-engagement-client"
	clientID, engagementID := seedClientWithEngagement(t, db, identityUID)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientID != clientID {
		t.Fatalf("clientId = %q, want %q", out.ClientID, clientID)
	}
	if len(out.Engagements) != 1 || out.Engagements[0].EngagementID != engagementID {
		t.Fatalf("engagements = %+v, want single engagement %q", out.Engagements, engagementID)
	}
	if out.Engagements[0].PracticeName != "Test Practice" {
		t.Fatalf("practiceName = %q, want %q", out.Engagements[0].PracticeName, "Test Practice")
	}
}

func TestSessionHandler_MultipleEngagements(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "multi-engagement-client"
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	clientID, engagementA := seedClientEngagement(t, db, practiceA, "Shared Client", "shared@example.com", "intake")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, 'intake')`,
		clientID, practiceB,
	); err != nil {
		t.Fatalf("seed second engagement: %v", err)
	}
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	var out clientauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Engagements) != 2 {
		t.Fatalf("expected 2 engagements, got %d", len(out.Engagements))
	}
	if out.Engagements[0].EngagementID != engagementA {
		t.Fatalf("expected first engagement (by created_at) = %q, got %q", engagementA, out.Engagements[0].EngagementID)
	}
}
