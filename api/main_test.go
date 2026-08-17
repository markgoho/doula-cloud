package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

type fakeVerifier struct {
	uid string
	err error
}

func (f fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*authn.VerifiedToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &authn.VerifiedToken{UID: f.uid}, nil
}

func TestHelloHandler(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()

	helloHandler(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := rec.Body.String(); got != `{"message":"hello world"}`+"\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestResolvePort(t *testing.T) {
	t.Setenv("PORT", "")
	if got := resolvePort(); got != "8080" {
		t.Fatalf("resolvePort() = %q, want 8080", got)
	}

	t.Setenv("PORT", "9090")
	if got := resolvePort(); got != "9090" {
		t.Fatalf("resolvePort() = %q, want 9090", got)
	}
}

func TestRoutes_MissingTokenPaths(t *testing.T) {
	mux := routes(fakeVerifier{}, nil, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/staff/signup"},
		{http.MethodGet, "/api/staff/session"},
		{http.MethodGet, "/api/practices/00000000-0000-0000-0000-000000000000/session"},
	}
	for _, c := range cases {
		req, err := http.NewRequestWithContext(t.Context(), c.method, srv.URL+c.path, nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %s %s: %v", c.method, c.path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want %d", c.method, c.path, resp.StatusCode, http.StatusUnauthorized)
		}
	}
}

// TestRoutes_SignupLoginLanding walks the full ticket flow through the
// real route table: sign up a new Practice, fetch the session to find
// where to land, then hit the practice-scoped landing route and confirm
// it recorded last_practice_id.
func TestRoutes_SignupLoginLanding(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "e2e-owner-uid"
	mux := routes(fakeVerifier{uid: identityUID}, db.App, objectstore.NewMemoryStore(), push.NewFakePusher(), billing.NewFakeStripeClient(), "whsec_test")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	signupBody, _ := json.Marshal(staffauth.SignupRequest{
		PracticeName: "Riverside Doulas",
		StaffName:    "Jamie Owner",
		StaffEmail:   "jamie@example.com",
	})
	signupReq, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", bytes.NewReader(signupBody))
	signupReq.Header.Set("Authorization", "Bearer tok")
	signupResp, err := http.DefaultClient.Do(signupReq)
	if err != nil {
		t.Fatalf("signup request: %v", err)
	}
	defer signupResp.Body.Close()
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup status = %d, want %d", signupResp.StatusCode, http.StatusCreated)
	}
	var signedUp staffauth.SignupResponse
	if err := json.NewDecoder(signupResp.Body).Decode(&signedUp); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	sessionReq, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/staff/session", nil)
	sessionReq.Header.Set("Authorization", "Bearer tok")
	sessionResp, err := http.DefaultClient.Do(sessionReq)
	if err != nil {
		t.Fatalf("session request: %v", err)
	}
	defer sessionResp.Body.Close()
	var session staffauth.SessionResponse
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if len(session.Memberships) != 1 || session.Memberships[0].PracticeID != signedUp.PracticeID {
		t.Fatalf("memberships = %+v, want single membership at %q", session.Memberships, signedUp.PracticeID)
	}

	landingReq, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+signedUp.PracticeID+"/session", nil)
	landingReq.Header.Set("Authorization", "Bearer tok")
	landingResp, err := http.DefaultClient.Do(landingReq)
	if err != nil {
		t.Fatalf("landing request: %v", err)
	}
	defer landingResp.Body.Close()
	if landingResp.StatusCode != http.StatusOK {
		t.Fatalf("landing status = %d, want %d", landingResp.StatusCode, http.StatusOK)
	}
	var landing practiceSessionResponse
	if err := json.NewDecoder(landingResp.Body).Decode(&landing); err != nil {
		t.Fatalf("decode landing response: %v", err)
	}
	if landing.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", landing.PracticeName, "Riverside Doulas")
	}

	var lastPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT last_practice_id FROM staff WHERE id = $1`, signedUp.StaffID).Scan(&lastPracticeID); err != nil {
		t.Fatalf("query last_practice_id: %v", err)
	}
	if lastPracticeID != signedUp.PracticeID {
		t.Fatalf("last_practice_id = %q, want %q", lastPracticeID, signedUp.PracticeID)
	}
}
