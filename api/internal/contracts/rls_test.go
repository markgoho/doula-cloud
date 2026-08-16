package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the contract_templates_practice_visibility RLS
// policy from 00014_contract_templates.sql directly via db.App and
// set_config, bypassing the Go handlers -- the same shape as plans'
// rls_test.go.

// TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet proves
// contract_templates denies all rows when app.current_practice_id is
// never set.
func TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	seedTemplate(t, db, practiceID, "Some prose")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM contract_templates`).Scan(&count); err != nil {
		t.Fatalf("query contract_templates with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows contract_templates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedTemplate(t, db, practiceA, "Practice A's prose")
	seedTemplate(t, db, practiceB, "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visiblePracticeIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT practice_id FROM contract_templates`)
	if err != nil {
		t.Fatalf("query contract_templates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visiblePracticeIDs = append(visiblePracticeIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visiblePracticeIDs) != 1 || visiblePracticeIDs[0] != practiceA {
		t.Fatalf("visible practice_ids = %v, want only %q", visiblePracticeIDs, practiceA)
	}
}

// TestRLS_ContractTemplatesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as Practice A can't
// edit a Contract Template that belongs to Practice B, even with the
// correct practice id, because the row isn't visible to update in the
// first place.
func TestRLS_ContractTemplatesUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedTemplate(t, db, practiceB, "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE contract_templates SET prose = 'hacked' WHERE practice_id = $1`,
		practiceB,
	)
	if err != nil {
		t.Fatalf("update contract_templates: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Contract Template at a different Practice, got %d", rows)
	}
}

// These tests exercise the contracts_practice_visibility RLS policy from
// 00016_contracts.sql directly via db.App and set_config, bypassing the
// Go handlers -- the same shape as plans' EXISTS-against-engagements
// policy tests for plan_instances.

// TestRLS_ContractsFailsClosedWithNoSessionVarSet proves contracts denies
// all rows when app.current_practice_id is never set.
func TestRLS_ContractsFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, "draft", "Some prose")

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM contracts`).Scan(&count); err != nil {
		t.Fatalf("query contracts with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_ContractsSelectIsScopedViaEngagementsExistsSubquery proves the
// policy narrows contracts to rows whose Engagement belongs to
// app.current_practice_id, not every row globally.
func TestRLS_ContractsSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA)
	engagementB := seedEngagement(t, db, practiceB)
	seedContract(t, db, engagementA, "draft", "Practice A's prose")
	seedContract(t, db, engagementB, "draft", "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleEngagementIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT engagement_id FROM contracts`)
	if err != nil {
		t.Fatalf("query contracts: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleEngagementIDs = append(visibleEngagementIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleEngagementIDs) != 1 || visibleEngagementIDs[0] != engagementA {
		t.Fatalf("visible engagement_ids = %v, want only %q", visibleEngagementIDs, engagementA)
	}
}

// TestRLS_ContractsUpdateRejectedAcrossPractice proves the policy scopes
// UPDATE, not just SELECT: a session acting as Practice A can't edit a
// Contract that belongs to Practice B's Engagement, even with the
// correct engagement id, because the row isn't visible to update in the
// first place.
func TestRLS_ContractsUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementB := seedEngagement(t, db, practiceB)
	seedContract(t, db, engagementB, "draft", "Practice B's prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE contracts SET merge_field_values = '{"hacked":"true"}' WHERE engagement_id = $1`,
		engagementB,
	)
	if err != nil {
		t.Fatalf("update contracts: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Contract at a different Practice, got %d", rows)
	}
}

// These tests exercise the contracts_client_visibility RLS policy from
// 00017_contracts_client_visibility.sql directly via db.App and
// set_config, bypassing the Go handlers -- the same shape as plans'
// client-tier RLS tests for plan_instances_client_birth_plan_visibility.

// TestRLS_ContractsClientCanReadOwnSentContract proves a Client-portal
// session can read a sent Contract on their own Engagement.
func TestRLS_ContractsClientCanReadOwnSentContract(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContract(t, db, engagementID, "sent", "Agreement prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM contracts WHERE engagement_id = $1`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("query contracts as Client: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected Client to see their own sent Contract, got count = %d", count)
	}
}

// TestRLS_ContractsClientCannotReadDraft proves the policy's status IN
// (...) condition, not just the Engagement-ownership EXISTS: a
// Client-portal session sees zero rows for a Draft Contract on their own
// Engagement -- zero rows, not an error, per the ticket's AC.
func TestRLS_ContractsClientCannotReadDraft(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContract(t, db, engagementID, "draft", "Agreement prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM contracts WHERE engagement_id = $1`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("query contracts as Client: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows for a Draft Contract from a Client-portal session, got %d", count)
	}
}

// TestRLS_ContractsClientCanReadSignedAndVoided proves the policy's
// status set covers all three post-Send statuses, not just 'sent'.
func TestRLS_ContractsClientCanReadSignedAndVoided(t *testing.T) {
	for _, status := range []string{statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			practiceID := seedPractice(t, db, "Practice")
			clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			seedContract(t, db, engagementID, status, "Agreement prose")

			tx, err := db.App.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer tx.Rollback()

			if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
				t.Fatalf("set_config: %v", err)
			}

			var count int
			if err := tx.QueryRowContext(t.Context(),
				`SELECT count(*) FROM contracts WHERE engagement_id = $1`, engagementID,
			).Scan(&count); err != nil {
				t.Fatalf("query contracts as Client: %v", err)
			}
			if count != 1 {
				t.Fatalf("expected Client to see their own %s Contract, got count = %d", status, count)
			}
		})
	}
}

// TestRLS_ContractsClientCannotReadOtherClientsContract proves the
// policy's Engagement-ownership EXISTS subquery, not just the status
// condition: a Client-portal session sees zero rows for a sent Contract
// on an Engagement belonging to a different Client.
func TestRLS_ContractsClientCannotReadOtherClientsContract(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientA, _ := seedClientEngagement(t, db, practiceID, "Client A", "a@example.com")
	_, engagementB := seedClientEngagement(t, db, practiceID, "Client B", "b@example.com")
	seedContract(t, db, engagementB, "sent", "Agreement prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM contracts WHERE engagement_id = $1`, engagementB,
	).Scan(&count); err != nil {
		t.Fatalf("query contracts as Client A: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected Client A to see 0 rows for Client B's Contract, got %d", count)
	}
}

// TestRLS_ContractsClientUpdateRejected proves the policy is SELECT-only:
// a Client-portal session can't UPDATE their own sent Contract, even
// though it can SELECT it.
func TestRLS_ContractsClientUpdateRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedContract(t, db, engagementID, "sent", "Agreement prose")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE contracts SET merge_field_values = '{"hacked":"true"}' WHERE engagement_id = $1`,
		engagementID,
	)
	if err != nil {
		t.Fatalf("update contracts: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected updating a Contract as a Client-portal session, got %d", rows)
	}
}
