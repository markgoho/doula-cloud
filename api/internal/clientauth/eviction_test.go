package clientauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/testdb"
)

// postRedeem spends token behind the Continue button while the browser
// holds the session named by cookieToken, confirming the eviction or not
// per confirmed.
func postRedeem(t *testing.T, srv *httptest.Server, token, cookieToken string, confirmed bool) *http.Response {
	t.Helper()
	body := strings.NewReader(`{"token":"` + token + `"}`)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/portal/magic-link", body)
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

// seedMagicLink puts a live Portal Account and an unspent sign-in link
// behind it, the state a Continue press arrives in.
func seedMagicLink(t *testing.T, db *testdb.DB, identifier string) string {
	t.Helper()
	testdb.SeedPortalAccount(t, db, identifier, identifier+"@example.com")
	token, err := authtoken.Mint(t.Context(), db.App, identifier, authtoken.PurposeClientMagicLink, 15*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return token
}

func countRows(t *testing.T, db *testdb.DB, query, arg string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), query, arg).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	return count
}

// A doula who is also a Client, following her sign-in link on the laptop
// where her Practice session is live, is told what continuing costs --
// and the link is still hers to spend afterwards.
func TestRedeemMagicLinkHandler_UnconfirmedStaffSessionWarnsAndSpendsNothing(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_evict-warn"
	token := seedMagicLink(t, db, identifier)
	staffToken := authntest.SeedSession(t, db.App, "staff-uid")
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postRedeem(t, srv, token, staffToken, false)
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
	if got := countRows(t, db, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, identifier); got != 0 {
		t.Errorf("portal session rows = %d, want 0 before she confirms", got)
	}
	if got := countRows(t, db, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, "staff-uid"); got != 1 {
		t.Errorf("Staff session rows = %d, want 1 -- the refusal must leave it alone", got)
	}

	// The refusal rolled the spend back, so the same live link redeems on
	// the confirmed retry rather than being burned by the warning.
	retry := postRedeem(t, srv, token, staffToken, true)
	defer retry.Body.Close()
	if retry.StatusCode != http.StatusOK {
		t.Fatalf("confirmed retry status = %d, want %d", retry.StatusCode, http.StatusOK)
	}
}

func TestRedeemMagicLinkHandler_ConfirmedEvictsAndNotifiesTheStaffSession(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_evict-confirm"
	token := seedMagicLink(t, db, identifier)
	staffToken := authntest.SeedSession(t, db.App, "staff-uid")
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postRedeem(t, srv, token, staffToken, true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countRows(t, db, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, identifier); got != 1 {
		t.Errorf("portal session rows = %d, want 1", got)
	}
	// Deleted, not left to expire: an evicted token that still verifies
	// is the defect #610's AC names.
	if got := countRows(t, db, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, "staff-uid"); got != 0 {
		t.Errorf("Staff session rows = %d, want 0", got)
	}
	if got := countRows(t,
		db, `SELECT count(*) FROM session_notice_outbox WHERE identity_uid = $1 AND kind = 'session_evicted'`, "staff-uid",
	); got != 1 {
		t.Errorf("eviction notices for the evicted Staff member = %d, want 1", got)
	}
}

// A live portal session is not a cross-population eviction: redeeming a
// fresh link is an ordinary re-sign-in, and goes straight through.
func TestRedeemMagicLinkHandler_LivePortalSessionSignsStraightThrough(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_evict-same-tier"
	token := seedMagicLink(t, db, identifier)
	existing := authntest.SeedSession(t, db.App, "portal_someone-else")
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postRedeem(t, srv, token, existing, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := countRows(t, db, `SELECT count(*) FROM sessions WHERE identity_uid = $1`, "portal_someone-else"); got != 1 {
		t.Errorf("the other portal session rows = %d, want 1 -- a same-population sign-in evicts nothing", got)
	}
}

// A dead link is refused before any warning is written: a stranger
// following a burned link learns nothing about whose browser he is in.
func TestRedeemMagicLinkHandler_DeadTokenIsRefusedBeforeTheWarning(t *testing.T) {
	db := testdb.New(t)
	staffToken := authntest.SeedSession(t, db.App, "staff-uid")
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postRedeem(t, srv, "never-issued", staffToken, false)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
