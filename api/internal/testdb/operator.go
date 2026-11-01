package testdb

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
)

// SeedOperatorAct records an operator named name and one sign_in act of theirs, and returns the act's id.
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

// SeedOperatorActivity writes one operator-kind activity row the way ops does, and returns the act's id.
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

// SeedFoundingGrant writes a founding grant the way ops does, and returns the ledger row's id.
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

// SeedPilotAddress adds address to the pilot allowlist the way ops does.
func SeedPilotAddress(t *testing.T, db *DB, address string) {
	t.Helper()
	actID := SeedOperatorAct(t, db, "Mark Goho")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO pilot_allowlist (address, operator_act_id, added_at) VALUES (lower(btrim($1)), $2, now())`, address, actID,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed pilot address: %v", err)
	}
}

// SeedLegalPublication publishes a version the way ops does, and returns how many emails it queued.
func SeedLegalPublication(t *testing.T, db *DB, document, version string, material bool, at time.Time) (queued int) {
	t.Helper()
	actID := SeedOperatorAct(t, db, "Mark Goho")
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT ops_publish_legal_version($1, $2, $3, $4, $5)`, actID, document, version, material, at,
	).Scan(&queued); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed legal publication: %v", err)
	}
	return queued
}
