package clientauth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/testdb"
)

func newMagicLinkRequestServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /portal/magic-link/request", clientauth.RequestMagicLinkHandler(db.App))
	return httptest.NewServer(mux)
}

func newMagicLinkRedeemServer(db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /portal/magic-link", clientauth.RedeemMagicLinkHandler(db.App))
	return httptest.NewServer(mux)
}

func postJSON(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, strings.NewReader(body))
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

func countLiveMagicLinkTokens(t *testing.T, db *testdb.DB, identifier string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM auth_tokens WHERE identity_uid = $1 AND purpose = 'client_magic_link' AND used_at IS NULL`,
		identifier,
	).Scan(&count); err != nil {
		t.Fatalf("count auth_tokens: %v", err)
	}
	return count
}

func countPendingMagicLinkMail(t *testing.T, db *testdb.DB, identifier string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM portal_magic_link_outbox WHERE identity_uid = $1 AND status = 'pending'`, identifier,
	).Scan(&count); err != nil {
		t.Fatalf("count portal_magic_link_outbox: %v", err)
	}
	return count
}

func TestRequestMagicLinkHandler_MissingEmail(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link/request", `{"email":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRequestMagicLinkHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link/request", `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestRequestMagicLinkHandler_UnknownAddressStillAccepted proves #168's
// account-enumeration rule: an address that names no Portal Account gets
// the same response as one that does, and nothing is minted or queued.
func TestRequestMagicLinkHandler_UnknownAddressStillAccepted(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link/request", `{"email":"nobody@example.com"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM auth_tokens`).Scan(&count); err != nil {
		t.Fatalf("count auth_tokens: %v", err)
	}
	if count != 0 {
		t.Fatal("expected no token minted for an unknown address")
	}
}

func TestRequestMagicLinkHandler_KnownAddressQueuesLinkMail(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_request-known"
	testdb.SeedPortalAccount(t, db, identifier, "known@example.com")
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	// Case and whitespace should not matter -- the lookup normalizes both.
	resp := postJSON(t, srv, "/portal/magic-link/request", `{"email":"  Known@Example.com  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	if countLiveMagicLinkTokens(t, db, identifier) != 1 {
		t.Fatal("expected exactly one live magic-link token")
	}
	if countPendingMagicLinkMail(t, db, identifier) != 1 {
		t.Fatal("expected exactly one pending magic-link outbox row")
	}
}

// TestRequestMagicLinkHandler_ReRequestResetsTheSameRow proves the
// re-request rule both authtoken.Mint and portal_magic_link_outbox_one_pending
// enforce: asking twice before reading the first mail leaves exactly one
// live token and one pending outbox row, not two.
func TestRequestMagicLinkHandler_ReRequestResetsTheSameRow(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_request-rerequest"
	testdb.SeedPortalAccount(t, db, identifier, "rerequest@example.com")
	srv := newMagicLinkRequestServer(db)
	defer srv.Close()

	for range 2 {
		resp := postJSON(t, srv, "/portal/magic-link/request", `{"email":"rerequest@example.com"}`)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
		}
	}

	if countLiveMagicLinkTokens(t, db, identifier) != 1 {
		t.Fatal("expected exactly one live token after a re-request")
	}
	if countPendingMagicLinkMail(t, db, identifier) != 1 {
		t.Fatal("expected exactly one pending outbox row after a re-request")
	}
}

func TestRedeemMagicLinkHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link", `{"token":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRedeemMagicLinkHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link", `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRedeemMagicLinkHandler_UnknownTokenInvalid(t *testing.T) {
	db := testdb.New(t)
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link", `{"token":"never-minted"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestRedeemMagicLinkHandler_Success proves the token is single-use and
// spending it mints a session naming the Portal Account identifier.
func TestRedeemMagicLinkHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_redeem-success"
	testdb.SeedPortalAccount(t, db, identifier, "redeem@example.com")
	token, err := authtoken.Mint(t.Context(), db.App, identifier, authtoken.PurposeClientMagicLink, 15*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()

	resp := postJSON(t, srv, "/portal/magic-link", `{"token":"`+token+`"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var foundCookie bool
	for _, c := range resp.Cookies() {
		if c.Name == "__session" {
			foundCookie = true
			if c.Value == "" {
				t.Fatal("cookie value is empty")
			}
		}
	}
	_ = resp.Body.Close()
	if !foundCookie {
		t.Fatal("no __session cookie set on successful redeem")
	}

	var sessionCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM sessions WHERE identity_uid = $1`, identifier).Scan(&sessionCount); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("sessions = %d, want 1", sessionCount)
	}

	// Single-use.
	replay := postJSON(t, srv, "/portal/magic-link", `{"token":"`+token+`"}`)
	defer replay.Body.Close()
	if replay.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want %d", replay.StatusCode, http.StatusBadRequest)
	}
}

// TestRedeemMagicLinkHandler_SessionStoreFailureRollsBackTheSpend proves a
// failed session mint leaves the token spendable again, the same
// atomicity SpendResetHandler's own failure test proves.
func TestRedeemMagicLinkHandler_SessionStoreFailureRollsBackTheSpend(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_redeem-rollback"
	testdb.SeedPortalAccount(t, db, identifier, "rollback@example.com")
	token, err := authtoken.Mint(t.Context(), db.App, identifier, authtoken.PurposeClientMagicLink, 15*time.Minute, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	srv := newMagicLinkRedeemServer(db)
	defer srv.Close()
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := postJSON(t, srv, "/portal/magic-link", `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
