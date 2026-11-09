package contracts_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// updatedRows runs an UPDATE on tx and returns how many rows it touched.
func updatedRows(t *testing.T, tx *sql.Tx, query string, args ...any) int64 {
	t.Helper()
	result, err := tx.ExecContext(t.Context(), query, args...)
	if err != nil {
		t.Fatalf("update contracts: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	return rows
}

// clientsContract is a Client with a Contract at status on her own
// Engagement, and the Engagement of another Client of the same Practice
// with a Contract at the same status.
type clientsContract struct {
	db                *testdb.DB
	clientID          string
	engagementID      string
	otherEngagementID string
}

func seedClientsContract(t *testing.T, status string) clientsContract {
	t.Helper()
	db := testdb.New(t)
	practice := fixture.Practice(t, db)
	own := fixture.Engagement(t, db, practice)
	other := fixture.Engagement(t, db, practice)
	seedContract(t, db, own.ID, status, mergeFieldProse)
	seedContract(t, db, other.ID, status, mergeFieldProse)
	return clientsContract{db: db, clientID: own.Client.ID, engagementID: own.ID, otherEngagementID: other.ID}
}

// asClient opens a Client-portal transaction for her.
func (c clientsContract) asClient(t *testing.T) *sql.Tx {
	t.Helper()
	return c.db.BeginAs(t, testdb.Client(c.clientID))
}

// These tests exercise the contract_templates_practice_visibility RLS
// policy from 00014_contract_templates.sql directly via db.App and
// set_config, bypassing the Go handlers -- the same shape as plans'
// rls_test.go.

// TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet proves
// contract_templates denies all rows when app.current_practice_id is
// never set.
func TestRLS_ContractTemplatesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	seedContractTemplate(t, db, practiceID, "Some prose")

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM contract_templates`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows contract_templates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_ContractTemplatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	seedContractTemplate(t, db, practiceA, "Practice A's prose")
	seedContractTemplate(t, db, practiceB, "Practice B's prose")

	tx := db.BeginAs(t, testdb.Practice(practiceA))

	visiblePracticeIDs := testdb.Column[string](t, tx, `SELECT practice_id FROM contract_templates`)

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
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	seedContractTemplate(t, db, practiceB, "Practice B's prose")

	tx := db.BeginAs(t, testdb.Practice(practiceA))

	rows := updatedRows(t, tx, `UPDATE contract_templates SET prose = 'hacked' WHERE practice_id = $1`, practiceB)
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
	practiceID := testdb.SeedPractice(t, db, "Some Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, "Some prose")

	count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM contracts`)
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_ContractsSelectIsScopedViaEngagementsExistsSubquery proves the
// policy narrows contracts to rows whose Engagement belongs to
// app.current_practice_id, not every row globally.
func TestRLS_ContractsSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	_, engagementA := testdb.SeedEngagement(t, db, practiceA)
	_, engagementB := testdb.SeedEngagement(t, db, practiceB)
	seedContract(t, db, engagementA, statusDraft, "Practice A's prose")
	seedContract(t, db, engagementB, statusDraft, "Practice B's prose")

	tx := db.BeginAs(t, testdb.Practice(practiceA))

	visibleEngagementIDs := testdb.Column[string](t, tx, `SELECT engagement_id FROM contracts`)

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
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	practiceB := testdb.SeedPractice(t, db, "Practice B")
	_, engagementB := testdb.SeedEngagement(t, db, practiceB)
	seedContract(t, db, engagementB, statusDraft, "Practice B's prose")

	tx := db.BeginAs(t, testdb.Practice(practiceA))

	rows := updatedRows(t, tx, `UPDATE contracts SET merge_field_values = '{"hacked":"true"}' WHERE engagement_id = $1`, engagementB)
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Contract at a different Practice, got %d", rows)
	}
}

// These tests exercise the contracts_client_visibility RLS policy from
// 00017_contracts_client_visibility.sql directly via db.App and
// set_config, bypassing the Go handlers -- the same shape as plans'
// client-tier RLS tests for plan_instances_client_birth_plan_visibility.

