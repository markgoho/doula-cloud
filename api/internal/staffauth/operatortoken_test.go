package staffauth_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const operatorTenant = "operators-local"

func assertOperatorRefusal(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	refusal := apierrtest.Decode(t, resp)
	if refusal.Code != apierr.CodeUnauthorized || refusal.Message != authn.MsgOperatorToken {
		t.Fatalf("refusal = %+v, want %s %q", refusal, apierr.CodeUnauthorized, authn.MsgOperatorToken)
	}
}

// #2470: the token verifies, names the session's own identity and is
// fresh, so only its tenant claim can refuse the reauth.
func TestRequireRecentAuth_RefusesAVerifiedOperatorToken(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-reauths-with-operator-token"
	testdb.SeedStaff(t, db, identityUID)
	session := authntest.SeedSession(t, db.App, identityUID)

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(identityUID, identityUID+"@example.com", true)
	accounts.EnrollTOTP(identityUID)

	verifier := authntest.Verifier{UID: identityUID, AuthTime: time.Now(), SecondFactor: true, Tenant: operatorTenant}
	srv := newRemoveSecondFactorServer(t, db, verifier, accounts)
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "operator-token")
	defer resp.Body.Close()

	assertOperatorRefusal(t, resp)
	if !accounts.HasSecondFactor(identityUID) {
		t.Fatal("the second factor was removed on an operator token")
	}
	if got := authntest.CountFor(t, db.App, identityUID); got != 1 {
		t.Fatalf("session rows = %d, want the existing 1", got)
	}
}

// A bootstrap route mints a session too, so an operator token must not
// become a Staff member there either.
func TestSignupHandler_RefusesAVerifiedOperatorToken(t *testing.T) {
	db := testdb.New(t)
	const uid = "operator-signs-up"
	srv := newSignupServer(authntest.Verifier{UID: uid, Email: "operator@example.com", AuthTime: time.Now(), Tenant: operatorTenant}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "operator-token", staffauth.SignupRequest{WorkState: "NY", PracticeName: "P", FirstName: "S", LastName: placeholderLastName})
	defer resp.Body.Close()

	assertOperatorRefusal(t, resp)
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set for an operator token: %+v", c)
	}
	var staffRows int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM staff WHERE identity_uid = $1`, uid).Scan(&staffRows); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffRows != 0 {
		t.Fatalf("staff rows = %d, want 0", staffRows)
	}
}
