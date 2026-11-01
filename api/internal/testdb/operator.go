package testdb

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
)

// SeedOperatorAct records a Platform operator named name and one
// operator_acts row of theirs, and returns the act's id. The row's
// action is 'sign_in', the one every operator has; a test about one
// act's own action writes that row itself.
func SeedOperatorAct(t *testing.T, db *DB, name string) (operatorActID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`WITH op AS (
		     INSERT INTO operators (identity_uid, name, email)
		     VALUES ('operator-' || gen_random_uuid(), $1, 'operator@example.com')
		     RETURNING id
		 )
		 INSERT INTO operator_acts (operator_id, action) SELECT id, 'sign_in' FROM op RETURNING id`, name,
	).Scan(&operatorActID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed operator act: %v", err)
	}
	return operatorActID
}

// SeedOperatorActivity writes one operator-kind activity row at
// practiceID, as ops writes it: through activity.Record on an ops_runtime
// transaction, keyed to a fresh act of an operator named operatorName.
// It returns the act's id.
func SeedOperatorActivity(t *testing.T, db *DB, practiceID, subjectKind, subjectID, action, operatorName string, at time.Time) (operatorActID string) {
	t.Helper()
	operatorActID = SeedOperatorAct(t, db, operatorName)
	tx, err := db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: fixture transaction failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed operator activity begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		Action:      action,
		Actor:       activity.OperatorActor(operatorActID),
		CreatedAt:   at,
	}); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed operator activity: %v", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: fixture transaction failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed operator activity commit: %v", err)
	}
	return operatorActID
}

// SeedFoundingGrant writes a founding grant of credits Credits at
// practiceID, created at at and keyed to a fresh act of an operator, as
// ops writes one. It returns the ledger row's id.
func SeedFoundingGrant(t *testing.T, db *DB, practiceID string, credits int, at time.Time) (lotID string) {
	t.Helper()
	actID := SeedOperatorAct(t, db, "Mark Goho")
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, operator_act_id, created_at)
		 VALUES ($1, 'founding_grant', $2, $3, $4) RETURNING id`, practiceID, credits, actID, at,
	).Scan(&lotID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed founding grant: %v", err)
	}
	return lotID
}
