package portalinvite_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// doulaRole is named once so golangci-lint's goconst check doesn't see
// repeated "doula" literals across this package's test surface.
const doulaRole = "doula"

// invitedAddress is the address every fixture invitation in this package
// goes to. Named rather than repeated as a literal because two fixtures
// have to agree on it for a reuse accept to reuse anything: the Portal
// Account's sign-in address is the invited Client's own contact address
// (ADR-0026), so a test seeding both sides is silently testing nothing
// if the two drift apart.
const invitedAddress = "invited@example.com"

// newInviteServer mounts this package's whole surface through
// portalinvite.Mount, the same call main.go makes on the real GatedRouter
// and idempotency.Router, and seeds a live session for uid, returning the
// token its __session cookie carries.
func newInviteServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	portalinvite.Mount(g, ir, db.App, &tasknudge.FakeEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// newAcceptServer mounts this package's whole surface, the same as
// newInviteServer -- the Client-portal invitation-acceptance route is one
// of them. Public and pre-account (#617): the invitation token itself is
// the whole credential, so this reads no Bearer token and no session.
func newAcceptServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	portalinvite.Mount(g, ir, db.App, &tasknudge.FakeEnqueuer{})
	return httptest.NewServer(mux)
}

func postInvite(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string) *http.Response {
	t.Helper()
	url := srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/portal-invite"
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

func postAccept(t *testing.T, srv *httptest.Server, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/portal/accept-invite", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// seedPendingPortalInvite seeds a Practice, a Staff member, a Client with
// an Engagement at that Practice, and a pending (unclaimed) portal invite
// for the Client -- the state InviteHandler leaves behind for
// AcceptInviteHandler to pick up.
func seedPendingPortalInvite(t *testing.T, db *testdb.DB) (clientID, inviteToken string) {
	t.Helper()
	return seedPendingPortalInviteExpiringAt(t, db, time.Now().Add(7*24*time.Hour))
}

// seedPendingPortalInviteExpiringAt is seedPendingPortalInvite with an
// explicit invite_token_expires_at, for a test that drives the #616
// expiry check directly (a time in the past for an already-expired
// invite; NULL is what a fixture predating this migration would carry --
// no other caller needs that case, so no third helper exists for it).
func seedPendingPortalInviteExpiringAt(t *testing.T, db *testdb.DB, expiresAt time.Time) (clientID, inviteToken string) {
	t.Helper()
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, "portal-invite-owner", []string{doulaRole}, "employee")
	clientID, _ = testdb.SeedNamedEngagement(t, db, practiceID, "Invited Client", invitedAddress)
	return clientID, insertPendingPortalInvite(t, db, clientID, expiresAt)
}

// insertPendingPortalInvite writes the unclaimed client_portal_users row
// itself -- the one row AcceptInviteHandler's token names -- and hands
// back the token. Split out from the fixture above so a test that needs a
// differently shaped Practice around it (see the two-Practice walk in
// accept_second_practice_test.go) reuses this row's exact shape rather
// than writing a second INSERT that could drift from it.
func insertPendingPortalInvite(t *testing.T, db *testdb.DB, clientID string, expiresAt time.Time) (inviteToken string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token, invite_token_expires_at) VALUES ($1, gen_random_uuid(), $2) RETURNING invite_token::text`,
		clientID, expiresAt,
	).Scan(&inviteToken); err != nil {
		t.Fatalf("seed pending portal invite: %v", err)
	}
	return inviteToken
}
