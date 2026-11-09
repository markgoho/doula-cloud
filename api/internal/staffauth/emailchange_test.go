package staffauth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func newChangeEmailServer(t *testing.T, db *testdb.DB, accounts *authntest.FakeAccountManager, uid string) (*httptest.Server, string) {
	t.Helper()
	return serveStaffauth(db, authntest.Verifier{}, accounts, tasknudge.NoOpEnqueuer{}), authntest.SeedSession(t, db.App, uid)
}

func putEmail(t *testing.T, srv *httptest.Server, session, body string) *http.Response {
	t.Helper()
	return reqtest.Put(t, srv.URL+"/api/staff/email", reqtest.Session(session), reqtest.RawJSON([]byte(body)))
}

func TestChangeEmailHandler_MissingCookieUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newChangeEmailServer(t, db, authntest.NewFakeAccountManager(), "no-cookie-change-email")
	defer srv.Close()

	resp := putEmail(t, srv, "", `{"newEmail":"new@example.com"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

func TestChangeEmailHandler_MissingNewEmail(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-missing-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "old@example.com", true)
	testdb.SeedStaff(t, db, uid)
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `{"newEmail":""}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

func TestChangeEmailHandler_InvalidRequestBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-invalid-body-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "old@example.com", true)
	testdb.SeedStaff(t, db, uid)
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `not json`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestChangeEmailHandler_Success proves the full chain: the Admin SDK
// address changes and its verified flag clears, staff.email is kept in
// step with it (#614), and the *old* address is queued for the outbox
// notice.
func TestChangeEmailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-success-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "old@example.com", true)
	testdb.SeedStaff(t, db, uid)
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `{"newEmail":"  New@Example.com  "}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusNoContent)

	account, err := accounts.GetAccount(t.Context(), uid)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if account.Email != "new@example.com" {
		t.Fatalf("account email = %q, want new@example.com", account.Email)
	}
	if account.EmailVerified {
		t.Fatal("EmailVerified = true, want false after an address change")
	}

	staffEmail := testdb.Scalar[string](t, db.Admin, `SELECT email FROM staff WHERE identity_uid = $1`, uid)
	if staffEmail != "new@example.com" {
		t.Fatalf("staff.email = %q, want new@example.com", staffEmail)
	}

	var oldEmail, status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT old_email, status FROM staff_email_change_outbox WHERE identity_uid = $1`, uid,
	).Scan(&oldEmail, &status); err != nil {
		t.Fatalf("query email change outbox row: %v", err)
	}
	if oldEmail != "old@example.com" || status != statusPending {
		t.Fatalf("old_email/status = %q/%q, want old@example.com/%s", oldEmail, status, statusPending)
	}

	pendingVerify := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1 AND kind = 'email_verification' AND status = 'pending'`, uid)
	if pendingVerify != 1 {
		t.Fatalf("pending verification rows = %d, want 1", pendingVerify)
	}
}

func TestChangeEmailHandler_GetAccountFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-get-fail-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Err = errors.New("admin sdk unreachable")
	testdb.SeedStaff(t, db, uid)
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `{"newEmail":"new@example.com"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusInternalServerError)
}

// TestChangeEmailHandler_SetEmailFailureReturns500 proves changeEmail's
// own SetEmail failure branch, distinct from a failure of the GetAccount
// read that precedes it.
func TestChangeEmailHandler_SetEmailFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-set-fail-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "old@example.com", true)
	accounts.SetEmailErr = errors.New("admin sdk rejected the write")
	testdb.SeedStaff(t, db, uid)
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `{"newEmail":"new@example.com"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusInternalServerError)
}

func TestChangeEmailHandler_NoStaffRowNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "change-email-no-staff-uid"
	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(uid, "old@example.com", true)
	// Deliberately no seedStaff call -- an Identity Platform account
	// exists (the session resolves) but no staff row does.
	srv, session := newChangeEmailServer(t, db, accounts, uid)
	defer srv.Close()

	resp := putEmail(t, srv, session, `{"newEmail":"new@example.com"}`)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}
