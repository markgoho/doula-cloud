package staffauth_test

import (
	"errors"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const newAddress = `{"newEmail":"new@example.com"}`

// newEmailChanger is a Staff member whose Identity Platform account holds
// old@example.com, verified, with the fake account manager the test may
// break.
func newEmailChanger(t *testing.T) (*env, *authntest.FakeAccountManager) {
	t.Helper()
	accounts := authntest.NewFakeAccountManager()
	e := newEnv(t, withAccounts(accounts))
	accounts.Seed(e.Staff.UID, "old@example.com", true)
	return e, accounts
}

// putEmail changes the address of the Staff member at the screen and
// requires want.
func (e *env) putEmail(body string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPut, e.URL("/api/staff/email"), want, e.SignedIn, reqtest.RawJSON([]byte(body)))
}

func TestChangeEmailHandler_MissingCookieUnauthorized(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPut, e.URL("/api/staff/email"), http.StatusUnauthorized, reqtest.Session(""), reqtest.RawJSON([]byte(newAddress)))
}

func TestChangeEmailHandler_RefusesAnUnreadableRequest(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"a missing new address", `{"newEmail":""}`},
		{"a body that is not JSON", notJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := newEmailChanger(t)

			e.putEmail(tc.body, http.StatusBadRequest)
		})
	}
}

// TestChangeEmailHandler_Success proves the full chain: the Admin SDK
// address changes and its verified flag clears, staff.email is kept in
// step with it (#614), and the *old* address is queued for the outbox
// notice.
func TestChangeEmailHandler_Success(t *testing.T) {
	e, accounts := newEmailChanger(t)

	e.putEmail(`{"newEmail":"  New@Example.com  "}`, http.StatusNoContent)

	account, err := accounts.GetAccount(t.Context(), e.Staff.UID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if account.Email != "new@example.com" {
		t.Fatalf("account email = %q, want new@example.com", account.Email)
	}
	if account.EmailVerified {
		t.Fatal("EmailVerified = true, want false after an address change")
	}

	staffEmail := testdb.Scalar[string](t, e.DB.Admin, `SELECT email FROM staff WHERE identity_uid = $1`, e.Staff.UID)
	if staffEmail != "new@example.com" {
		t.Fatalf("staff.email = %q, want new@example.com", staffEmail)
	}

	var oldEmail, status string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT old_email, status FROM staff_email_change_outbox WHERE identity_uid = $1`, e.Staff.UID,
	).Scan(&oldEmail, &status); err != nil {
		t.Fatalf("query email change outbox row: %v", err)
	}
	if oldEmail != "old@example.com" || status != statusPending {
		t.Fatalf("old_email/status = %q/%q, want old@example.com/%s", oldEmail, status, statusPending)
	}

	pendingVerify := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM staff_token_mail_outbox WHERE identity_uid = $1 AND kind = 'email_verification' AND status = 'pending'`, e.Staff.UID)
	if pendingVerify != 1 {
		t.Fatalf("pending verification rows = %d, want 1", pendingVerify)
	}
}

// TestChangeEmailHandler_AdminSDKFailureReturns500 covers changeEmail's
// two Admin SDK calls on their own: the GetAccount read, and the SetEmail
// write that follows it.
func TestChangeEmailHandler_AdminSDKFailureReturns500(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(*authntest.FakeAccountManager)
	}{
		{"the account cannot be read", func(a *authntest.FakeAccountManager) { a.Err = errors.New("admin sdk unreachable") }},
		{"the new address is rejected", func(a *authntest.FakeAccountManager) {
			a.SetEmailErr = errors.New("admin sdk rejected the write")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, accounts := newEmailChanger(t)
			tc.breakIt(accounts)

			e.putEmail(newAddress, http.StatusInternalServerError)
		})
	}
}

func TestChangeEmailHandler_NoStaffRowNotFound(t *testing.T) {
	accounts := authntest.NewFakeAccountManager()
	e := newEnv(t, withAccounts(accounts))
	// An Identity Platform account exists (the session resolves) but no
	// staff row does.
	const uid = "change-email-no-staff-uid"
	accounts.Seed(uid, "old@example.com", true)

	reqtest.ExpectStatus(t, http.MethodPut, e.URL("/api/staff/email"), http.StatusNotFound,
		reqtest.Session(authntest.SeedSession(t, e.DB.App, uid)), reqtest.RawJSON([]byte(newAddress)))
}
