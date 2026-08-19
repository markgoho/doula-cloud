package staffauth_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// sessionCookie returns the __session cookie from resp, or nil if none
// was set. Shared across staffauth_test files.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func newAcceptServer(verifier authntest.Verifier, db *testdb.DB) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /staff/accept-invite", staffauth.AcceptInviteHandler(verifier, db.App))
	return httptest.NewServer(mux)
}

func postAccept(t *testing.T, srv *httptest.Server, token string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/accept-invite", bytes.NewReader(payload))
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

// seedPendingInvite seeds a Practice, an Owner, and a pending (unclaimed)
// invite for a second person -- the state InviteHandler leaves behind for
// AcceptInviteHandler to pick up.
func seedPendingInvite(t *testing.T, db *testdb.DB) (staffID, inviteToken string) {
	t.Helper()
	practiceID := seedPractice(t, db, "Accept Test Practice")
	ownerID := seedStaff(t, db, "accept-test-owner")
	seedMembership(t, db, practiceID, ownerID)

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (id, name, email, invite_token) VALUES (gen_random_uuid(), 'Invitee', 'invitee@example.com', gen_random_uuid()) RETURNING id, invite_token::text`,
	).Scan(&staffID, &inviteToken); err != nil {
		t.Fatalf("seed pending invite: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{}')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed pending membership: %v", err)
	}
	return staffID, inviteToken
}

func TestAcceptInviteHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "", staffauth.AcceptInviteRequest{InviteToken: "whatever"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on missing token: %+v", c)
	}
}

func TestAcceptInviteHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{Err: errBadToken}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "bad", staffauth.AcceptInviteRequest{InviteToken: "whatever"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on invalid token: %+v", c)
	}
}

func TestAcceptInviteHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{UID: someUID}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/accept-invite", bytes.NewReader([]byte("not json")))
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

func TestAcceptInviteHandler_MissingInviteToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{UID: someUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: ""})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAcceptInviteHandler_UnknownToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{UID: someUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: "00000000-0000-0000-0000-000000000000"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAcceptInviteHandler_Success(t *testing.T) {
	db := testdb.New(t)
	staffID, inviteToken := seedPendingInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: inviteeIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out staffauth.AcceptInviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.StaffID != staffID {
		t.Fatalf("staffId = %q, want %q", out.StaffID, staffID)
	}

	var identityUID string
	var storedToken sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM staff WHERE id = $1`, staffID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query claimed staff: %v", err)
	}
	if identityUID != inviteeIdentityUID {
		t.Fatalf("identity_uid = %q, want %q", identityUID, inviteeIdentityUID)
	}
	if storedToken.Valid {
		t.Fatalf("expected invite_token cleared after accept, got %q", storedToken.String)
	}

	// #145: accept-invite sets the session cookie on its own response, same
	// name/attributes/lifetime as the create-session endpoint's (#144) --
	// deliberately not asserting on the cookie's value.
	c := sessionCookie(resp)
	if c == nil {
		t.Fatal("no __session cookie set on successful accept")
	}
	if c.Value == "" {
		t.Fatal("cookie value is empty")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("cookie attributes = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/", c)
	}
	if wantMaxAge := int(session.Lifetime.Seconds()); c.MaxAge != wantMaxAge {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, wantMaxAge)
	}
}

// TestAcceptInviteHandler_SessionStoreFailure proves an accept that
// cannot create its session commits nothing: no cookie, and the invite
// left exactly as pending as it was before the request, so retrying it
// is safe (#145). Creating the session is the last thing the handler
// does before committing, which is what makes that ordering hold.
func TestAcceptInviteHandler_SessionStoreFailure(t *testing.T) {
	db := testdb.New(t)
	staffID, inviteToken := seedPendingInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: inviteeIdentityUID}, db)
	defer srv.Close()
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on session store failure: %+v", c)
	}

	var identityUID sql.NullString
	var storedToken string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM staff WHERE id = $1`, staffID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query staff row: %v", err)
	}
	if identityUID.Valid {
		t.Fatalf("expected identity_uid to remain unset after rollback, got %q", identityUID.String)
	}
	if storedToken != inviteToken {
		t.Fatalf("expected invite_token to remain %q after rollback, got %q", inviteToken, storedToken)
	}
}

func TestAcceptInviteHandler_TokenAlreadyClaimed(t *testing.T) {
	db := testdb.New(t)
	_, inviteToken := seedPendingInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: inviteeIdentityUID}, db)
	defer srv.Close()

	first := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: inviteToken})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first accept status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: inviteToken})
	defer second.Body.Close()
	if second.StatusCode != http.StatusNotFound {
		t.Fatalf("second accept status = %d, want %d", second.StatusCode, http.StatusNotFound)
	}
}

// TestAcceptInviteHandler_IdentityAlreadyClaimedElsewhere proves the
// already-active-staff edge case (someone accepting a second invite with
// an identity_uid that's already claimed by their own first account)
// fails cleanly with 409, not a 500 or a corrupted row.
func TestAcceptInviteHandler_IdentityAlreadyClaimedElsewhere(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "already-active-identity"
	seedStaffWithMembership(t, db, identityUID)
	_, inviteToken := seedPendingInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: identityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", staffauth.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
