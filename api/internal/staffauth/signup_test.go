package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	testStaffEmail = "s@example.com"
	jamieEmail     = "jamie@example.com"

	// Shared across staffauth_test files: goconst flags repeated literals
	// package-wide, not just within one file.
	someUID            = "some-uid"
	inviteeIdentityUID = "invitee-identity"
	inviteeName        = "Invitee"
	ownerRole          = "owner"
	doulaRole          = "doula"
)

func newSignupServer(verifier fakeVerifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /staff/signup", staffauth.SignupHandler(verifier, db.App))
	return httptest.NewServer(mux)
}

func postSignup(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/signup", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestSignupHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "", staffauth.SignupRequest{PracticeName: "P", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSignupHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "bad-token", staffauth.SignupRequest{PracticeName: "P", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestSignupHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "bad-body-uid"}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/signup", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignupHandler_MissingFields(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "new-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{PracticeName: "", StaffName: "S", StaffEmail: testStaffEmail})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignupHandler_Success(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "new-owner"}, db)
	defer srv.Close()

	resp := postSignup(t, srv, "tok", staffauth.SignupRequest{
		PracticeName: "Solo Doula Co",
		StaffName:    "Jamie Owner",
		StaffEmail:   jamieEmail,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var out staffauth.SignupResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID == "" || out.PracticeID == "" {
		t.Fatalf("expected non-empty ids, got %+v", out)
	}

	var roleCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_length(roles, 1) FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		out.PracticeID, out.StaffID,
	).Scan(&roleCount); err != nil {
		t.Fatalf("query membership roles: %v", err)
	}
	if roleCount != 3 {
		t.Fatalf("expected 3 roles (owner, office_manager, doula), got %d", roleCount)
	}
}

func TestSignupHandler_DuplicateSignup(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(fakeVerifier{uid: "repeat-owner"}, db)
	defer srv.Close()

	body := staffauth.SignupRequest{PracticeName: "First Practice", StaffName: "Jamie", StaffEmail: jamieEmail}
	first := postSignup(t, srv, "tok", body)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first signup status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	second := postSignup(t, srv, "tok", staffauth.SignupRequest{PracticeName: "Second Practice", StaffName: "Jamie", StaffEmail: jamieEmail})
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second signup status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}
