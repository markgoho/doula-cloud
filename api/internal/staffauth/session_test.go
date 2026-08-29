package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newSessionServer mounts the Staff session route and seeds a live
// session for uid, returning the token its __session cookie carries.
func newSessionServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /staff/session", staffauth.SessionHandler(db.App))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getSession(t *testing.T, srv *httptest.Server, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/staff/session", nil)
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
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID)

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

func TestSessionHandler_MultiplePracticesWithLastUsed(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "multi-practice-staff"
	staffID := seedStaff(t, db, identityUID)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
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
