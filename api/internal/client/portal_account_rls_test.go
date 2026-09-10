package client_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These exercise portal_accounts_erasure_delete
// (00108_portal_account_survives_another_practice.sql) directly through
// db.App and set_config, the same way payments' own RLS tests do for the
// policies they cover. client.enqueuePortalErasure asks
// portal_account_reaches_a_live_client before it ever issues the DELETE,
// so these prove the second boundary: the one that holds when a future
// caller does not ask.

// seedTwoPracticeLogin gives one Portal Account a Client at each of two
// Practices -- ADR-0015's shape -- and returns the two Practices' ids,
// the two Clients', and the login's identifier.
func seedTwoPracticeLogin(t *testing.T, db *testdb.DB, tag string) (practiceA, clientA, clientB, portalUID string) {
	t.Helper()
	practiceA = testdb.SeedPractice(t, db, "Rooted "+tag)
	practiceB := testdb.SeedPractice(t, db, "Ridgeline "+tag)
	clientA, _ = testdb.SeedNamedEngagement(t, db, practiceA, "Camille Rooted", "camille-a-"+tag+"@example.com")
	clientB, _ = testdb.SeedNamedEngagement(t, db, practiceB, "Camille Ridgeline", "camille-b-"+tag+"@example.com")
	portalUID = "portal-rls-" + tag
	testdb.SeedPortalAccount(t, db, portalUID, portalUID+"@example.com")
	testdb.AttachPortalUser(t, db, portalUID, clientA)
	testdb.AttachPortalUser(t, db, portalUID, clientB)
	return practiceA, clientA, clientB, portalUID
}

// markErased stamps erased_at the way client.redactRecord does, without
// running the whole act -- these tests are about the policy, not about
// the endpoint that trips it.
func markErased(t *testing.T, db *testdb.DB, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET erased_at = now() WHERE id = $1`, clientID,
	); err != nil {
		t.Fatalf("mark erased: %v", err)
	}
}

// deleteLoginAs runs the erasure DELETE as app_runtime under practiceID's
// own session variables, and reports how many rows the policy let it
// remove.
func deleteLoginAs(t *testing.T, db *testdb.DB, practiceID, portalUID string) int64 {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	var res sql.Result
	if res, err = tx.ExecContext(t.Context(),
		`DELETE FROM portal_accounts WHERE identifier = $1`, portalUID); err != nil {
		t.Fatalf("delete portal account: %v", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		// coverage:ignore reason: lib/pq always reports a row count for a DELETE, not exercised by tests
		t.Fatalf("rows affected: %v", err)
	}
	return affected
}

// TestRLS_PortalAccountDeleteIsRefusedWhileAnotherPracticeReachesIt is
// #830's refusal at the database. Rooted has erased its own Client;
// Ridgeline's has not been touched, so the login is still hers to use
// and the policy admits no row to the DELETE at all.
func TestRLS_PortalAccountDeleteIsRefusedWhileAnotherPracticeReachesIt(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, _, portalUID := seedTwoPracticeLogin(t, db, "refused")
	markErased(t, db, clientA)

	if affected := deleteLoginAs(t, db, practiceA, portalUID); affected != 0 {
		t.Fatalf("rows deleted = %d, want 0 -- another Practice's un-erased Client still reaches this login", affected)
	}
}

// TestRLS_PortalAccountDeleteIsAdmittedOnceNobodyReachesIt is the other
// half: with both Practices' Clients erased, the last one out may take
// the login, and the policy's own practice-scoped EXISTS still governs
// which Practice is allowed to ask.
func TestRLS_PortalAccountDeleteIsAdmittedOnceNobodyReachesIt(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, clientB, portalUID := seedTwoPracticeLogin(t, db, "admitted")
	markErased(t, db, clientA)
	markErased(t, db, clientB)

	if affected := deleteLoginAs(t, db, practiceA, portalUID); affected != 1 {
		t.Fatalf("rows deleted = %d, want 1 -- no un-erased Client reaches this login any more", affected)
	}
}
