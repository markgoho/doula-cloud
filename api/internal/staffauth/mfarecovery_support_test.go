package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const supportTestSecret = "support-internal-test-secret"

func newSupportClearServer(accounts *authntest.FakeAccountManager, db *testdb.DB, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /internal/staffauth/mfa-recovery/support-clear", staffauth.SupportClearHandler(accounts, db.App, secret))
	return httptest.NewServer(mux)
}

func supportClearRequest(t *testing.T, srv *httptest.Server, secret, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/internal/staffauth/mfa-recovery/support-clear", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestSupportClearHandler_RefusesWithoutTheInternalSecret(t *testing.T) {
	db := testdb.New(t)
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, supportTestSecret)
	defer srv.Close()

	for _, secret := range []string{"", "wrong-secret"} {
		resp := supportClearRequest(t, srv, secret, `{}`)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("secret %q: status = %d, want 401", secret, resp.StatusCode)
		}
	}
}

func TestSupportClearHandler_EmptyConfiguredSecretAlwaysUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, "")
	defer srv.Close()

	resp := supportClearRequest(t, srv, "", `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSupportClearHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, supportTestSecret)
	defer srv.Close()

	resp := supportClearRequest(t, srv, supportTestSecret, `not json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSupportClearHandler_MissingOperator(t *testing.T) {
	db := testdb.New(t)
	staffID := seedStaff(t, db, "support-target-no-operator")
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, supportTestSecret)
	defer srv.Close()

	resp := supportClearRequest(t, srv, supportTestSecret, `{"staffId":"`+staffID+`","operator":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSupportClearHandler_MalformedStaffID(t *testing.T) {
	db := testdb.New(t)
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, supportTestSecret)
	defer srv.Close()

	resp := supportClearRequest(t, srv, supportTestSecret, `{"staffId":"not-a-uuid","operator":"Jamie"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestSupportClearHandler_UnknownStaffID(t *testing.T) {
	db := testdb.New(t)
	srv := newSupportClearServer(authntest.NewFakeAccountManager(), db, supportTestSecret)
	defer srv.Close()

	resp := supportClearRequest(t, srv, supportTestSecret, `{"staffId":"`+emptyUUID+`","operator":"Jamie"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// TestSupportClearHandler_Success covers #605's last-resort path: the
// enrolment clears and the audit row names the operator by free text,
// carrying no actor_staff_id -- she is not a Staff member.
func TestSupportClearHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const targetUID = "support-target-success"
	targetID := seedStaff(t, db, targetUID)
	authntest.SeedSession(t, db.App, targetUID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(targetUID, targetUID+"@example.com", true)
	accounts.EnrollTOTP(targetUID)
	srv := newSupportClearServer(accounts, db, supportTestSecret)
	defer srv.Close()

	resp := supportClearRequest(t, srv, supportTestSecret, `{"staffId":"`+targetID+`","operator":"Jamie Chen"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if accounts.HasSecondFactor(targetUID) {
		t.Fatal("TOTP enrolment still present, want it cleared")
	}
	if got := authntest.CountFor(t, db.App, targetUID); got != 0 {
		t.Fatalf("session rows = %d, want 0", got)
	}

	var reason, actorOperator string
	var actorStaffID *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT reason::text, actor_operator, actor_staff_id FROM staff_auth_events WHERE staff_id = $1`, targetID,
	).Scan(&reason, &actorOperator, &actorStaffID); err != nil {
		t.Fatalf("query staff_auth_events: %v", err)
	}
	if reason != "support" || actorOperator != "Jamie Chen" || actorStaffID != nil {
		t.Fatalf("event = (%q, %q, %v), want (support, Jamie Chen, nil)", reason, actorOperator, actorStaffID)
	}
}
