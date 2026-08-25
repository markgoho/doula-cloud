package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newStaffListServer mounts ListStaffHandler the way main.go really does
// -- through GatedRouter with the "owner","admin" declaration -- since
// the Owner/Admin-vs-Doula boundary lives at that mount, not inside the
// handler (#315).
func newStaffListServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	g.Get("/practices/{practiceId}/staff", []string{ownerRole, adminRole}, staffauth.ListStaffHandler())
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getStaffList(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/staff", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestListStaffHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-listing-staff"
	_, practiceID := seedStaffWithMembership(t, db, identityUID) // '{doula}', not owner

	srv, session := newStaffListServer(t, db, identityUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestListStaffHandler_AdminSuccess proves ADR-0008's read table widened
// this route past Owner-only: an Admin membership, which the pre-#315
// handler's internal RequireOwner check would have 403'd, now reaches it
// because the mount declares both roles.
func TestListStaffHandler_AdminSuccess(t *testing.T) {
	db := testdb.New(t)
	const adminUID = "admin-lists-staff"
	practiceID := seedPractice(t, db, "Admin Roster Practice")
	adminID := seedStaff(t, db, adminUID)
	seedMembershipWithRoles(t, db, practiceID, adminID, "{admin}")

	srv, session := newStaffListServer(t, db, adminUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListStaffHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-lists-staff"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	doulaID := seedStaff(t, db, "doula-on-roster")
	seedMembership(t, db, practiceID, doulaID) // seeds '{doula}'

	zeroRoleID := seedStaff(t, db, "zero-role-on-roster")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{}', 'employee')`,
		practiceID, zeroRoleID,
	); err != nil {
		t.Fatalf("seed zero-role membership: %v", err)
	}

	srv, session := newStaffListServer(t, db, ownerUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var roster staffauth.Roster
	if err := json.NewDecoder(resp.Body).Decode(&roster); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(roster.Members) != 3 {
		t.Fatalf("members = %+v, want 3 entries", roster.Members)
	}
	if len(roster.Invitations) != 0 {
		t.Fatalf("invitations = %+v, want none", roster.Invitations)
	}

	byID := map[string]staffauth.StaffSummary{}
	for _, s := range roster.Members {
		byID[s.StaffID] = s
	}
	owner, ok := byID[ownerID]
	if !ok || len(owner.Roles) != 1 || owner.Roles[0] != ownerRole {
		t.Fatalf("owner entry = %+v, want roles [owner]", owner)
	}
	doula, ok := byID[doulaID]
	if !ok || len(doula.Roles) != 1 || doula.Roles[0] != doulaRole {
		t.Fatalf("doula entry = %+v, want roles [doula]", doula)
	}
	zeroRole, ok := byID[zeroRoleID]
	if !ok || len(zeroRole.Roles) != 0 {
		t.Fatalf("zero-role entry = %+v, want no roles", zeroRole)
	}
	if owner.EmploymentType != employeeType {
		t.Fatalf("owner employmentType = %q, want employee", owner.EmploymentType)
	}
}

// TestListStaffHandler_PendingInvitationsAreTheirOwnGroup is #261's
// actual complaint: a pending Invitation must be tellable apart from a
// member holding no roles, which a single list could not do. It also
// covers #339's dead-letter flag, whose only read surface is this group.
func TestListStaffHandler_PendingInvitationsAreTheirOwnGroup(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-sees-invitations"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)

	pendingID := seedInvitation(t, db, practiceID, ownerID, "pending@example.com", "{doula}", contractorType, time.Now().Add(time.Hour))
	deadID := seedInvitation(t, db, practiceID, ownerID, "dead@example.com", "{admin}", employeeType, time.Now().Add(time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_invite_outbox (invitation_id, status) VALUES ($1, 'dead_lettered')`, deadID,
	); err != nil {
		t.Fatalf("seed dead-lettered outbox row: %v", err)
	}
	// A lapsed Invitation is still listed, flagged: it holds this
	// address's slot in practice_invitations_one_pending until it is
	// revoked or re-sent, and an Owner who cannot see it cannot act on it.
	expiredID := seedInvitation(t, db, practiceID, ownerID, "lapsed@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))
	// A revoked Invitation is history, not a pending ask -- it must not
	// appear in either group.
	revokedID := seedInvitation(t, db, practiceID, ownerID, "revoked@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`, revokedID,
	); err != nil {
		t.Fatalf("revoke invitation: %v", err)
	}

	srv, session := newStaffListServer(t, db, ownerUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	var roster staffauth.Roster
	if err := json.NewDecoder(resp.Body).Decode(&roster); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(roster.Members) != 1 {
		t.Fatalf("members = %+v, want only the Owner", roster.Members)
	}
	if len(roster.Invitations) != 3 {
		t.Fatalf("invitations = %+v, want 3 pending", roster.Invitations)
	}

	byID := map[string]staffauth.InvitationSummary{}
	for _, inv := range roster.Invitations {
		byID[inv.InvitationID] = inv
	}
	pending := byID[pendingID]
	if pending.Address != "pending@example.com" || pending.EmploymentType != contractorType ||
		len(pending.Roles) != 1 || pending.Roles[0] != doulaRole || pending.ExpiresAt == "" {
		t.Fatalf("pending invitation = %+v", pending)
	}
	if pending.DeliveryFailed {
		t.Fatalf("pending invitation reports a failed delivery: %+v", pending)
	}
	if !byID[deadID].DeliveryFailed {
		t.Fatalf("dead-lettered invitation = %+v, want deliveryFailed", byID[deadID])
	}
	if pending.Expired {
		t.Fatalf("live invitation = %+v, want not expired", pending)
	}
	if !byID[expiredID].Expired {
		t.Fatalf("lapsed invitation = %+v, want expired", byID[expiredID])
	}
}

// TestListStaffHandler_ReInviteAfterASendDoesNotDuplicate pins the shape
// staffinvite.Queue's partial-index upsert allows: once a send is marked
// 'sent' it leaves the index, so a re-invite inserts a second outbox row
// for one Invitation. The pending group must still print that Invitation
// once, and must read only the newest attempt -- a give-up the Owner has
// since re-sent past is no longer a warning.
func TestListStaffHandler_ReInviteAfterASendDoesNotDuplicate(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "owner-reinvites-after-a-send"
	ownerID, practiceID := seedOwnerMembership(t, db, ownerUID)
	invitationID := seedInvitation(t, db, practiceID, ownerID, "twice-sent@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO staff_invite_outbox (invitation_id, status, created_at)
		 VALUES ($1, 'dead_lettered', now() - interval '1 hour'), ($1, 'pending', now())`,
		invitationID,
	); err != nil {
		t.Fatalf("seed two outbox rows: %v", err)
	}

	srv, session := newStaffListServer(t, db, ownerUID)
	defer srv.Close()

	resp := getStaffList(t, srv, session, practiceID)
	defer resp.Body.Close()

	var roster staffauth.Roster
	if err := json.NewDecoder(resp.Body).Decode(&roster); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(roster.Invitations) != 1 {
		t.Fatalf("invitations = %+v, want exactly 1", roster.Invitations)
	}
	if roster.Invitations[0].DeliveryFailed {
		t.Fatalf("invitation = %+v, want the re-send to clear the flag", roster.Invitations[0])
	}
}

// seedInvitation inserts a pending practice_invitations row directly, for
// the read tests that need one without going through InviteHandler.
func seedInvitation(t *testing.T, db *testdb.DB, practiceID, invitedBy, address, roles, employmentType string, expiresAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type, $5, $6, $7) RETURNING id`,
		practiceID, address, roles, employmentType, staffauth.TokenDigest(address+"-token"), invitedBy, expiresAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed invitation for %q: %v", address, err)
	}
	return id
}
