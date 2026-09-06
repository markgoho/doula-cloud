package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// #816: the three Staff bootstrap paths refuse an unconfirmed mint when
// the caller holds a live portal session, tell the refusal apart by
// APIError.code, and a rollback leaves the signup or invitation still
// claimable so the confirmed retry succeeds. sessionmint's own tests
// prove the ritual works for an arbitrary Result; these three prove each
// handler actually wires authn.TierStaff and its own business step into
// it.

// statusPending is practice_invitations.status's initial value -- shared
// across staffauth_test files, since goconst flags a repeated literal
// package-wide, not just within one file.
const statusPending = "pending"

func readEvictionCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out.Code
}

func addPortalSessionCookie(req *http.Request, token string, confirmed bool) {
	authntest.AddSessionCookie(req, token)
	if confirmed {
		req.Header.Set("X-Confirmed", "true")
	}
}

func TestSignupHandler_LivePortalSessionRefusesThenConfirmedRetrySucceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "signup-with-portal-session"
	portalUID := portalaccount.NewIdentifier()
	portalToken := authntest.SeedSession(t, db.App, portalUID)
	srv := newSignupServer(authntest.Verifier{UID: identityUID, Email: "signup-evicts@example.com"}, db)
	defer srv.Close()

	body, err := json.Marshal(staffauth.SignupRequest{PracticeName: "Evicting Practice", StaffName: "Jamie", WorkState: "NY"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	signupReq := func(confirmed bool) *http.Response {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/signup", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer any-token")
		req.Header.Set("Content-Type", "application/json")
		addPortalSessionCookie(req, portalToken, confirmed)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}

	refused := signupReq(false)
	defer refused.Body.Close()
	if refused.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", refused.StatusCode, http.StatusConflict)
	}
	if got := readEvictionCode(t, refused); got != string(authn.EvictionUnconfirmed) {
		t.Errorf("code = %q, want %q", got, authn.EvictionUnconfirmed)
	}
	var practiceCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM practices WHERE name = 'Evicting Practice'`).Scan(&practiceCount); err != nil {
		t.Fatalf("count practices: %v", err)
	}
	if practiceCount != 0 {
		t.Errorf("practices named 'Evicting Practice' = %d, want 0 -- a refused mint rolls the signup back", practiceCount)
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 1 {
		t.Errorf("portal session rows = %d, want 1 -- a refusal leaves it alone", got)
	}

	confirmed := signupReq(true)
	defer confirmed.Body.Close()
	if confirmed.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", confirmed.StatusCode, http.StatusCreated)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM practices WHERE name = 'Evicting Practice'`).Scan(&practiceCount); err != nil {
		t.Fatalf("count practices: %v", err)
	}
	if practiceCount != 1 {
		t.Errorf("practices named 'Evicting Practice' = %d, want 1 -- the confirmed retry succeeds", practiceCount)
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 0 {
		t.Errorf("portal session rows = %d, want 0 -- confirmed, evicted", got)
	}
	if got := authntest.CountFor(t, db.App, identityUID); got != 1 {
		t.Errorf("staff session rows = %d, want 1", got)
	}
}

func TestAcceptInviteHandler_LivePortalSessionRefusesThenConfirmedRetrySucceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "accept-with-portal-session"
	const address = "invitee-portal-collision@example.com"
	practiceID := seedPractice(t, db, "Inviting Practice")
	inviterID := seedStaff(t, db, "inviter-uid")
	_, inviteToken := seedInvitationWithToken(t, db, practiceID, inviterID, address, "{doula}", "employee", time.Now().Add(time.Hour))
	portalUID := portalaccount.NewIdentifier()
	portalToken := authntest.SeedSession(t, db.App, portalUID)
	srv := newAcceptServer(t, db, identityUID, address)
	defer srv.Close()

	body, err := json.Marshal(staffauth.AcceptInviteRequest{InviteToken: inviteToken, Name: "Invitee", WorkState: "NY"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	acceptReq := func(confirmed bool) *http.Response {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/accept-invite", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer any-token")
		req.Header.Set("Content-Type", "application/json")
		addPortalSessionCookie(req, portalToken, confirmed)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}

	refused := acceptReq(false)
	defer refused.Body.Close()
	if refused.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", refused.StatusCode, http.StatusConflict)
	}
	if got := readEvictionCode(t, refused); got != string(authn.EvictionUnconfirmed) {
		t.Errorf("code = %q, want %q", got, authn.EvictionUnconfirmed)
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM practice_invitations WHERE token_digest = $1`, staffauth.TokenDigest(inviteToken)).Scan(&status); err != nil {
		t.Fatalf("read invitation status: %v", err)
	}
	if status != statusPending {
		t.Errorf("invitation status = %q, want %q -- a refused mint leaves it claimable", status, statusPending)
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 1 {
		t.Errorf("portal session rows = %d, want 1 -- a refusal leaves it alone", got)
	}

	confirmed := acceptReq(true)
	defer confirmed.Body.Close()
	if confirmed.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", confirmed.StatusCode, http.StatusOK)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM practice_invitations WHERE token_digest = $1`, staffauth.TokenDigest(inviteToken)).Scan(&status); err != nil {
		t.Fatalf("read invitation status: %v", err)
	}
	if status != "accepted" {
		t.Errorf("invitation status = %q, want %q -- the confirmed retry succeeds", status, "accepted")
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 0 {
		t.Errorf("portal session rows = %d, want 0 -- confirmed, evicted", got)
	}
}

func TestFinishEnrollmentHandler_LivePortalSessionRefusesThenConfirmedRetrySucceeds(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "enrol-with-portal-session"
	staffID := seedStaff(t, db, identityUID)
	portalUID := portalaccount.NewIdentifier()
	portalToken := authntest.SeedSession(t, db.App, portalUID)
	srv := newFinishEnrollmentServer(t, db, authntest.Verifier{UID: identityUID, SecondFactor: true})
	defer srv.Close()

	enrollReq := func(confirmed bool) *http.Response {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/staff/mfa", nil)
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer tok")
		addPortalSessionCookie(req, portalToken, confirmed)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		return resp
	}

	refused := enrollReq(false)
	defer refused.Body.Close()
	if refused.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", refused.StatusCode, http.StatusConflict)
	}
	if got := readEvictionCode(t, refused); got != string(authn.EvictionUnconfirmed) {
		t.Errorf("code = %q, want %q", got, authn.EvictionUnconfirmed)
	}
	var eventCount int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM staff_auth_events WHERE staff_id = $1`, staffID).Scan(&eventCount); err != nil {
		t.Fatalf("count staff_auth_events: %v", err)
	}
	if eventCount != 0 {
		t.Errorf("staff_auth_events rows = %d, want 0 -- a refused mint records no enrolment", eventCount)
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 1 {
		t.Errorf("portal session rows = %d, want 1 -- a refusal leaves it alone", got)
	}

	confirmed := enrollReq(true)
	defer confirmed.Body.Close()
	if confirmed.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", confirmed.StatusCode, http.StatusOK)
	}
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM staff_auth_events WHERE staff_id = $1`, staffID).Scan(&eventCount); err != nil {
		t.Fatalf("count staff_auth_events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("staff_auth_events rows = %d, want 1 -- the confirmed retry succeeds", eventCount)
	}
	if got := authntest.CountFor(t, db.App, portalUID); got != 0 {
		t.Errorf("portal session rows = %d, want 0 -- confirmed, evicted", got)
	}
}
