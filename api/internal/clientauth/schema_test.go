package clientauth_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// This file holds the schema invariants on client_portal_users that no
// RLS policy expresses -- the cardinality ADR-0015 states, asserted
// against the database rather than against the Go that happens to
// respect it. Its sibling rls_test.go covers the policies themselves.

// TestSchema_OnePortalAccountLinkPerClient proves #819's own settlement:
// ADR-0015 keeps one Portal Account reaching many Clients, at most one
// per Practice, so the table admits a second row for a second Practice's
// Client (the shape 00006's table-wide UNIQUE forbade, lifted by #309)
// and refuses a second row for a Client it already reaches.
func TestSchema_OnePortalAccountLinkPerClient(t *testing.T) {
	db := testdb.New(t)
	identifier := portalaccount.NewIdentifier()

	practiceA := testdb.SeedPractice(t, db, "Practice A")
	clientA, _ := testdb.SeedEngagementInStatus(t, db, practiceA, "Camille at A", "camille-a@example.com", "active")
	testdb.SeedPortalUser(t, db, identifier, clientA)

	practiceB := testdb.SeedPractice(t, db, "Practice B")
	clientB, _ := testdb.SeedEngagementInStatus(t, db, practiceB, "Camille at B", "camille-b@example.com", "active")
	// A second Practice's Client, reached by the same Portal Account:
	// AttachPortalUser fails the test itself if the insert is refused.
	testdb.AttachPortalUser(t, db, identifier, clientB)

	// The same Client twice is the pair the constraint exists to refuse.
	_, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identifier, clientB,
	)
	if err == nil {
		t.Fatal("a second link row for the same (Portal Account, Client) pair was admitted; ADR-0015 allows at most one per Practice, and a Client belongs to one Practice")
	}
	if !strings.Contains(err.Error(), "client_portal_users_identity_client_key") {
		t.Fatalf("insert refused by %v, want the (identity_uid, client_id) uniqueness constraint", err)
	}
}

// TestSchema_PendingInviteRowsAreNotPairConstrained proves the
// constraint above leaves the pre-acceptance shape alone: a pending row
// carries no identity_uid, Postgres treats NULLs as distinct, and
// client_portal_users_one_pending_per_client (00026) is what governs
// there instead.
//
// Built by direct insert on purpose. portalinvite.invite refuses to
// produce this state -- a Client whose row already carries an
// identity_uid is a 409, "this client already has portal access" -- so
// the only way to ask the database what it thinks of a NULL alongside a
// value is to write the pair itself. The question is worth asking
// because the answer is what keeps an invitation reachable after this
// constraint exists.
func TestSchema_PendingInviteRowsAreNotPairConstrained(t *testing.T) {
	db := testdb.New(t)
	identifier := portalaccount.NewIdentifier()

	practiceID := testdb.SeedPractice(t, db, "Pending Invite Practice")
	clientID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Invited Client", "invited@example.com", "intake")
	testdb.SeedPortalUser(t, db, identifier, clientID)

	// An accepted row and a pending row for one Client: the pair
	// constraint has nothing to say about the second, because its
	// identity_uid is NULL.
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid())`,
		clientID,
	); err != nil {
		t.Fatalf("a pending row alongside an accepted one was refused: %v", err)
	}
}
