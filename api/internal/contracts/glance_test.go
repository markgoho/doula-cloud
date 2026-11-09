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
	e := newEnv(t)
	read := func() (contracts.ClientGlance, bool) {
		return testdb.ReadAs(t, e.db, testdb.Client(e.Engagement.Client.ID), e.Engagement.ID, contracts.ClientContractGlance)
	}

	if _, found := read(); found {
		t.Fatalf("found a Contract before any exists")
	}

	e.seedDraft()
	if _, found := read(); found {
		t.Fatalf("found a Draft, want it hidden from her")
	}

	e.exec(`UPDATE contracts SET status = 'signed', signed_at = '2026-08-14T15:00:00Z' WHERE engagement_id = $1`, e.Engagement.ID)
	glance, found := read()
	if !found || glance.Status != statusSigned {
		t.Fatalf("glance = %+v, found = %v, want a signed Contract", glance, found)
	}
	if glance.SignedAt == nil || glance.SignedAt.UTC().Format("2006-01-02") != "2026-08-14" {
		t.Fatalf("signedAt = %v, want 2026-08-14", glance.SignedAt)
	}
}
