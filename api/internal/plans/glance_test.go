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
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Riverside Doulas")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)

	read := func(t *testing.T) (plans.ClientGlance, bool) {
		t.Helper()
		tx, err := db.App.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
			t.Fatalf("set_config: %v", err)
		}
		glance, found, err := plans.ClientBirthPlanGlance(t.Context(), tx, engagementID)
		if err != nil {
			t.Fatalf("glance: %v", err)
		}
		return glance, found
	}

	if _, found := read(t); found {
		t.Fatalf("found a Birth Plan before one is started")
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_instances (engagement_id, plan_type, fields, changed_at) VALUES ($1, 'birth_plan', '[]'::jsonb, now())`,
		engagementID); err != nil {
		t.Fatalf("seed birth plan: %v", err)
	}
	glance, found := read(t)
	if !found || glance.AcknowledgedAt != nil {
		t.Fatalf("glance = %+v, found = %v, want a Birth Plan she has not confirmed", glance, found)
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE plan_instances SET client_acknowledged_at = '2026-08-20T12:00:00Z' WHERE engagement_id = $1`,
		engagementID); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	glance, found = read(t)
	if !found || glance.AcknowledgedAt == nil || glance.AcknowledgedAt.UTC().Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("glance = %+v, found = %v, want acknowledged on 2026-08-20", glance, found)
	}
}
