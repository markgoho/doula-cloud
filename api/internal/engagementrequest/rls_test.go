package engagementrequest_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// beginAs opens a tx on db.App and sets app.current_practice_id and
// app.current_staff_id on it, mirroring staffauth.Middleware's own
// set_config calls -- the pattern billing/rls_test.go and
// staffauth/rls_test.go already use, since a *sql.DB connection pool
// gives no guarantee that two separate ExecContext calls land on the
// same backend connection, but everything run on one *sql.Tx does.
func beginAs(t *testing.T, db *testdb.DB, practiceID, staffID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set current_practice_id: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_staff_id', $1, true)`, staffID); err != nil {
		t.Fatalf("set current_staff_id: %v", err)
	}
	return tx
}

// TestRLS_ContractorInsertRefused proves engagement_requests_insert
// (00047) refuses a contractor Doula's INSERT independent of
// RequestHandler's own Go-level check.
func TestRLS_ContractorInsertRefused(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	contractorID := seedMember(t, db, practiceID, "contractor-1", []string{doulaRole}, contractorType)
	clientID := seedClient(t, db, practiceID)
	tx := beginAs(t, db, practiceID, contractorID)

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, 'birth', $3, $4)`,
		practiceID, clientID, testDueDate, contractorID,
	)
	if err == nil {
		t.Fatal("expected engagement_requests_insert to refuse a contractor Doula's INSERT, got no error")
	}
}

// TestRLS_EmployeeDoulaInsertAllowed proves the same policy admits an
// employee Doula, so the contractor refusal above is the policy's role
// gate working, not a blanket refusal.
func TestRLS_EmployeeDoulaInsertAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	doulaID := seedMember(t, db, practiceID, "doula-1", []string{doulaRole}, employeeType)
	clientID := seedClient(t, db, practiceID)
	tx := beginAs(t, db, practiceID, doulaID)

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, 'birth', $3, $4)`,
		practiceID, clientID, testDueDate, doulaID,
	)
	if err != nil {
		t.Fatalf("expected an employee Doula's INSERT to be admitted, got: %v", err)
	}
}

// TestRLS_ContractorOwnerInsertAllowed proves ADR-0017's solo-Practice
// carve-out: a contractor employment type does not refuse an Owner or
// Admin membership.
func TestRLS_ContractorOwnerInsertAllowed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db)
	ownerID := seedMember(t, db, practiceID, "owner-1", []string{ownerRole}, contractorType)
	clientID := seedClient(t, db, practiceID)
	tx := beginAs(t, db, practiceID, ownerID)

	_, err := tx.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, requested_by)
		 VALUES ($1, $2, 'birth', $3, $4)`,
		practiceID, clientID, testDueDate, ownerID,
	)
	if err != nil {
		t.Fatalf("expected a contractor Owner's INSERT to be admitted, got: %v", err)
	}
}
