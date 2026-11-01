package testdb_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

func TestDeadLetterReason(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Dead Letter Doulas")
	var pendingID, deadID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO low_credit_outbox (practice_id) VALUES ($1) RETURNING id`, practiceID,
	).Scan(&pendingID); err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO low_credit_outbox (practice_id, status, dead_letter_reason)
		 VALUES ($1, 'dead_lettered', 'own_data') RETURNING id`, practiceID,
	).Scan(&deadID); err != nil {
		t.Fatalf("seed dead-lettered: %v", err)
	}

	if got := testdb.DeadLetterReason(t, db, "low_credit_outbox", pendingID); got != "" {
		t.Fatalf("pending row reason = %q, want none", got)
	}
	if got := testdb.DeadLetterReason(t, db, "low_credit_outbox", deadID); got != "own_data" {
		t.Fatalf("dead-lettered row reason = %q, want own_data", got)
	}
}
