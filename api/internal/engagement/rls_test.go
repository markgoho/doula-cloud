package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00005_client_engagement.sql directly via db.App and set_config,
// bypassing the Go handlers -- proving the SQL policies themselves scope
// visibility correctly, not just that the handlers happen to agree with
// them.

func TestRLS_ClientsFailsClosedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "fail-closed-staff")
	seedClientEngagement(t, db, practiceID, "Some Client", "some@example.com", "intake")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM clients`).Scan(&count); err != nil {
		t.Fatalf("query clients with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_ClientsSelectIsScopedViaEngagementsExistsSubquery proves the
// clients_select policy narrows to Clients who have an Engagement at
// app.current_practice_id, per the ticket's EXISTS-subquery requirement,
// not to every Client row globally.
func TestRLS_ClientsSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedStaffWithMembership(t, db, "staff-at-a")
	practiceB := seedStaffWithMembership(t, db, "staff-at-b")
	clientAtA, _ := seedClientEngagement(t, db, practiceA, "Client A", "a@example.com", "intake")
	seedClientEngagement(t, db, practiceB, "Client B", "b@example.com", "intake")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM clients`)
	if err != nil {
		t.Fatalf("query clients: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleIDs = append(visibleIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleIDs) != 1 || visibleIDs[0] != clientAtA {
		t.Fatalf("visible clients = %v, want only %q", visibleIDs, clientAtA)
	}
}

// TestRLS_ClientsInsertRejectedWithNoPracticeSet proves clients_insert
// fails closed too: an INSERT attempted with no app.current_practice_id
// set is rejected by RLS, not silently allowed.
func TestRLS_ClientsInsertRejectedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(t.Context(), `INSERT INTO clients (name, email) VALUES ('No Context', 'no-context@example.com')`)
	if err == nil {
		t.Fatal("expected INSERT to be rejected by RLS with no app.current_practice_id set, got no error")
	}
}

// TestRLS_ClientsInsertAllowedWithPracticeSet proves clients_insert
// permits an INSERT once a Practice context is set, even though no
// Engagement referencing the new Client exists yet -- the chicken-and-egg
// case CreateHandler relies on.
func TestRLS_ClientsInsertAllowedWithPracticeSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-inserting")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(), `INSERT INTO clients (name, email) VALUES ('With Context', 'with-context@example.com')`); err != nil {
		t.Fatalf("expected INSERT to be allowed with a Practice context set, got error: %v", err)
	}
}

// TestRLS_EngagementsVisibilityIsScopedToCurrentPractice proves the
// engagements policy narrows to rows for app.current_practice_id, not
// every Engagement globally.
func TestRLS_EngagementsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedStaffWithMembership(t, db, "staff-eng-a")
	practiceB := seedStaffWithMembership(t, db, "staff-eng-b")
	seedClientEngagement(t, db, practiceA, "Client A", "a@example.com", "intake")
	seedClientEngagement(t, db, practiceB, "Client B", "b@example.com", "intake")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM engagements`).Scan(&count); err != nil {
		t.Fatalf("query engagements: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only Practice A's engagement visible, got count = %d", count)
	}
}
