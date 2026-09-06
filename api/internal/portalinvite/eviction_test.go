package portalinvite_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

// postAcceptWithSession claims an invitation while the browser holds the
// session named by cookieToken, confirming the eviction or not per
// confirmed. ADR-0026 makes the invitation the first magic link, so this
// is a Client sign-in and #610 governs it exactly as it governs a later
// one.
func postAcceptWithSession(t *testing.T, srv *httptest.Server, inviteToken, cookieToken string, confirmed bool) *http.Response {
	t.Helper()
	payload, err := json.Marshal(portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	if err != nil {
		// coverage:ignore reason: marshalling a two-field struct cannot fail
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/portal/accept-invite", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, cookieToken)
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func countSessionsFor(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM sessions WHERE identity_uid = $1`, identityUID,
	).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return count
}

func TestAcceptInviteHandler_UnconfirmedStaffSessionWarnsAndClaimsNothing(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)
	staffToken := authntest.SeedSession(t, db.App, "staff-uid")
	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAcceptWithSession(t, srv, inviteToken, staffToken, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Code != string(authn.EvictionUnconfirmed) {
		t.Fatalf("code = %q, want %q", out.Code, authn.EvictionUnconfirmed)
	}
	if got := countSessionsFor(t, db, "staff-uid"); got != 1 {
		t.Errorf("Staff session rows = %d, want 1 -- the refusal must leave it alone", got)
	}
	// The refusal rolled back everything acceptInvite wrote, so the
	// invitation is still unclaimed and the confirmed retry claims it.
	var identityUID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT coalesce(identity_uid, '') FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&identityUID); err != nil {
		t.Fatalf("query portal row: %v", err)
	}
	if identityUID != "" {
		t.Fatalf("identity_uid = %q, want empty -- a refused accept may claim nothing", identityUID)
	}

	retry := postAcceptWithSession(t, srv, inviteToken, staffToken, true)
	defer retry.Body.Close()
	if retry.StatusCode != http.StatusOK {
		t.Fatalf("confirmed retry status = %d, want %d", retry.StatusCode, http.StatusOK)
	}
	if got := countSessionsFor(t, db, "staff-uid"); got != 0 {
		t.Errorf("Staff session rows after the confirmed accept = %d, want 0", got)
	}
}
