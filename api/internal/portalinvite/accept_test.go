package portalinvite_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/testdb"
)

const acceptIdentityUID = "portal-invite-accepting-uid"

var errMintFail = errors.New("mint failed")

// sessionCookie returns the __session cookie from resp, or nil if none
// was set.
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

func TestAcceptInviteHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "", portalinvite.AcceptInviteRequest{InviteToken: "whatever"})
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

	resp := postAccept(t, srv, "bad", portalinvite.AcceptInviteRequest{InviteToken: "whatever"})
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
	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/portal/accept-invite", bytes.NewReader([]byte("not json")))
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
	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: ""})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAcceptInviteHandler_UnknownToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: "00000000-0000-0000-0000-000000000000"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAcceptInviteHandler_Success(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out portalinvite.AcceptInviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ClientID != clientID {
		t.Fatalf("clientId = %q, want %q", out.ClientID, clientID)
	}

	var identityUID string
	var storedToken sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM client_portal_users WHERE client_id = $1`, clientID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query claimed row: %v", err)
	}
	if identityUID != acceptIdentityUID {
		t.Fatalf("identity_uid = %q, want %q", identityUID, acceptIdentityUID)
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

// TestAcceptInviteHandler_MintFailure proves a failed accept-invite request
// -- here, the session cookie failing to mint -- sets no cookie and rolls
// back the claim: the invite is left exactly as pending as it was before
// the request, so retrying it is safe (#145).
func TestAcceptInviteHandler_MintFailure(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID, MintErr: errMintFail}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on mint failure: %+v", c)
	}

	var identityUID sql.NullString
	var storedToken string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM client_portal_users WHERE client_id = $1`, clientID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query portal user row: %v", err)
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
	_, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	first := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first accept status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer second.Body.Close()
	if second.StatusCode != http.StatusNotFound {
		t.Fatalf("second accept status = %d, want %d", second.StatusCode, http.StatusNotFound)
	}
}

// TestAcceptInviteHandler_IdentityAlreadyClaimedElsewhere proves the
// already-linked-identity edge case (someone accepting a second invite
// with an identity_uid that already has a client_portal_users row) fails
// cleanly with 409, not a 500 or a corrupted row.
func TestAcceptInviteHandler_IdentityAlreadyClaimedElsewhere(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "portal-invite-other-owner")
	otherClientID, _ := seedClientEngagement(t, db, practiceID, "Other Client", "other@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		acceptIdentityUID, otherClientID,
	); err != nil {
		t.Fatalf("seed already-linked portal user: %v", err)
	}
	_, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(authntest.Verifier{UID: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
