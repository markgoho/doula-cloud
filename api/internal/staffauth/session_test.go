package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// newSessionServer mounts this package's whole surface through
// staffauth.Mount, and seeds a live session for uid, returning the token
// its __session cookie carries.
func newSessionServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/staff/session", nil)
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

func TestSessionHandler_MissingCookie(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "no-cookie-sent")
	defer srv.Close()

	resp := getSession(t, srv, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestSessionHandler_UnknownSession covers a cookie that names no live
// session -- the shape a stale or forged cookie arrives in.
func TestSessionHandler_UnknownSession(t *testing.T) {
	db := testdb.New(t)
	srv, _ := newSessionServer(t, db, "irrelevant")
	defer srv.Close()

	resp := getSession(t, srv, "never-issued")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSessionHandler_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv, session := newSessionServer(t, db, "no-such-staff")
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSessionHandler_SingleMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "single-practice-staff"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out staffauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID != staffID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, staffID)
	}
	// The shell's avatar menu shows which account a person is signed in as
	// (#452), which matters because one account holds Memberships at
	// several Practices.
	if wantEmail := identityUID + "@example.com"; out.Email != wantEmail {
		t.Fatalf("email = %q, want %q", out.Email, wantEmail)
	}
	if len(out.Memberships) != 1 || out.Memberships[0].PracticeID != practiceID {
		t.Fatalf("memberships = %+v, want single membership at %q", out.Memberships, practiceID)
	}
	if len(out.Memberships[0].Roles) != 1 || out.Memberships[0].Roles[0] != doulaRole {
		t.Fatalf("roles = %v, want [doula]", out.Memberships[0].Roles)
	}
	if out.LastPracticeID != nil {
		t.Fatalf("lastPracticeId = %v, want nil (never set)", *out.LastPracticeID)
	}
}

// TestSessionHandler_SecondFactor proves the response carries this
// session's own second-factor fact (#606) -- the account screen's
// "Enrol" vs "Remove" branch reads it straight from here, not from a
// fresh Identity Platform lookup.
func TestSessionHandler_SecondFactor(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "session-second-factor"
	testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	unenrolled := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, false)
	resp1 := getSession(t, srv, unenrolled)
	defer resp1.Body.Close()
	var out1 staffauth.SessionResponse
	if err := json.NewDecoder(resp1.Body).Decode(&out1); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out1.SecondFactor {
		t.Fatalf("secondFactor = true, want false")
	}

	enrolled := authntest.SeedSessionWithSecondFactor(t, db.App, identityUID, true)
	resp2 := getSession(t, srv, enrolled)
	defer resp2.Body.Close()
	var out2 staffauth.SessionResponse
	if err := json.NewDecoder(resp2.Body).Decode(&out2); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out2.SecondFactor {
		t.Fatalf("secondFactor = false, want true")
	}
}

func TestSessionHandler_MultiplePracticesWithLastUsed(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "multi-practice-staff"
	staffID := testdb.SeedStaff(t, db, identityUID)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	seedMembership(t, db, practiceA, staffID)
	seedMembership(t, db, practiceB, staffID)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE staff SET last_practice_id = $1 WHERE id = $2`, practiceB, staffID); err != nil {
		t.Fatalf("seed last_practice_id: %v", err)
	}

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	resp := getSession(t, srv, session)
	defer resp.Body.Close()

	var out staffauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Memberships) != 2 {
		t.Fatalf("expected 2 memberships, got %d", len(out.Memberships))
	}
	if out.LastPracticeID == nil || *out.LastPracticeID != practiceB {
		t.Fatalf("lastPracticeId = %v, want %q", out.LastPracticeID, practiceB)
	}
}

// TestSessionHandler_SoleOwner is the two-Owner fixture #694's account
// screen turns on: a sole Owner is offered saved recovery codes, a
// co-Owner is offered none.
//
// It is here rather than beside the rotate endpoint because of what it
// is really guarding. The "is there another Owner?" half of the question
// reads Membership rows belonging to *other people*, and this endpoint's
// transaction sets only app.current_identity_uid -- so under
// practice_memberships_self_visibility that subquery sees nothing, finds
// no other Owner, and answers "sole" for everyone. The failure is
// silent: no error, no 403, just a co-Owner shown a section whose button
// then 403s. The second half of this test, with a second Owner seeded at
// the same Practice, is the assertion that catches it.
func TestSessionHandler_SoleOwner(t *testing.T) {
	db := testdb.New(t)
	const soleUID = "session-sole-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, soleUID, []string{ownerRole}, employeeType)

	srv, soleSession := newSessionServer(t, db, soleUID)
	defer srv.Close()

	if got := decodeSession(t, srv, soleSession); !got.SoleOwner {
		t.Fatalf("soleOwner = false for the only Owner of a practice, want true")
	}

	// A second Owner at the same Practice. Neither of them is sole now,
	// and neither can see the other's Membership row through RLS -- which
	// is the whole point of the assertions below.
	const secondUID = "session-second-owner"
	secondStaffID := testdb.SeedStaff(t, db, secondUID)
	seedMembershipWithRoles(t, db, practiceID, secondStaffID, "{owner}")

	if got := decodeSession(t, srv, soleSession); got.SoleOwner {
		t.Fatalf("soleOwner = true once a second Owner holds a membership at the same practice, want false")
	}

	secondSession := authntest.SeedSession(t, db.App, secondUID)
	if got := decodeSession(t, srv, secondSession); got.SoleOwner {
		t.Fatalf("soleOwner = true for the second Owner, want false")
	}
}

// TestSessionHandler_SoleOwnerFalseForNonOwner covers the population
// #615's AC names last: somebody who is not an Owner anywhere holds no
// saved codes, and the account screen must not imply she does.
func TestSessionHandler_SoleOwnerFalseForNonOwner(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "session-doula-not-owner"
	testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{doulaRole}, employeeType)

	srv, session := newSessionServer(t, db, identityUID)
	defer srv.Close()

	if got := decodeSession(t, srv, session); got.SoleOwner {
		t.Fatalf("soleOwner = true for a Doula, want false")
	}
}

// decodeSession reads one GET /api/staff/session into its DTO, failing
// the test on any status but 200.
func decodeSession(t *testing.T, srv *httptest.Server, session string) staffauth.SessionResponse {
	t.Helper()
	resp := getSession(t, srv, session)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out staffauth.SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
