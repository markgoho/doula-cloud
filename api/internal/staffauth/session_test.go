package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func newSessionServer(verifier fakeVerifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /staff/session", staffauth.SessionHandler(verifier, db.App))
	return httptest.NewServer(mux)
}

func getSession(t *testing.T, srv *httptest.Server, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/staff/session", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestSessionHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newSessionServer(fakeVerifier{}, db)
	defer srv.Close()

	resp := getSession(t, srv, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSessionHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newSessionServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	resp := getSession(t, srv, "bad-token")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSessionHandler_UnknownStaff(t *testing.T) {
	db := testdb.New(t)
	srv := newSessionServer(fakeVerifier{uid: "no-such-staff"}, db)
	defer srv.Close()

	resp := getSession(t, srv, "tok")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSessionHandler_SingleMembership(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "single-practice-staff"
	staffID, practiceID := seedStaffWithMembership(t, db, identityUID)

	srv := newSessionServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := getSession(t, srv, "tok")
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
	if len(out.Memberships) != 1 || out.Memberships[0].PracticeID != practiceID {
		t.Fatalf("memberships = %+v, want single membership at %q", out.Memberships, practiceID)
	}
	if len(out.Memberships[0].Roles) != 1 || out.Memberships[0].Roles[0] != "doula" {
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
	if _, err := db.Admin.Exec(`UPDATE staff SET last_practice_id = $1 WHERE id = $2`, practiceB, staffID); err != nil {
		t.Fatalf("seed last_practice_id: %v", err)
	}

	srv := newSessionServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := getSession(t, srv, "tok")
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
