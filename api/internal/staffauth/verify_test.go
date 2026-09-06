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

func newVerifyRequestServer(t *testing.T, db *testdb.DB, uid string) (*httptest.Server, string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postVerifyRequest(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/verify-email/request", nil)
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

func newVerifySpendServer(accounts *authntest.FakeAccountManager, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, accounts, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func postVerifySpend(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/verify-email", strings.NewReader(body))
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

func countPendingTokenMail(t *testing.T, db *testdb.DB, identityUID, kind string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1 AND kind = $2 AND status = 'pending'`,
		identityUID, kind,
	).Scan(&count); err != nil {
		t.Fatalf("count pending token mail: %v", err)
	}
	return count
}

func countLiveAuthTokens(t *testing.T, db *testdb.DB, identityUID string, purpose authtoken.Purpose) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM auth_tokens WHERE identity_uid = $1 AND purpose = $2 AND used_at IS NULL`,
		identityUID, purpose,
	).Scan(&count); err != nil {
		t.Fatalf("count live auth tokens: %v", err)
	}
	return count
}

func TestRequestVerificationHandler_MissingCookieUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newVerifyRequestServer(t, db, "no-cookie-verify")
	defer srv.Close()

	resp := postVerifyRequest(t, srv, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRequestVerificationHandler_QueuesTokenMail(t *testing.T) {
	db := testdb.New(t)
	const uid = "verify-request-uid"
	srv, session := newVerifyRequestServer(t, db, uid)
	defer srv.Close()

	resp := postVerifyRequest(t, srv, session)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	if countLiveAuthTokens(t, db, uid, authtoken.PurposeStaffEmailVerification) != 1 {
		t.Fatal("expected exactly one live verification token")
	}
	if countPendingTokenMail(t, db, uid, "email_verification") != 1 {
		t.Fatal("expected exactly one pending verification outbox row")
	}
}

// TestRequestVerificationHandler_ReRequestSupersedesPriorToken proves the
// re-request AC: a second call kills the first token and still leaves
// exactly one pending outbox row, not two.
func TestRequestVerificationHandler_ReRequestSupersedesPriorToken(t *testing.T) {
	db := testdb.New(t)
	const uid = "verify-rerequest-uid"
	srv, session := newVerifyRequestServer(t, db, uid)
	defer srv.Close()

	first := postVerifyRequest(t, srv, session)
	_ = first.Body.Close()
	second := postVerifyRequest(t, srv, session)
	_ = second.Body.Close()

	if countLiveAuthTokens(t, db, uid, authtoken.PurposeStaffEmailVerification) != 1 {
		t.Fatal("expected exactly one live verification token after a re-request")
	}
	if countPendingTokenMail(t, db, uid, "email_verification") != 1 {
		t.Fatal("expected exactly one pending outbox row after a re-request")
	}
}

func TestSpendVerificationHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newVerifySpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postVerifySpend(t, srv, `{"token":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendVerificationHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newVerifySpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postVerifySpend(t, srv, `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendVerificationHandler_UnknownTokenInvalid(t *testing.T) {
	db := testdb.New(t)
	srv := newVerifySpendServer(authntest.NewFakeAccountManager(), db)
	defer srv.Close()

	resp := postVerifySpend(t, srv, `{"token":"never-minted"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSpendVerificationHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "verify-spend-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "person@example.com", false)
	token, err := authtoken.Mint(t.Context(), db.App, uid, authtoken.PurposeStaffEmailVerification, 24*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	srv := newVerifySpendServer(accounts, db)
	defer srv.Close()

	resp := postVerifySpend(t, srv, `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	account, err := accounts.GetAccount(t.Context(), uid)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !account.EmailVerified {
		t.Fatal("EmailVerified = false, want true")
	}

	// Single-use: spending the same link again is refused.
	replay := postVerifySpend(t, srv, `{"token":"`+token+`"}`)
	defer replay.Body.Close()
	if replay.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay status = %d, want %d", replay.StatusCode, http.StatusBadRequest)
	}
}

// TestSpendVerificationHandler_AccountManagerFailureRollsBackTheSpend
// proves a failed Admin SDK write leaves the token spendable again --
// the transaction that marked it used never committed.
func TestSpendVerificationHandler_AccountManagerFailureRollsBackTheSpend(t *testing.T) {
	db := testdb.New(t)
	const uid = "verify-spend-fail-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "person@example.com", false)
	token, err := authtoken.Mint(t.Context(), db.App, uid, authtoken.PurposeStaffEmailVerification, 24*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	accounts.Err = errors.New("admin sdk unreachable")
	srv := newVerifySpendServer(accounts, db)
	defer srv.Close()

	resp := postVerifySpend(t, srv, `{"token":"`+token+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	accounts.Err = nil
	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffEmailVerification, time.Now()); err != nil {
		t.Fatalf("token should still be spendable after the rollback: %v", err)
	}
}