// TestRLS_ContractsClientReadsOnlyHerOwnSentOrLaterContract proves the
// policy's status IN (...) condition covers all three post-Send statuses,
// not just 'sent', and hides a Draft on her own Engagement -- zero rows,
// not an error, per the ticket's AC. The Engagement-ownership EXISTS
// subquery is proved beside it: a Client-portal session sees zero rows
// for a Contract on an Engagement belonging to a different Client, at any
// status.
func TestRLS_ContractsClientReadsOnlyHerOwnSentOrLaterContract(t *testing.T) {
	cases := []struct {
		status    string
		wantOwn   int
		wantOther int
	}{
		{statusDraft, 0, 0},
		{statusSent, 1, 0},
		{statusSigned, 1, 0},
		{statusVoided, 1, 0},
	}
	for _, c := range cases {
		t.Run(c.status, func(t *testing.T) {
			seeded := seedClientsContract(t, c.status)
			tx := seeded.asClient(t)

			own := testdb.Scalar[int](t, tx, `SELECT count(*) FROM contracts WHERE engagement_id = $1`, seeded.engagementID)
			other := testdb.Scalar[int](t, tx, `SELECT count(*) FROM contracts WHERE engagement_id = $1`, seeded.otherEngagementID)
			if own != c.wantOwn || other != c.wantOther {
				t.Fatalf("a Client sees %d of her own %s Contracts and %d of another Client's, want %d and %d", own, c.status, other, c.wantOwn, c.wantOther)
			}
		})
	}
}

// These tests exercise the contracts_client_sign RLS policy from
// 00018_contracts_signing.sql directly via db.App and set_config,
// bypassing the Go handler -- the sent -> signed transition it admits, on
// the caller's own Engagement only.

// TestRLS_ContractsClientSignsOnlyHerOwnSentContract proves the policy
// allows a Client-portal session to transition their own sent Contract to
// signed; that its Engagement-ownership EXISTS subquery, not just the
// status condition, stops her signing a sent Contract on an Engagement
// belonging to a different Client; and that its USING clause, not just
// WITH CHECK, hides an already-signed or voided Contract from this UPDATE
// altogether, so the sent -> signed transition can't be re-run or
// reversed through it.
func TestRLS_ContractsClientSignsOnlyHerOwnSentContract(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		otherOnes bool
		want      int64
	}{
		{"her own sent Contract", statusSent, false, 1},
		{"another Client's sent Contract", statusSent, true, 0},
		{"her own signed Contract", statusSigned, false, 0},
		{"her own voided Contract", statusVoided, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seeded := seedClientsContract(t, c.status)
			engagementID := seeded.engagementID
			if c.otherOnes {
				engagementID = seeded.otherEngagementID
			}
			tx := seeded.asClient(t)

			rows := updatedRows(t, tx, `UPDATE contracts SET status = 'signed'::contract_status WHERE engagement_id = $1`, engagementID)
			if rows != c.want {
				t.Fatalf("signing: %d rows affected, want %d", rows, c.want)
			}
		})
	}
}

// TestRLS_ContractsClientWritesOutsideTheSignTransitionRejected proves
// contracts_client_visibility itself grants no write access, and that the
// sign policy's WITH CHECK, not just USING, admits only sent -> signed.
// The row is visible for UPDATE (contracts_client_sign's USING clause
// passes on a 'sent' Contract the caller owns), so Postgres evaluates
// WITH CHECK against the new row and rejects it outright rather than
// silently affecting 0 rows -- unlike the SELECT-only case, where an
// invisible row is simply filtered out. That holds for an UPDATE that
// leaves the status at 'sent' and for one that moves it to voided (or any
// status other than signed).
func TestRLS_ContractsClientWritesOutsideTheSignTransitionRejected(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"a field edit that leaves the status sent", `UPDATE contracts SET merge_field_values = '{"hacked":"true"}' WHERE engagement_id = $1`},
		{"moving a sent Contract to voided", `UPDATE contracts SET status = 'voided'::contract_status WHERE engagement_id = $1`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			seeded := seedClientsContract(t, statusSent)
			tx := seeded.asClient(t)

			if _, err := tx.ExecContext(t.Context(), c.query, seeded.engagementID); err == nil {
				t.Fatal("expected an RLS violation updating a Contract as a Client-portal session, got no error")
			}
		})
	}
}
