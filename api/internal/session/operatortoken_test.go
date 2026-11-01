package session_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
)

const operatorTenant = "operators-local"

// #2470: the fake verifies the token, so only the tenant claim can refuse it.
func TestCreateHandler_RefusesAVerifiedOperatorToken(t *testing.T) {
	const uid = "operator-uid"
	srv, db := newServer(t, authntest.Verifier{UID: uid, AuthTime: time.Now(), SecondFactor: true, Tenant: operatorTenant})
	defer srv.Close()

	resp := postCreate(t, srv, "operator-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	refusal := apierrtest.Decode(t, resp)
	if refusal.Code != apierr.CodeUnauthorized || refusal.Message != authn.MsgOperatorToken {
		t.Fatalf("refusal = %+v, want %s %q", refusal, apierr.CodeUnauthorized, authn.MsgOperatorToken)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set for an operator token: %+v", c)
	}
	if got := authntest.CountFor(t, db.Admin, uid); got != 0 {
		t.Fatalf("session rows = %d, want 0", got)
	}
}
