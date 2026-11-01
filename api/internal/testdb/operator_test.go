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
