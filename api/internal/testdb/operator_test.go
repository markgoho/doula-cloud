package testdb_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

func TestSeedOperatorActivity(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Operator Practice")
	at := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

	actID := testdb.SeedOperatorActivity(t, db, practiceID, activity.SubjectPractice, practiceID, "credits_refunded", "Mark Goho", at)

	var operatorName, actorKind, action string
	var createdAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT o.name, a.actor_kind::text, a.action, a.created_at
		   FROM activity a
		   JOIN operator_acts oa ON oa.id = a.actor_operator_act_id
		   JOIN operators o ON o.id = oa.operator_id
		  WHERE a.practice_id = $1 AND oa.id = $2`, practiceID, actID,
	).Scan(&operatorName, &actorKind, &action, &createdAt); err != nil {
		t.Fatalf("read seeded operator row: %v", err)
	}
	if operatorName != "Mark Goho" || actorKind != "operator" || action != "credits_refunded" || !createdAt.Equal(at) {
		t.Fatalf("row = (%s, %s, %s, %v), want (Mark Goho, operator, credits_refunded, %v)", operatorName, actorKind, action, createdAt, at)
	}
}

func TestSeedFoundingGrant(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Grant Practice")
	at := time.Date(2026, 10, 30, 14, 0, 0, 0, time.UTC)

	lotID := testdb.SeedFoundingGrant(t, db, practiceID, 6, at)

	var quantity int
	var createdAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT l.quantity, l.created_at FROM credit_ledger l JOIN operator_acts oa ON oa.id = l.operator_act_id
		  WHERE l.id = $1 AND l.practice_id = $2 AND l.origin = 'founding_grant'`, lotID, practiceID,
	).Scan(&quantity, &createdAt); err != nil {
		t.Fatalf("read seeded grant: %v", err)
	}
	if quantity != 6 || !createdAt.Equal(at) {
		t.Fatalf("grant = (%d, %v), want (6, %v)", quantity, createdAt, at)
	}
}

func TestSeedPilotAddress(t *testing.T) {
	db := testdb.New(t)
	testdb.ClosePilotGate(t, db)

	testdb.SeedPilotAddress(t, db, " Ana@Example.com ")

	var admitted bool
	if err := db.App.QueryRowContext(t.Context(), `SELECT pilot_admits('ana@example.com')`).Scan(&admitted); err != nil {
		t.Fatalf("ask pilot_admits: %v", err)
	}
	var keyed int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM pilot_allowlist WHERE address = 'ana@example.com' AND operator_act_id IS NOT NULL`,
	).Scan(&keyed); err != nil {
		t.Fatalf("read the row: %v", err)
	}
	if !admitted || keyed != 1 {
		t.Fatalf("admitted = %v, keyed rows = %d, want true and 1", admitted, keyed)
	}
}
