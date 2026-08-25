package staffauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// recordingEnqueuer stands in for the Cloud Tasks nudge, so a test can
// assert the invite registered one without a queue existing.
type recordingEnqueuer struct{ fired []tasknudge.OutboxType }

func (e *recordingEnqueuer) Enqueue(_ context.Context, kind tasknudge.OutboxType) error {
	e.fired = append(e.fired, kind)
	return nil
}

func newInviteServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string, enq *recordingEnqueuer) {
	t.Helper()
	enq = &recordingEnqueuer{}
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/staff/invitations",
		staffauth.Middleware(db.App)(staffauth.InviteHandler(enq)))
	mux.Handle("POST /practices/{practiceId}/staff/invitations/{invitationId}/revoke",
		staffauth.Middleware(db.App)(staffauth.RevokeInvitationHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid), enq
}

func postInvite(t *testing.T, srv *httptest.Server, session, practiceID string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return doInviteRequest(t, srv, session, "/practices/"+practiceID+"/staff/invitations", payload)
}

func doInviteRequest(t *testing.T, srv *httptest.Server, session, path string, payload []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestInviteHandler_CreatesInvitationAndNoStaffRow is the ticket's
// headline claim (#316, #291): inviting somebody produces an Invitation
// and nothing else. The staff row that used to be created here is what
// left a member behind who could never sign in.
func TestInviteHandler_CreatesInvitationAndNoStaffRow(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-invites"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session, enq := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
		Email: "  Lena@Example.com ", Roles: []string{doulaRole}, EmploymentType: contractorType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created staffauth.InviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.InvitationID == "" || created.ExpiresAt == "" {
		t.Fatalf("response = %+v, want an invitation id and expiry", created)
	}

	var address, roles, employmentType, digest, invitedBy string
	var expiresAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT address, array_to_string(roles, ','), employment_type::text, token_digest, invited_by, expires_at
		 FROM practice_invitations WHERE id = $1`, created.InvitationID,
	).Scan(&address, &roles, &employmentType, &digest, &invitedBy, &expiresAt); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if address != "lena@example.com" {
		t.Fatalf("address = %q, want it normalized", address)
	}
	if roles != doulaRole || employmentType != contractorType {
		t.Fatalf("roles/employmentType = %q/%q, want the invited membership", roles, employmentType)
	}
	if invitedBy != ownerID {
		t.Fatalf("invitedBy = %q, want the Owner %q", invitedBy, ownerID)
	}
	if expiresAt.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatalf("expiresAt = %v, want roughly 7 days out", expiresAt)
	}

	// The mailable token lives only in the outbox row; the Invitation
	// keeps its digest, and the Owner's response keeps neither.
	var token string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT invite_token::text FROM staff_invite_outbox WHERE invitation_id = $1`, created.InvitationID,
	).Scan(&token); err != nil {
		t.Fatalf("read queued outbox row: %v", err)
	}
	if staffauth.TokenDigest(token) != digest {
		t.Fatalf("stored digest does not match the queued token")
	}

	var staffCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff WHERE lower(email) = 'lena@example.com'`,
	).Scan(&staffCount); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffCount != 0 {
		t.Fatalf("staff rows for the invitee = %d, want 0", staffCount)
	}

	if len(enq.fired) != 1 || enq.fired[0] != tasknudge.StaffInvite {
		t.Fatalf("nudges fired = %v, want one staff-invite", enq.fired)
	}
}

// TestInviteHandler_ReInviteRotatesTheSameInvitation pins that a second
// invite to one address rotates rather than adding a second pending row
// -- the invitation-era rerun of the duplicate-row shape #291 found.
func TestInviteHandler_ReInviteRotatesTheSameInvitation(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reinvites"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session, _ := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	first := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
		Email: "again@example.com", Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer first.Body.Close()
	var created staffauth.InviteResponse
	if err := json.NewDecoder(first.Body).Decode(&created); err != nil {
		t.Fatalf("decode first: %v", err)
	}

	second := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
		Email: "AGAIN@example.com", Roles: []string{adminRole}, EmploymentType: contractorType,
	})
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("re-invite status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	var rotated staffauth.InviteResponse
	if err := json.NewDecoder(second.Body).Decode(&rotated); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if rotated.InvitationID != created.InvitationID {
		t.Fatalf("re-invite created a second Invitation (%q vs %q)", rotated.InvitationID, created.InvitationID)
	}

	var pending int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_invitations WHERE practice_id = $1 AND status = 'pending'`, practiceID,
	).Scan(&pending); err != nil {
		t.Fatalf("count pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending invitations = %d, want 1", pending)
	}

	// The rotated row carries the roles typed the second time, not the
	// first: re-inviting is re-deciding the Membership, not resending.
	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_invitations WHERE id = $1`,
		created.InvitationID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read rotated invitation: %v", err)
	}
	if roles != adminRole || employmentType != contractorType {
		t.Fatalf("rotated membership = %q/%q, want the second invite's", roles, employmentType)
	}
}

func TestInviteHandler_AlreadyAMemberConflicts(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-invites-a-member"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)
	memberID := seedStaffWithEmail(t, db, "already-here", "here@example.com")
	seedMembership(t, db, practiceID, memberID)

	srv, session, _ := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
		Email: "HERE@example.com", Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

func TestInviteHandler_Rejects(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-invite-validation"
	_, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session, _ := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	// One well-formed address, reused so each case fails for the one
	// reason it names.
	const validAddress = "a@example.com"
	cases := []struct {
		name string
		body staffauth.InviteRequest
	}{
		{"no address", staffauth.InviteRequest{Roles: []string{doulaRole}, EmploymentType: employeeType}},
		{"no roles", staffauth.InviteRequest{Email: validAddress, EmploymentType: employeeType}},
		{"unknown role", staffauth.InviteRequest{Email: validAddress, Roles: []string{"superuser"}, EmploymentType: employeeType}},
		{"unknown employment type", staffauth.InviteRequest{Email: validAddress, Roles: []string{doulaRole}, EmploymentType: "volunteer"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postInvite(t, srv, session, practiceID, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}

	t.Run("invalid body", func(t *testing.T) {
		resp := doInviteRequest(t, srv, session, "/practices/"+practiceID+"/staff/invitations", []byte("not json"))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestInviteHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-invites"
	_, practiceID := seedStaffWithMembership(t, db, doulaUID) // '{doula}'

	srv, session, _ := newInviteServer(t, db, doulaUID)
	defer srv.Close()

	resp := postInvite(t, srv, session, practiceID, staffauth.InviteRequest{
		Email: "nope@example.com", Roles: []string{doulaRole}, EmploymentType: employeeType,
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestRevokeInvitationHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-revokes"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)
	invitationID := seedInvitation(t, db, practiceID, ownerID, "revoke-me@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	srv, session, _ := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	resp := doInviteRequest(t, srv, session,
		"/practices/"+practiceID+"/staff/invitations/"+invitationID+"/revoke", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	var status, revokedBy string
	var revokedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, revoked_by, revoked_at FROM practice_invitations WHERE id = $1`, invitationID,
	).Scan(&status, &revokedBy, &revokedAt); err != nil {
		t.Fatalf("read revoked invitation: %v", err)
	}
	if status != "revoked" || revokedBy != ownerID || revokedAt.IsZero() {
		t.Fatalf("revoked row = %q/%q/%v, want the actor and moment recorded", status, revokedBy, revokedAt)
	}
}

func TestRevokeInvitationHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-revokes-nothing"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	srv, session, _ := newInviteServer(t, db, ownerUID)
	defer srv.Close()

	t.Run("no such invitation", func(t *testing.T) {
		resp := doInviteRequest(t, srv, session,
			"/practices/"+practiceID+"/staff/invitations/"+emptyUUID+"/revoke", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("malformed id", func(t *testing.T) {
		resp := doInviteRequest(t, srv, session,
			"/practices/"+practiceID+"/staff/invitations/not-a-uuid/revoke", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("already revoked", func(t *testing.T) {
		invitationID := seedInvitation(t, db, practiceID, ownerID, "twice@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))
		if _, err := db.Admin.ExecContext(t.Context(),
			`UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`, invitationID,
		); err != nil {
			t.Fatalf("pre-revoke: %v", err)
		}
		resp := doInviteRequest(t, srv, session,
			"/practices/"+practiceID+"/staff/invitations/"+invitationID+"/revoke", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})
}

func TestRevokeInvitationHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const doulaUID = "doula-revokes"
	staffID, practiceID := seedStaffWithMembership(t, db, doulaUID)
	invitationID := seedInvitation(t, db, practiceID, staffID, "safe@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	srv, session, _ := newInviteServer(t, db, doulaUID)
	defer srv.Close()

	resp := doInviteRequest(t, srv, session,
		"/practices/"+practiceID+"/staff/invitations/"+invitationID+"/revoke", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
