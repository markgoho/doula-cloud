package client_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// portalRow is one client_portal_users row as the superuser reads it:
// the state, and the record of how it got there.
type portalRow struct {
	state        string
	hasToken     bool
	revokedBy    *string
	revokeReason *string
	revokedAt    *time.Time
	unlinkedAt   *time.Time
}

// portalRows reads clientID's rows, oldest first.
func portalRows(t *testing.T, db *testdb.DB, clientID string) []portalRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT state, invite_token IS NOT NULL, revoked_by_staff_id::text, revoke_reason::text, revoked_at, unlinked_at
		   FROM client_portal_users WHERE client_id = $1 ORDER BY created_at, id`, clientID)
	if err != nil {
		t.Fatalf("read portal rows: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []portalRow
	for rows.Next() {
		var r portalRow
		if err := rows.Scan(&r.state, &r.hasToken, &r.revokedBy, &r.revokeReason, &r.revokedAt, &r.unlinkedAt); err != nil {
			t.Fatalf("scan portal row: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate portal rows: %v", err)
	}
	return out
}

// requireRevoked fails unless r is a revocation recorded with who, when
// and reason.
func requireRevoked(t *testing.T, r portalRow, staffID, reason string) {
	t.Helper()
	if r.state != string(portalinvite.StateRevoked) || r.hasToken {
		t.Fatalf("row = %+v, want revoked with no token left to redeem", r)
	}
	if r.revokedBy == nil || *r.revokedBy != staffID || r.revokeReason == nil || *r.revokeReason != reason || r.revokedAt == nil {
		t.Fatalf("revocation = by %v, reason %v, at %v; want by %q, reason %q, with a time -- who stopped an invitation and why must have an answer", r.revokedBy, r.revokeReason, r.revokedAt, staffID, reason)
	}
}

// TestEraseHandler_AMergedClientWithTwoPortalRows is the erasure the
// audit inferred and nobody had run: a merge moves the absorbed record's
// accepted portal row onto the survivor, whose own pending invitation
// stays, and erasing her used to clear identity_uid on both rows and fail
// on client_portal_users_one_pending_per_client (00026) with a 500. The
// invitation is now revoked and the login unlinked -- two kinds of
// history a Client may hold any number of.
func TestEraseHandler_AMergedClientWithTwoPortalRows(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID, absorbedID, _ := e.twoRecordsOfOneWoman("Iris Vale", "postpartum")
	testdb.SeedPendingPortalInvite(t, e.DB, survivorID)
	testdb.SeedPortalUser(t, e.DB, "portal-iris-second", absorbedID)

	e.mergeOK(absorbedID, client.MergeRequest{GivenName: "Iris Vale", Email: "first@example.com", OtherClientID: survivorID})
	e.erasure(survivorID)

	rows := portalRows(t, e.DB, survivorID)
	if len(rows) != 2 {
		t.Fatalf("portal rows = %+v, want the invitation and the moved login", rows)
	}
	requireRevoked(t, rows[0], e.Staff.ID, "client_erased")
	if rows[1].state != string(portalinvite.StateUnlinked) || rows[1].unlinkedAt == nil {
		t.Fatalf("moved login = %+v, want unlinked with a time -- deleting the Portal Account nulls identity_uid through the FK, and the row has to say what that made it", rows[1])
	}
}

// TestEditHandler_EmailChangeRevokesALiveInvitation proves a revocation
// is a recorded transition, not an outbox last_error string. The invite
// had already been sent, so before 00168 nothing stopped it: its token
// stayed redeemable by whoever held the old address, and the screen
// went on reading "Invite sent".
func TestEditHandler_EmailChangeRevokesALiveInvitation(t *testing.T) {
	e := newEnv(t)
	clientID := e.attachedClient("Mae Holt", "mae-typo@example.com")
	testdb.SeedPendingPortalInvite(t, e.DB, clientID)
	e.Exec(`UPDATE portal_invite_outbox SET status = 'sent', sent_at = now()
	          WHERE client_portal_user_id IN (SELECT id FROM client_portal_users WHERE client_id = $1)`, clientID)

	reqtest.ExpectStatus(t, http.MethodPut, e.clientURL(clientID, ""), http.StatusOK, e.SignedIn,
		reqtest.JSON(client.EditRequest{GivenName: "Mae Holt", Email: "mae@example.com"}))

	rows := portalRows(t, e.DB, clientID)
	if len(rows) != 1 {
		t.Fatalf("portal rows = %+v, want one", rows)
	}
	requireRevoked(t, rows[0], e.Staff.ID, "email_changed")

	out := reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(clientID, ""), http.StatusOK, e.SignedIn)
	if out.PortalInviteStatus == nil || *out.PortalInviteStatus != portalinvite.StateRevoked || out.PortalInviteDelivery != nil {
		t.Fatalf("portalInviteStatus = %v, delivery %v; want revoked with no delivery -- the outbox's \"sent\" is not her access", out.PortalInviteStatus, out.PortalInviteDelivery)
	}
}

// TestMergeHandler_RecordsTheRevocationOfTheAbsorbedInvitation is the
// merge's half of the same record: merge.go used to null the token with
// no who or when.
func TestMergeHandler_RecordsTheRevocationOfTheAbsorbedInvitation(t *testing.T) {
	e := newEnv(t)
	survivorID := e.attachedClient("Nell Park", "nell@example.com")
	absorbedID, _ := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Nell Park", "nell2@example.com")
	testdb.SeedPendingPortalInvite(t, e.DB, absorbedID)
	e.setCreatedAt(survivorID, "2024-01-01T00:00:00Z")
	e.setCreatedAt(absorbedID, "2025-01-01T00:00:00Z")

	e.mergeOK(absorbedID, client.MergeRequest{GivenName: "Nell Park", OtherClientID: survivorID})

	rows := portalRows(t, e.DB, absorbedID)
	if len(rows) != 1 {
		t.Fatalf("portal rows = %+v, want one", rows)
	}
	requireRevoked(t, rows[0], e.Staff.ID, "record_merged")
}

// TestInviteAfterRevocation_IsANewRow proves a revoked invitation keeps
// its record: inviting her again writes a new row rather than rotating
// the old one back to life, and her access reads from the new one.
func TestInviteAfterRevocation_IsANewRow(t *testing.T) {
	e := newEnv(t)
	clientID := e.attachedClient("Ona Reed", "ona-old@example.com")
	testdb.SeedPendingPortalInvite(t, e.DB, clientID)
	reqtest.ExpectStatus(t, http.MethodPut, e.clientURL(clientID, ""), http.StatusOK, e.SignedIn,
		reqtest.JSON(client.EditRequest{GivenName: "Ona Reed", Email: "ona@example.com"}))
	testdb.SeedPendingPortalInvite(t, e.DB, clientID)

	rows := portalRows(t, e.DB, clientID)
	if len(rows) != 2 || rows[0].state != string(portalinvite.StateRevoked) || rows[1].state != string(portalinvite.StateInvited) {
		t.Fatalf("portal rows = %+v, want the revoked invitation kept beside the new one", rows)
	}
	out := reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(clientID, ""), http.StatusOK, e.SignedIn)
	if out.PortalInviteStatus == nil || *out.PortalInviteStatus != portalinvite.StateInvited {
		t.Fatalf("portalInviteStatus = %v, want invited -- the live invitation is hers ahead of the history", out.PortalInviteStatus)
	}
}

// TestSchema_RefusesAStateItsColumnsDoNotDescribe proves 00168's CHECKs:
// each state owns exactly its columns, so the combinations the old
// four-column shape allowed -- accepted and refused at once, a live
// invitation with no expiry, a revocation with no record of why -- are
// refused by the database rather than decided by a reader's `if` order.
func TestSchema_RefusesAStateItsColumnsDoNotDescribe(t *testing.T) {
	e := newEnv(t)
	clientID := e.attachedClient("Pia Lund", "pia@example.com")
	testdb.SeedPortalAccount(t, e.DB, "portal-pia", "pia@example.com")

	for _, tc := range []struct {
		name, insert, constraint string
	}{
		{"accepted and refused", `INSERT INTO client_portal_users (client_id, state, identity_uid, access_refused_at) VALUES ($1, 'accepted', 'portal-pia', now())`, "client_portal_users_refusal"},
		{"accepted with no login", `INSERT INTO client_portal_users (client_id, state) VALUES ($1, 'accepted')`, "client_portal_users_accepted_has_identity"},
		{"invited with no expiry", `INSERT INTO client_portal_users (client_id, state, invite_token) VALUES ($1, 'invited', gen_random_uuid())`, "client_portal_users_open_invite_has_expiry"},
		{"revoked with no reason", `INSERT INTO client_portal_users (client_id, state, revoked_at) VALUES ($1, 'revoked', now())`, "client_portal_users_revocation"},
		{"unlinked with no time", `INSERT INTO client_portal_users (client_id, state) VALUES ($1, 'unlinked')`, "client_portal_users_unlink"},
	} {
		_, err := e.DB.Admin.ExecContext(t.Context(), tc.insert, clientID)
		if err == nil || !strings.Contains(err.Error(), tc.constraint) {
			t.Errorf("%s: err = %v, want refused by %s", tc.name, err, tc.constraint)
		}
	}
}
