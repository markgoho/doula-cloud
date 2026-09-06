package staffauth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func newResetRequestServer(accounts *authntest.FakeAccountManager, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, accounts, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func newResetSpendServer(accounts *authntest.FakeAccountManager, db *testdb.DB) *httptest.Server {
	return newResetRequestServer(accounts, db)
}

func postJSONTo(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
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

func TestRequestResetHandler_MissingEmail(t *testing.T) {
	db := testdb.New(t)
	srv := newResetRequestServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset/request", `{"email":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestRequestResetHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newResetRequestServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset/request", `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestRequestResetHandler_UnknownAddressStillAccepted proves #168's
// account-enumeration rule: an address that names no account gets the
// same response as one that does, and no token or outbox row is ever
// created for it.
func TestRequestResetHandler_UnknownAddressStillAccepted(t *testing.T) {
	db := testdb.New(t)
	srv := newResetRequestServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset/request", `{"email":"nobody@example.com"}`)
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

func TestRequestResetHandler_KnownAddressQueuesTokenMail(t *testing.T) {
	db := testdb.New(t)
	accounts := authntest.NewFakeAccountManager()
	const uid = "reset-request-uid"
	accounts.Seed(uid, "known@example.com", true)
	srv := newResetRequestServer(accounts, db)
	defer srv.Close()

	// Case and whitespace should not matter -- NormalizeAddress handles both.
	resp := postJSONTo(t, srv, "/api/staff/password-reset/request", `{"email":"  Known@Example.com  "}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	if countLiveAuthTokens(t, db, uid, authtoken.PurposeStaffPasswordReset) != 1 {
		t.Fatal("expected exactly one live reset token")
	}
	if countPendingTokenMail(t, db, uid, "password_reset") != 1 {
		t.Fatal("expected exactly one pending reset outbox row")
	}
}

func TestRequestResetHandler_AccountManagerErrorReturns500(t *testing.T) {
	db := testdb.New(t)
	accounts := authntest.NewFakeAccountManager()
	accounts.Err = errors.New("admin sdk unreachable")
	srv := newResetRequestServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset/request", `{"email":"anyone@example.com"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestSpendResetHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newResetSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"","newPassword":"longenough"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendResetHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newResetSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendResetHandler_PasswordTooShort(t *testing.T) {
	db := testdb.New(t)
	srv := newResetSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"some-token","newPassword":"short"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendResetHandler_UnknownTokenInvalid(t *testing.T) {
	db := testdb.New(t)
	srv := newResetSpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"never-minted","newPassword":"longenough"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestSpendResetHandler_Success proves the three properties reset has
// that verification does not: no session minted, every existing session
// ended, and the password actually changed.
func TestSpendResetHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "reset-spend-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "person@example.com", true)
	authntest.SeedSession(t, db.App, uid)
	token, err := authtoken.Mint(t.Context(), db.App, uid, authtoken.PurposeStaffPasswordReset, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	srv := newResetSpendServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"`+token+`","newPassword":"a-new-password"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_ = resp.Body.Close()
	if len(resp.Cookies()) > 0 {
		t.Fatal("a reset response must not mint a session cookie")
	}

	if accounts.Password(uid) != "a-new-password" {
		t.Fatalf("Password = %q, want a-new-password", accounts.Password(uid))
	}
	if authntest.CountFor(t, db.App, uid) != 0 {
		t.Fatal("expected every existing session to be ended")
	}

	// Single-use.
	replay := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"`+token+`","newPassword":"another-password"}`)
	defer replay.Body.Close()
	if replay.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want %d", replay.StatusCode, http.StatusBadRequest)
	}
}

// TestSpendResetHandler_SetPasswordFailureRollsBackTheSpend proves a
// failed Admin SDK write leaves the token spendable again.
func TestSpendResetHandler_SetPasswordFailureRollsBackTheSpend(t *testing.T) {
	db := testdb.New(t)
	const uid = "reset-spend-fail-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "person@example.com", true)
	token, err := authtoken.Mint(t.Context(), db.App, uid, authtoken.PurposeStaffPasswordReset, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	accounts.Err = errors.New("admin sdk unreachable")
	srv := newResetSpendServer(accounts, db)
	defer srv.Close()

	resp := postJSONTo(t, srv, "/api/staff/password-reset", `{"token":"`+token+`","newPassword":"a-new-password"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	accounts.Err = nil
	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffPasswordReset, time.Now()); err != nil {
		t.Fatalf("token should still be spendable after the rollback: %v", err)
	}
}
