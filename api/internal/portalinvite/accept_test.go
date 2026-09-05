package portalinvite_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/session"
	"doula-cloud/api/internal/testdb"
)

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

func TestAcceptInviteHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(db)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/portal/accept-invite", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
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
	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: ""})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAcceptInviteHandler_UnknownToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: "00000000-0000-0000-0000-000000000000"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestAcceptInviteHandler_ExpiredInvite proves #616's AC: an invitation
// past its invite_token_expires_at is refused with 410, and the pending
// row is left untouched (the doula re-sends rather than this handler
// flipping any state -- client_portal_users has no status column to
// flip).
func TestAcceptInviteHandler_ExpiredInvite(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInviteExpiringAt(t, db, time.Now().Add(-time.Minute))

	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGone {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusGone)
	}

	var identityUID sql.NullString
	var storedToken string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT identity_uid, invite_token::text FROM client_portal_users WHERE client_id = $1`, clientID).Scan(&identityUID, &storedToken); err != nil {
		t.Fatalf("query portal user row: %v", err)
	}
	if identityUID.Valid {
		t.Fatalf("expected identity_uid to remain unset for an expired invite, got %q", identityUID.String)
	}
	if storedToken != inviteToken {
		t.Fatalf("expected invite_token to remain %q, got %q", inviteToken, storedToken)
	}
}

func TestAcceptInviteHandler_Success(t *testing.T) {
	db := testdb.New(t)
	clientID, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
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
	// identity_uid is a Portal Account Doula Cloud mints itself (#616): with
	// #617 landed, there is no caller-presented Identity Platform uid left
	// to compare it against at all.
	if !strings.HasPrefix(identityUID, portalaccount.Prefix) {
		t.Fatalf("identity_uid = %q, want prefix %q", identityUID, portalaccount.Prefix)
	}
	if storedToken.Valid {
		t.Fatalf("expected invite_token cleared after accept, got %q", storedToken.String)
	}

	var signInAddress string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, identityUID).Scan(&signInAddress); err != nil {
		t.Fatalf("query portal account: %v", err)
	}
	if signInAddress != "invited@example.com" {
		t.Fatalf("sign_in_address = %q, want the invited Client's own contact address", signInAddress)
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
	clientID, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(db)
	defer srv.Close()
	if _, err := db.Admin.ExecContext(t.Context(), `DROP TABLE sessions`); err != nil {
		t.Fatalf("drop sessions: %v", err)
	}

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if c := sessionCookie(resp); c != nil {
		t.Fatalf("cookie set on session store failure: %+v", c)
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

	srv := newAcceptServer(db)
	defer srv.Close()

	first := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first accept status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer second.Body.Close()
	if second.StatusCode != http.StatusNotFound {
		t.Fatalf("second accept status = %d, want %d", second.StatusCode, http.StatusNotFound)
	}
}

// TestAcceptInviteHandler_SignInAddressAlreadyClaimed proves #309's
// scenario in its new shape (see #616): a person who already holds a
// Portal Account for a given sign-in address cannot get a second one for
// the same address by accepting another invite -- caught on
// portal_accounts.sign_in_address's unique index now that identity_uid is
// a fresh identifier every time, never on identity_uid itself. Cased
// differently to prove the index is case-insensitive, matching
// practice_invitations_one_pending's shape.
func TestAcceptInviteHandler_SignInAddressAlreadyClaimed(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_existing-account", "Invited@Example.com")
	_, inviteToken := seedPendingPortalInvite(t, db) // invites "invited@example.com"

	srv := newAcceptServer(db)
	defer srv.Close()

	resp := postAccept(t, srv, portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
