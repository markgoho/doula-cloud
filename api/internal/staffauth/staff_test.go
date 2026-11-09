package staffauth_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// roster reads the Practice's roster, with query appended, as the Staff
// member at the screen.
func (e *env) roster(query string) staffauth.Roster {
	e.T.Helper()
	return reqtest.Expect[staffauth.Roster](e.T, http.MethodGet, e.PracticeURL("/staff"+query), http.StatusOK, e.SignedIn)
}

// TestListStaffHandler_ReadGate holds ADR-0008's read table at the mount:
// a Doula may not list Staff, and (#315) an Admin may, which the pre-#315
// handler's internal RequireOwner check would have refused.
func TestListStaffHandler_ReadGate(t *testing.T) {
	cases := []struct {
		name string
		role fixture.StaffOption
		want int
	}{
		{"a Doula", fixture.Doula(), http.StatusForbidden},
		{"an Admin", fixture.Admin(), http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.role))

			reqtest.ExpectStatus(t, http.MethodGet, e.PracticeURL("/staff"), tc.want, e.SignedIn)
		})
	}
}

func TestListStaffHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()
	zeroRole := e.Colleague(fixture.NoRole())

	roster := e.roster("")

	if len(roster.Members) != 3 {
		t.Fatalf("members = %+v, want 3 entries", roster.Members)
	}
	if len(roster.Invitations.Items) != 0 {
		t.Fatalf("invitations = %+v, want none", roster.Invitations)
	}

	byID := map[string]staffauth.StaffSummary{}
	for _, s := range roster.Members {
		byID[s.StaffID] = s
	}
	owner, ok := byID[e.Staff.ID]
	if !ok || len(owner.Roles) != 1 || owner.Roles[0] != ownerRole {
		t.Fatalf("owner entry = %+v, want roles [owner]", owner)
	}
	doulaEntry, ok := byID[doula.ID]
	if !ok || len(doulaEntry.Roles) != 1 || doulaEntry.Roles[0] != doulaRole {
		t.Fatalf("doula entry = %+v, want roles [doula]", doulaEntry)
	}
	zeroRoleEntry, ok := byID[zeroRole.ID]
	if !ok || len(zeroRoleEntry.Roles) != 0 {
		t.Fatalf("zero-role entry = %+v, want no roles", zeroRoleEntry)
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
	e := newEnv(t, signedInAs(fixture.Owner()))
	inOneHour := time.Now().Add(time.Hour)

	pendingID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "pending@example.com", "{doula}", contractorType, inOneHour)
	deadID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "dead@example.com", "{admin}", employeeType, inOneHour)
	e.Exec(`INSERT INTO staff_invite_outbox (invitation_id, status, dead_letter_reason) VALUES ($1, 'dead_lettered', 'retries_exhausted')`, deadID)
	// A lapsed Invitation is still listed, flagged: it holds this
	// address's slot in practice_invitations_one_pending until it is
	// revoked or re-sent, and an Owner who cannot see it cannot act on it.
	expiredID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "lapsed@example.com", "{doula}", employeeType, time.Now().Add(-time.Hour))
	// A revoked Invitation is history, not a pending ask -- it must not
	// appear in either group.
	revokedID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "revoked@example.com", "{doula}", employeeType, inOneHour)
	e.Exec(`UPDATE practice_invitations SET status = 'revoked' WHERE id = $1`, revokedID)

	roster := e.roster("")

	if len(roster.Members) != 1 {
		t.Fatalf("members = %+v, want only the Owner", roster.Members)
	}
	if len(roster.Invitations.Items) != 3 {
		t.Fatalf("invitations = %+v, want 3 pending", roster.Invitations)
	}

	byID := map[string]staffauth.InvitationSummary{}
	for _, inv := range roster.Invitations.Items {
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
	e := newEnv(t, signedInAs(fixture.Owner()))
	invitationID := seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, "twice-sent@example.com", "{doula}", employeeType, time.Now().Add(time.Hour))
	e.Exec(`INSERT INTO staff_invite_outbox (invitation_id, status, dead_letter_reason, created_at)
		 VALUES ($1, 'dead_lettered', 'retries_exhausted', now() - interval '1 hour'), ($1, 'pending', NULL, now())`, invitationID)

	roster := e.roster("")

	if len(roster.Invitations.Items) != 1 {
		t.Fatalf("invitations = %+v, want exactly 1", roster.Invitations)
	}
	if roster.Invitations.Items[0].DeliveryFailed {
		t.Fatalf("invitation = %+v, want the re-send to clear the flag", roster.Invitations.Items[0])
	}
}

// TestListStaffHandler_InvalidCursorRejected mirrors
// message.TestListHandler_InvalidCursorRejected.
func TestListStaffHandler_InvalidCursorRejected(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		reqtest.ExpectStatus(t, http.MethodGet, e.PracticeURL("/staff?cursor="+cursor), http.StatusBadRequest, e.SignedIn)
	}
}

// TestListStaffHandler_PaginatesInvitationsNewestFirst seeds more than
// one page of pending Invitations and walks the cursor, mirroring
// message.TestListHandler_PaginatesNewestFirst. Members stays whole
// (#446), so only Invitations has a cursor to walk.
func TestListStaffHandler_PaginatesInvitationsNewestFirst(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	const total = 31 // invitationPageSize (30) + 1, to force a second page
	for i := range total {
		seedInvitation(t, e.DB, e.Practice.ID, e.Staff.ID, fmt.Sprintf("invite-%d@example.com", i), "{doula}", employeeType, time.Now().Add(time.Hour))
	}

	first := e.roster("")
	if len(first.Invitations.Items) != 30 || !first.Invitations.HasMore || first.Invitations.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Invitations.Items), first.Invitations.HasMore, first.Invitations.NextCursor)
	}

	second := e.roster("?cursor=" + *first.Invitations.NextCursor)
	if len(second.Invitations.Items) != 1 || second.Invitations.HasMore || second.Invitations.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Invitations.Items), second.Invitations.HasMore, second.Invitations.NextCursor)
	}
}

// seedInvitation inserts a pending practice_invitations row directly, for
// the read tests that need one without going through InviteHandler.
func seedInvitation(t *testing.T, db *testdb.DB, practiceID, invitedBy, address, roles, employmentType string, expiresAt time.Time) string {
	t.Helper()
	id := testdb.Scalar[string](t, db.Admin, `INSERT INTO practice_invitations
		     (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, $2, $3::practice_role[], $4::employment_type, $5, $6, $7) RETURNING id`, practiceID, address, roles, employmentType, staffauth.TokenDigest(address+"-token"), invitedBy, expiresAt)
	return id
}
