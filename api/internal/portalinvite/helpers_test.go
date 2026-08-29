package portalinvite_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

var errBadToken = errors.New("invalid token")

// newInviteServer mounts the Staff-side portal-invite route behind
// staffauth.Middleware and seeds a live session for uid, returning the
// token its __session cookie carries.
func newInviteServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/portal-invite",
		staffauth.Middleware(db.App)(portalinvite.InviteHandler(&tasknudge.FakeEnqueuer{})))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// newAcceptServer mounts the Client-portal invitation-acceptance route.
// It is one of the three bootstrap endpoints, so it still reads a Bearer
// ID token: it runs before a session exists.
func newAcceptServer(verifier authn.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /portal/accept-invite", portalinvite.AcceptInviteHandler(verifier, db.App))
	return httptest.NewServer(mux)
}

func postInvite(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string) *http.Response {
	t.Helper()
	url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/portal-invite"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func postAccept(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/portal/accept-invite", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

// seedStaffWithMembership seeds a Practice and a Staff member holding a
// membership there. Per #90's scope decision, portal-invite gating is
// practice-tier, so the doula role (no owner) is enough to exercise it.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Portal Invite Test Practice")
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Test Staff', 'staff@example.com', 'NY') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return practiceID
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $3) RETURNING id`,
		practiceID, name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPendingPortalInvite seeds a Practice, a Staff member, a Client with
// an Engagement at that Practice, and a pending (unclaimed) portal invite
// for the Client -- the state InviteHandler leaves behind for
// AcceptInviteHandler to pick up.
func seedPendingPortalInvite(t *testing.T, db *testdb.DB) (clientID, inviteToken string) {
	t.Helper()
	practiceID := seedStaffWithMembership(t, db, "portal-invite-owner")
	clientID, _ = seedClientEngagement(t, db, practiceID, "Invited Client", "invited@example.com")

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING invite_token::text`,
		clientID,
	).Scan(&inviteToken); err != nil {
		t.Fatalf("seed pending portal invite: %v", err)
	}
	return clientID, inviteToken
}
