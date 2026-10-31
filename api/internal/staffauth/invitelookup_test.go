package staffauth_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func postLookup(t *testing.T, srv *httptest.Server, token string) *http.Response {
	t.Helper()
	payload, err := json.Marshal(staffauth.LookupInvitationRequest{InviteToken: token})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/accept-invite/lookup", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func decodeLookup(t *testing.T, resp *http.Response) staffauth.LookupInvitationResponse {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got staffauth.LookupInvitationResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

// TestLookupInvitation_NamesThePracticeTheAddressAndTheRoles is #2238: the
// accept screen shows what she is joining before she types anything, and
// knows whether the invited address already signs in.
func TestLookupInvitation_NamesThePracticeTheAddressAndTheRoles(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-whose-invite-is-read")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "Dee@Example.com", "{admin,doula}", employeeType, time.Now().Add(time.Hour))
	var practiceName string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&practiceName); err != nil {
		t.Fatalf("read practice name: %v", err)
	}

	srv := newAcceptServerWithAccounts(t, db, "", "", authntest.NewFakeAccountManager())
	defer srv.Close()

	resp := postLookup(t, srv, token)
	defer resp.Body.Close()
	got := decodeLookup(t, resp)
	if got.PracticeName != practiceName || got.Address != "dee@example.com" || got.EmploymentType != employeeType || got.HasAccount {
		t.Fatalf("lookup = %+v", got)
	}
	if len(got.Roles) != 2 || got.Roles[0] != adminRole || got.Roles[1] != doulaRole {
		t.Fatalf("roles = %v, want [admin doula]", got.Roles)
	}
}

func TestLookupInvitation_SaysWhenTheAddressAlreadySignsIn(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-inviting-a-returning-person")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "lena@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	accounts := authntest.NewFakeAccountManager()
	accounts.Seed("lena-uid", "lena@example.com", true)
	srv := newAcceptServerWithAccounts(t, db, "", "", accounts)
	defer srv.Close()

	resp := postLookup(t, srv, token)
	defer resp.Body.Close()
	if got := decodeLookup(t, resp); !got.HasAccount {
		t.Fatalf("hasAccount = false, want true for an address Identity Platform holds")
	}
}

func TestLookupInvitation_Refusals(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-whose-invites-are-unusable")
	expiredID, expired := seedInvitationWithToken(t, db, practiceID, ownerID, "late@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))
	revokedID, revoked := seedInvitationWithToken(t, db, practiceID, ownerID, "gone@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`, revokedID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	srv := newAcceptServerWithAccounts(t, db, "", "", authntest.NewFakeAccountManager())
	defer srv.Close()

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{"no token", "  ", http.StatusBadRequest},
		{"unknown", "no-such-token", http.StatusNotFound},
		{"revoked", revoked, http.StatusNotFound},
		{"expired", expired, http.StatusGone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := postLookup(t, srv, tc.token)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}

	// A read writes nothing: the accept path is what marks it expired.
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status::text FROM practice_invitations WHERE id = $1`, expiredID).Scan(&status); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if status != statusPending {
		t.Fatalf("status = %q, want pending", status)
	}
}

func TestLookupInvitation_AccountLookupFailureIs500(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-when-identity-platform-fails")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "someone@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	accounts := authntest.NewFakeAccountManager()
	accounts.Err = errors.New("identity platform down")
	srv := newAcceptServerWithAccounts(t, db, "", "", accounts)
	defer srv.Close()

	resp := postLookup(t, srv, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestLookupInvitation_BadJSON(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServerWithAccounts(t, db, "", "", authntest.NewFakeAccountManager())
	defer srv.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/accept-invite/lookup", bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
