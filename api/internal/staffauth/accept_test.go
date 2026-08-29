package staffauth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// seedInvitationWithToken seeds a pending Invitation and returns the
// plaintext token to accept it with -- the token only ever exists in the
// outbox row in production, so a test mints its own and stores the
// matching digest.
func seedInvitationWithToken(t *testing.T, db *testdb.DB, practiceID, invitedBy, address, roles, employmentType string, expiresAt time.Time) (invitationID, token string) {
	t.Helper()
	token = "token-for-" + address
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type, $5, $6, $7) RETURNING id`,
		practiceID, address, roles, employmentType, staffauth.TokenDigest(token), invitedBy, expiresAt,
	).Scan(&invitationID); err != nil {
		t.Fatalf("seed invitation for %q: %v", address, err)
	}
	return invitationID, token
}

func postAccept(t *testing.T, srv *httptest.Server, body staffauth.AcceptInviteRequest) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return postAcceptRaw(t, srv, payload)
}

func postAcceptRaw(t *testing.T, srv *httptest.Server, payload []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/accept-invite", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer any-token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func newAcceptServer(t *testing.T, db *testdb.DB, uid, email string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /staff/accept-invite",
		staffauth.AcceptInviteHandler(authntest.Verifier{UID: uid, Email: email}, db.App))
	return httptest.NewServer(mux)
}

// TestAcceptInviteHandler_CreatesStaffAndMembership is the accept half of
// #226: the staff row appears here, when someone proves she controls the
// invited address -- not at invite time.
func TestAcceptInviteHandler_CreatesStaffAndMembership(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-whose-invite-is-accepted")
	invitationID, token := seedInvitationWithToken(t, db, practiceID, ownerID, "lena@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "lena-uid", "Lena@Example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Lena Vasquez"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var accepted staffauth.AcceptInviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if accepted.PracticeID != practiceID {
		t.Fatalf("practiceId = %q, want %q", accepted.PracticeID, practiceID)
	}

	// The session cookie is minted on the accept response itself (#145),
	// so the invitee is signed in without a second exchange.
	var hasSession bool
	for _, c := range resp.Cookies() {
		if c.Name == "__session" && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatalf("cookies = %v, want a __session cookie", resp.Cookies())
	}

	var name, email string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM staff WHERE id = $1`, accepted.StaffID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read staff row: %v", err)
	}
	// The address stored is the verified one, normalized -- not whatever
	// the caller typed, which is what a later invitation is matched on.
	if name != "Lena Vasquez" || email != "lena@example.com" {
		t.Fatalf("staff row = %q/%q", name, email)
	}

	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT array_to_string(roles, ','), employment_type::text FROM practice_memberships
		 WHERE practice_id = $1 AND staff_id = $2`, practiceID, accepted.StaffID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read membership: %v", err)
	}
	if roles != doulaRole || employmentType != contractorType {
		t.Fatalf("membership = %q/%q, want the Invitation's own (#266)", roles, employmentType)
	}

	var status, acceptedStaffID string
	var acceptedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, accepted_staff_id, accepted_at FROM practice_invitations WHERE id = $1`, invitationID,
	).Scan(&status, &acceptedStaffID, &acceptedAt); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if status != "accepted" || acceptedStaffID != accepted.StaffID || acceptedAt.IsZero() {
		t.Fatalf("invitation = %q/%q/%v, want acceptance recorded", status, acceptedStaffID, acceptedAt)
	}

	var eventType, actor string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2`, practiceID, accepted.StaffID,
	).Scan(&eventType, &actor); err != nil {
		t.Fatalf("read membership event: %v", err)
	}
	if eventType != "joined" || actor != accepted.StaffID {
		t.Fatalf("membership event = %q by %q, want joined by the accepter", eventType, actor)
	}
}

// TestAcceptInviteHandler_ResolvesAnExistingStaffRow covers the other
// half of #226: someone who already works at another Practice joins this
// one on the staff row she already has, not a second one.
func TestAcceptInviteHandler_ResolvesAnExistingStaffRow(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-inviting-a-known-person")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "known@example.com", "{admin}", employeeType, time.Now().Add(time.Hour))

	existingID := seedStaffWithEmail(t, db, "known-uid", "known@example.com")
	elsewhere := seedPractice(t, db, "Her Other Practice")
	seedMembership(t, db, elsewhere, existingID)

	srv := newAcceptServer(t, db, "known-uid", "known@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var accepted staffauth.AcceptInviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if accepted.StaffID != existingID {
		t.Fatalf("staffId = %q, want the existing staff row %q", accepted.StaffID, existingID)
	}

	var staffRows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff WHERE lower(email) = 'known@example.com'`,
	).Scan(&staffRows); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffRows != 1 {
		t.Fatalf("staff rows on that address = %d, want 1 (#291)", staffRows)
	}
}

