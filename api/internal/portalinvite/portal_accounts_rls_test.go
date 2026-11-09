package portalinvite_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise portal_accounts' own RLS policies (#616's
// migration) directly via db.App and set_config, the same shape
// rls_test.go uses for client_portal_users.

// TestRLS_PortalAccountsSelfInsertRejectsMismatchedIdentity proves
// portal_accounts_self_insert's WITH CHECK: a caller cannot create a
// Portal Account under an identifier other than the one it presents as
// app.current_identity_uid.
func TestRLS_PortalAccountsSelfInsertRejectsMismatchedIdentity(t *testing.T) {
	db := testdb.New(t)

	tx := db.BeginAs(t, testdb.Identity("portal_the-real-caller"))
	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ('portal_someone-else', 'mismatched@example.com')`,
	)
	if err == nil {
		t.Fatal("expected the insert to be rejected -- identifier must equal the caller's own app.current_identity_uid")
	}
}

// TestRLS_PortalAccountsSelfInsertFailsClosedWithNoIdentitySet proves the
// door stays closed with no session context at all -- the state a
// bootstrap transaction starts in before acceptInvite sets anything.
func TestRLS_PortalAccountsSelfInsertFailsClosedWithNoIdentitySet(t *testing.T) {
	db := testdb.New(t)

	tx := db.BeginAs(t)

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ('portal_no-session', 'nosession@example.com')`,
	)
	if err == nil {
		t.Fatal("expected the insert to be rejected with no app.current_identity_uid set")
	}
}

// TestRLS_PortalAccountsSelfInsertSucceedsForMatchingIdentity proves the
// door acceptInvite relies on actually opens.
func TestRLS_PortalAccountsSelfInsertSucceedsForMatchingIdentity(t *testing.T) {
	db := testdb.New(t)

	tx := db.BeginAs(t, testdb.Identity("portal_matching-caller"))
	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ('portal_matching-caller', 'matching@example.com')`,
	); err != nil {
		t.Fatalf("expected the insert to succeed for a matching identifier: %v", err)
	}
}
