package plans_test

import (
	"testing"

	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

// TestClientBirthPlanGlance reads the one line "Your care" says about her
// Birth Plan (#2007), on a Client-portal transaction: nothing before one
// is started, then the Plan with no acknowledgment, then the instant she
// confirmed she read it.
func TestClientBirthPlanGlance(t *testing.T) {
	e := newEnv(t)
	read := func() (plans.ClientGlance, bool) {
		return testdb.ReadAs(t, e.db, testdb.Client(e.Engagement.Client.ID), e.Engagement.ID, plans.ClientBirthPlanGlance)
	}

	if _, found := read(); found {
		t.Fatalf("found a Birth Plan before one is started")
	}

	e.seedInstance(birthPlanType, `[]`, `{}`)
	glance, found := read()
	if !found || glance.AcknowledgedAt != nil {
		t.Fatalf("glance = %+v, found = %v, want a Birth Plan she has not confirmed", glance, found)
	}

	e.exec(`UPDATE plan_instances SET client_acknowledged_at = '2026-08-20T12:00:00Z' WHERE engagement_id = $1`, e.Engagement.ID)
	glance, found = read()
	if !found || glance.AcknowledgedAt == nil || glance.AcknowledgedAt.UTC().Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("glance = %+v, found = %v, want acknowledged on 2026-08-20", glance, found)
	}
}