// TestAcceptInviteHandler_AlreadyAMemberIs409 is the AC's named status:
// the current behaviour is a 500 from an unhandled unique violation.
func TestAcceptInviteHandler_AlreadyAMemberIs409(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-inviting-a-member-again")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "member@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	memberID := seedStaffWithEmail(t, db, "member-uid", "member@example.com")
	seedMembership(t, db, practiceID, memberID)

	srv := newAcceptServer(t, db, "member-uid", "member@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestAcceptInviteHandler_SecondIdentityOnOneAddressIs409 is the same
// conflict reached the way staff.email's lack of a unique constraint
// allows: a second identity provider, a second staff row, one address.
// Resolving by identity alone would let this through and put two rows for
// one person on the roster -- LV-G8 (#291) exactly.
func TestAcceptInviteHandler_SecondIdentityOnOneAddressIs409(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-and-two-identities")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "twice@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	firstIdentity := seedStaffWithEmail(t, db, "twice-uid-a", "twice@example.com")
	seedMembership(t, db, practiceID, firstIdentity)

	srv := newAcceptServer(t, db, "twice-uid-b", "twice@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Twice Over"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	var memberships int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_memberships pm JOIN staff s ON s.id = pm.staff_id
		 WHERE pm.practice_id = $1 AND lower(s.email) = 'twice@example.com'`, practiceID,
	).Scan(&memberships); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberships != 1 {
		t.Fatalf("memberships on that address = %d, want 1", memberships)
	}
}

func TestAcceptInviteHandler_WrongAddressForbidden(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-invite-wrong-address")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "invited@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "someone-else-uid", "someone.else@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Someone Else"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAcceptInviteHandler_NoVerifiedAddressForbidden(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-invite-no-address")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "someone@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "address-less-uid", "")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "No Address"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAcceptInviteHandler_ExpiredIsGoneAndMarked(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-invite-expired")
	invitationID, token := seedInvitationWithToken(t, db, practiceID, ownerID, "late@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))

	srv := newAcceptServer(t, db, "late-uid", "late@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Too Late"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusGone {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusGone)
	}

	// The expiry is written down on the way past, so the Owner's Staff
	// screen stops offering it as pending. It survives the rolled-back
	// request because nothing else in it succeeded.
	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM practice_invitations WHERE id = $1`, invitationID,
	).Scan(&status); err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if status != "expired" {
		t.Fatalf("status = %q, want expired", status)
	}
}

func TestAcceptInviteHandler_UnknownToken(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(t, db, "nobody-uid", "nobody@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: "not-a-real-token", Name: "Nobody"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAcceptInviteHandler_RevokedTokenIsNotFound(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-revoked-then-accepted")
	invitationID, token := seedInvitationWithToken(t, db, practiceID, ownerID, "gone@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`, invitationID,
	); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	srv := newAcceptServer(t, db, "gone-uid", "gone@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Too Late"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAcceptInviteHandler_BadRequests(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-invite-bad-requests")
	_, token := seedInvitationWithToken(t, db, practiceID, ownerID, "nameless@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	srv := newAcceptServer(t, db, "nameless-uid", "nameless@example.com")
	defer srv.Close()

	t.Run("invalid body", func(t *testing.T) {
		resp := postAcceptRaw(t, srv, []byte("not json"))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", Name: "Nameless"})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	// A new person must name herself: practice_invitations carries no
	// name column, so there is nothing to fall back to.
	t.Run("missing name for a new person", func(t *testing.T) {
		resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

func TestAcceptInviteHandler_NoCredential(t *testing.T) {
	db := testdb.New(t)
	srv := newAcceptServer(t, db, "unused-uid", "unused@example.com")
	defer srv.Close()

	payload, err := json.Marshal(staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: "whatever"})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/staff/accept-invite", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestAcceptInviteHandler_BackfillsOfferStaffID is ADR-0008's
// email-target Offer path closing (#317): an Offer mailed to an address
// names the Invitation, not a person, because no person existed to name.
// Accepting the Invitation is the moment one does, so every Offer on it
// gets the staff id it was always going to have -- past Offers included,
// since they are part of the history she reads.
func TestAcceptInviteHandler_BackfillsOfferStaffID(t *testing.T) {
	db := testdb.New(t)
	ownerID, practiceID := seedOwnerMembership(t, db, "owner-offering-by-email")
	invitationID, token := seedInvitationWithToken(t, db, practiceID, ownerID, "renata@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))

	var clientID, engagementID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Offer Client', 'offer-client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`, clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	var offerID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, invitation_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 52000, 'R', 'North side', now() + interval '90 days', $3, now() + interval '7 days')
		 RETURNING id`,
		engagementID, invitationID, ownerID,
	).Scan(&offerID); err != nil {
		t.Fatalf("seed offer: %v", err)
	}

	srv := newAcceptServer(t, db, "renata-uid", "renata@example.com")
	defer srv.Close()

	resp := postAccept(t, srv, staffauth.AcceptInviteRequest{WorkState: "NY", InviteToken: token, Name: "Renata Alvarez"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var accepted staffauth.AcceptInviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	var offerStaffID *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT staff_id::text FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&offerStaffID); err != nil {
		t.Fatalf("read offer: %v", err)
	}
	if offerStaffID == nil || *offerStaffID != accepted.StaffID {
		t.Fatalf("offer staff_id = %v, want the just-accepted %q", offerStaffID, accepted.StaffID)
	}
}
