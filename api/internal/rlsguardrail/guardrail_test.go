// Package rlsguardrail holds a single, dedicated cross-cutting test proving
// tenant isolation holds -- both RLS tiers (practice-level and
// client-level) plus the fail-closed no-session-var case -- so a
// regression is caught here even if a future change trims or weakens the
// per-feature RLS tests in staffauth/, engagement/, clientauth/, and
// visit/. See #47/#55.
//
// It runs against the "engagements" table (00005_client_engagement.sql,
// widened by 00006_client_portal_users.sql), the first table in the schema
// to carry both tiers' policies at once (engagements_practice_visibility
// and engagements_client_visibility), making it the natural place to prove
// both hold together -- clients and staff later grew their own both-tier
// policies too (00009_messaging_client_portal_read.sql), covered directly
// by message/client_rls_test.go rather than duplicated here. No dedicated
// CI step is added for this package -- the existing "api" job in
// .github/workflows/ci.yml already runs `go test ./...` on every push and
// pull_request with no path filter, so it already runs this test on every
// change to schema or RLS policies.
package rlsguardrail_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// seedEngagementAt inserts a Practice, a Client, and an Engagement linking
// them, using the superuser Admin connection so fixture setup isn't gated
// by the RLS policies under test.
func seedEngagementAt(t *testing.T, db *testdb.DB, label string) (practiceID, clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, label,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $2 || '@example.com') RETURNING id`, practiceID, label,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return practiceID, clientID, engagementID
}

// engagementVisible sets sessionVar to sessionValue for a single
// transaction on db.App (the low-privilege, RLS-subject connection) and
// reports whether engagementID is visible under that session context.
func engagementVisible(t *testing.T, db *testdb.DB, sessionVar, sessionValue, engagementID string) bool {
	t.Helper()

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config($1, $2, true)`, sessionVar, sessionValue); err != nil {
		t.Fatalf("set_config %s: %v", sessionVar, err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements WHERE id = $1`, engagementID).Scan(&count); err != nil {
		t.Fatalf("query engagements: %v", err)
	}
	return count == 1
}

// TestRLS_GuardrailPracticeTierIsolation exercises
// engagements_practice_visibility: the correct app.current_practice_id
// sees only that Practice's Engagement, and a different Practice's
// session sees none of it.
func TestRLS_GuardrailPracticeTierIsolation(t *testing.T) {
	db := testdb.New(t)
	practiceA, _, engagementA := seedEngagementAt(t, db, "Guardrail Practice A")
	practiceB, _, engagementB := seedEngagementAt(t, db, "Guardrail Practice B")

	if !engagementVisible(t, db, "app.current_practice_id", practiceA, engagementA) {
		t.Fatal("Practice A's own session could not see its own Engagement")
	}
	if !engagementVisible(t, db, "app.current_practice_id", practiceB, engagementB) {
		t.Fatal("Practice B's own session could not see its own Engagement")
	}
	if engagementVisible(t, db, "app.current_practice_id", practiceB, engagementA) {
		t.Fatal("Practice B's session saw Practice A's Engagement")
	}
	if engagementVisible(t, db, "app.current_practice_id", practiceA, engagementB) {
		t.Fatal("Practice A's session saw Practice B's Engagement")
	}
}

// TestRLS_GuardrailClientTierIsolation exercises
// engagements_client_visibility: the correct app.current_client_id sees
// only that Client's Engagement, and a different Client's session sees
// none of it.
func TestRLS_GuardrailClientTierIsolation(t *testing.T) {
	db := testdb.New(t)
	_, clientA, engagementA := seedEngagementAt(t, db, "Guardrail Client A")
	_, clientB, engagementB := seedEngagementAt(t, db, "Guardrail Client B")

	if !engagementVisible(t, db, "app.current_client_id", clientA, engagementA) {
		t.Fatal("Client A's own session could not see its own Engagement")
	}
	if !engagementVisible(t, db, "app.current_client_id", clientB, engagementB) {
		t.Fatal("Client B's own session could not see its own Engagement")
	}
	if engagementVisible(t, db, "app.current_client_id", clientB, engagementA) {
		t.Fatal("Client B's session saw Client A's Engagement")
	}
	if engagementVisible(t, db, "app.current_client_id", clientA, engagementB) {
		t.Fatal("Client A's session saw Client B's Engagement")
	}
}

// TestRLS_GuardrailFailsClosedWithNoSessionVariableSet proves the
// fail-closed case: a query issued through the RLS-subject App connection
// with neither app.current_practice_id nor app.current_client_id set
// returns zero rows -- without erroring -- even though a matching row
// genuinely exists.
func TestRLS_GuardrailFailsClosedWithNoSessionVariableSet(t *testing.T) {
	db := testdb.New(t)
	seedEngagementAt(t, db, "Guardrail Fail Closed")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements`).Scan(&count); err != nil {
		t.Fatalf("query engagements with no session variables set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set (fail closed), got %d", count)
	}
}
