package portalinvite_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/testdb"
)

const acceptIdentityUID = "portal-invite-accepting-uid"

func TestAcceptInviteHandler_MissingToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(fakeVerifier{}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "", portalinvite.AcceptInviteRequest{InviteToken: "whatever"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAcceptInviteHandler_TokenVerificationFailure(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(fakeVerifier{err: errBadToken}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "bad", portalinvite.AcceptInviteRequest{InviteToken: "whatever"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAcceptInviteHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
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
	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: ""})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAcceptInviteHandler_UnknownToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
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

	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
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
}

func TestAcceptInviteHandler_TokenAlreadyClaimed(t *testing.T) {
	db := testdb.New(t)
	_, inviteToken := seedPendingPortalInvite(t, db)

	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
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

	srv := newAcceptServer(fakeVerifier{uid: acceptIdentityUID}, db)
	defer srv.Close()

	resp := postAccept(t, srv, "tok", portalinvite.AcceptInviteRequest{InviteToken: inviteToken})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
