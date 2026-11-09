package staffauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func newFinishEnrollmentServer(t *testing.T, db *testdb.DB, verifier authntest.Verifier) *httptest.Server {
	t.Helper()
	return serveStaffauth(db, verifier, authntest.NewFakeAccountManager())
}

func postFinishEnrollment(t *testing.T, srv *httptest.Server) *http.Response {
	t.Helper()
	return postFinishEnrollmentWithSession(t, srv, "")
}

func postFinishEnrollmentWithSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	return reqtest.Post(t, srv.URL+"/api/staff/mfa", reqtest.Session(session), reqtest.Header("Authorization", "Bearer tok"))
}

// TestFinishEnrollmentHandler_Success proves decision 4's happy path: a
// token that shows a second factor mints a fresh session carrying it and
// records the enrollment in staff_auth_events.
func TestFinishEnrollmentHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-finishes-enrollment"
	staffID := testdb.SeedStaff(t, db, identityUID)

	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{UID: identityUID, SecondFactor: true})
	defer srv.Close()

	resp := postFinishEnrollment(t, srv)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == authn.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatalf("no __session cookie set")
	}

	secondFactor := testdb.Scalar[bool](t, db.Admin, `SELECT second_factor FROM sessions WHERE identity_uid = $1`, identityUID)
	if !secondFactor {
		t.Fatalf("session.second_factor = false, want true")
	}

	var reason, actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT reason, actor_staff_id FROM staff_auth_events WHERE staff_id = $1`, staffID,
	).Scan(&reason, &actor); err != nil {
		t.Fatalf("read staff_auth_events row: %v", err)
	}
	if reason != "enrolled" {
		t.Fatalf("reason = %q, want %q", reason, "enrolled")
	}
	if actor != staffID {
		t.Fatalf("actor_staff_id = %q, want %q (self-caused)", actor, staffID)
	}
}

// TestFinishEnrollmentHandler_TokenWithoutSecondFactorRejected is
// decision 4's fallback trigger: a token that doesn't carry the claim
// (enroll() didn't actually finish, or the token is stale) is rejected
// rather than silently minting a session that lies about her enrollment.
func TestFinishEnrollmentHandler_TokenWithoutSecondFactorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-stale-token"
	testdb.SeedStaff(t, db, identityUID)

	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{UID: identityUID, SecondFactor: false})
	defer srv.Close()

	resp := postFinishEnrollment(t, srv)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
	if got := authntest.CountFor(t, db.App, identityUID); got != 0 {
		t.Fatalf("session rows = %d, want 0 -- no session should be minted", got)
	}
}

// TestFinishEnrollmentHandler_UnknownStaff pins the family's answer,
// which this route did not give until #1182: it refused a caller with no
// staff row with 403, alone with DELETE /api/staff/mfa, where the other
// six pre-Practice routes answered 404. See requireSelf for why 404 is
// the right one.
func TestFinishEnrollmentHandler_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "unknown-enroller"

	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{UID: identityUID, SecondFactor: true})
	defer srv.Close()

	resp := postFinishEnrollment(t, srv)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusNotFound)
}

func TestFinishEnrollmentHandler_InvalidToken(t *testing.T) {
	db := testdb.New(t)
	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{Err: errBadToken})
	defer srv.Close()

	resp := postFinishEnrollment(t, srv)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusUnauthorized)
}

// TestFinishEnrollmentHandler_EndsPriorSession proves the voluntary
// account-settings entry point (the other of the AC's two required
// paths, alongside a refusal-driven enrollment): a person already holding
// a pre-enrollment session who enrolls mid-session ends up with exactly
// one session, not two, and it is the new one -- "replace, don't leave
// in place".
func TestFinishEnrollmentHandler_EndsPriorSession(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-enrolls-mid-session"
	testdb.SeedStaff(t, db, identityUID)
	priorSession := authntest.SeedSession(t, db.App, identityUID)

	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{UID: identityUID, SecondFactor: true})
	defer srv.Close()

	resp := postFinishEnrollmentWithSession(t, srv, priorSession)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)

	if got := authntest.CountFor(t, db.App, identityUID); got != 1 {
		t.Fatalf("session rows = %d, want 1 (the prior one ended, the new one minted)", got)
	}

	var newToken string
	for _, c := range resp.Cookies() {
		if c.Name == authn.SessionCookieName {
			newToken = c.Value
		}
	}
	if newToken == priorSession {
		t.Fatalf("the new session cookie reuses the prior token, want a freshly minted one")
	}
}
