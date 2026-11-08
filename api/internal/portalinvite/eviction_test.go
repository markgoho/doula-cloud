package portalinvite_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// postAcceptWithSession claims an invitation while the browser holds the
// session named by cookieToken, confirming the eviction or not per
// confirmed. ADR-0026 makes the invitation the first magic link, so this
// is a Client sign-in and #610 governs it exactly as it governs a later
// one.
func postAcceptWithSession(t *testing.T, srv *httptest.Server, inviteToken, cookieToken string, confirmed bool) *http.Response {
	t.Helper()
	resp := reqtest.Post(t, srv.URL+"/api/portal/accept-invite",
		reqtest.JSON(portalinvite.AcceptInviteRequest{InviteToken: inviteToken}),
		reqtest.Session(cookieToken),
		reqtest.HeaderIf(confirmed, "X-Confirmed", "true"))
	return resp
}

func countSessionsFor(t *testing.T, db *testdb.DB, identityUID string) int {
	t.Helper()
	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, identityUID)
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
	out := apierrtest.Decode(t, resp)
	if out.Code != authn.EvictionUnconfirmed {
		t.Fatalf("code = %q, want %q", out.Code, authn.EvictionUnconfirmed)
	}
	if got := countSessionsFor(t, db, "staff-uid"); got != 1 {
		t.Errorf("Staff session rows = %d, want 1 -- the refusal must leave it alone", got)
	}
	// The refusal rolled back everything acceptInvite wrote, so the
	// invitation is still unclaimed and the confirmed retry claims it.
	identityUID := testdb.Scalar[string](t, db.Admin, `SELECT coalesce(identity_uid, '') FROM client_portal_users WHERE client_id = $1`, clientID)
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
