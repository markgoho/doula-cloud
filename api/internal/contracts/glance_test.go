package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

// TestClientContractGlance reads the one line "Your care" says about her
// Contract (#2007), on a Client-portal transaction: nothing before one is
// sent, nothing for a Draft her RLS hides, and the newest she may see,
// with when she signed it, once there is one.
func TestClientContractGlance(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Riverside Doulas")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)

	read := func(t *testing.T) (contracts.ClientGlance, bool) {
		t.Helper()
		tx, err := db.App.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
			t.Fatalf("set_config: %v", err)
		}
		glance, found, err := contracts.ClientContractGlance(t.Context(), tx, engagementID)
		if err != nil {
			t.Fatalf("glance: %v", err)
		}
		return glance, found
	}

	if _, found := read(t); found {
		t.Fatalf("found a Contract before any exists")
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', 'Test prose', 15000)`,
		engagementID); err != nil {
		t.Fatalf("seed draft: %v", err)
	}
	if _, found := read(t); found {
		t.Fatalf("found a Draft, want it hidden from her")
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET status = 'signed', signed_at = '2026-08-14T15:00:00Z' WHERE engagement_id = $1`,
		engagementID); err != nil {
		t.Fatalf("sign contract: %v", err)
	}
	glance, found := read(t)
	if !found || glance.Status != "signed" {
		t.Fatalf("glance = %+v, found = %v, want a signed Contract", glance, found)
	}
	if glance.SignedAt == nil || glance.SignedAt.UTC().Format("2006-01-02") != "2026-08-14" {
		t.Fatalf("signedAt = %v, want 2026-08-14", glance.SignedAt)
	}
}
