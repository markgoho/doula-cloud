package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

func newRemoveSecondFactorServer(t *testing.T, db *testdb.DB, verifier authn.Verifier, accounts *authntest.FakeAccountManager) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, verifier, accounts, tasknudge.NoOpEnqueuer{})
	return httptest.NewServer(mux)
}

func deleteSecondFactor(t *testing.T, srv *httptest.Server, session, bearer string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/staff/mfa", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestRemoveSecondFactorHandler_Success proves the voluntary-removal
// mirror of #615's recovery paths: clears the Admin SDK factor, ends
// every live session, and records a self-caused staff_auth_events row.
func TestRemoveSecondFactorHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-removes-own-factor"
	staffID := seedStaff(t, db, identityUID)

	session := authntest.SeedSession(t, db.App, identityUID)
	authntest.SeedSession(t, db.App, identityUID) // a second device's session

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed(identityUID, identityUID+"@example.com", true)
	accounts.EnrollTOTP(identityUID)

	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: identityUID, AuthTime: time.Now()}, accounts)
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "fresh-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	if accounts.HasSecondFactor(identityUID) {
		t.Fatalf("expected the Admin SDK factor to be cleared")
	}
	if got := authntest.CountFor(t, db.App, identityUID); got != 0 {
		t.Fatalf("session rows = %d, want 0 -- every session should have ended", got)
	}

	var reason, actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT reason, actor_staff_id FROM staff_auth_events WHERE staff_id = $1`, staffID,
	).Scan(&reason, &actor); err != nil {
		t.Fatalf("read staff_auth_events row: %v", err)
	}
	if reason != "removed" {
		t.Fatalf("reason = %q, want %q", reason, "removed")
	}
	if actor != staffID {
		t.Fatalf("actor_staff_id = %q, want %q (self-caused)", actor, staffID)
	}
}

func TestRemoveSecondFactorHandler_MissingBearerUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-removes-no-bearer"
	seedStaff(t, db, identityUID)
	session := authntest.SeedSession(t, db.App, identityUID)

	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: identityUID, AuthTime: time.Now()}, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRemoveSecondFactorHandler_StaleReauthUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-removes-stale-reauth"
	seedStaff(t, db, identityUID)
	session := authntest.SeedSession(t, db.App, identityUID)

	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: identityUID, AuthTime: time.Now().Add(-time.Hour)}, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "stale-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRemoveSecondFactorHandler_ReauthUIDMismatchUnauthorized(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-removes-uid-mismatch"
	seedStaff(t, db, identityUID)
	session := authntest.SeedSession(t, db.App, identityUID)

	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: "someone-else", AuthTime: time.Now()}, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "someone-elses-token")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestRemoveSecondFactorHandler_UnknownStaffForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "unknown-remover"
	session := authntest.SeedSession(t, db.App, identityUID)

	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: identityUID, AuthTime: time.Now()}, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, session, "tok")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestRemoveSecondFactorHandler_MissingSessionUnauthorized(t *testing.T) {
	db := testdb.New(t)
	srv := newRemoveSecondFactorServer(t, db, authntest.Verifier{UID: "irrelevant", AuthTime: time.Now()}, authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := deleteSecondFactor(t, srv, "never-issued", "tok")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
