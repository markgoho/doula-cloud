package portal_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts the same route main.go wires up for this package,
// behind clientauth.Middleware.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /portal/engagements/{engagementId}",
		clientauth.Middleware(db.App)(portal.DetailHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedClientWithEngagement inserts a Practice, a Client with an
// Engagement at it, and a client_portal_users row linking identityUID to
// that Client, using the superuser Admin connection.
func seedClientWithEngagement(t *testing.T, db *testdb.DB, identityUID, practiceName string) (engagementID, status string) {
	t.Helper()

	var practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, practiceName,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ('Test Client', 'client@example.com') RETURNING id`,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	status = "intake"
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, $3) RETURNING id`,
		clientID, practiceID, status,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}

	return engagementID, status
}

func TestDetailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-detail-uid"
	engagementID, status := seedClientWithEngagement(t, db, identityUID, "Riverside Doulas")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/"+engagementID, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out portal.Detail
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, engagementID)
	}
	if out.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", out.PracticeName, "Riverside Doulas")
	}
	if out.Status != status {
		t.Fatalf("status = %q, want %q", out.Status, status)
	}
}

func TestDetailHandler_NotLinkedToClient(t *testing.T) {
	db := testdb.New(t)
	_, _ = seedClientWithEngagement(t, db, "other-portal-uid", "Other Practice")

	srv, session := newServer(t, db, "unrelated-uid")
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/portal/engagements/00000000-0000-0000-0000-000000000000", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
