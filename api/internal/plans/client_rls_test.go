package plans_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the plan_instances_client_birth_plan_visibility RLS
// policy from 00013_plan_instances_client_birth_plan_read.sql directly via
// db.App and set_config, bypassing the Go handler -- the same shape as
// rls_test.go's practice-tier tests.

// TestRLS_PlanInstancesClientVisibility proves a Client-portal session
// reads the Birth Plan instance on their own Engagement; that the policy's
// plan_type = 'birth_plan' condition, not just the Engagement-ownership
// EXISTS, hides a Care Plan instance on it (zero rows, not an error, per
// the ticket's acceptance criteria); and that the Engagement-ownership
// EXISTS subquery, not just the plan_type condition, hides a Birth Plan
// instance on an Engagement belonging to a different Client.
func TestRLS_PlanInstancesClientVisibility(t *testing.T) {
	cases := []struct {
		name     string
		planType string
		others   bool
		want     int
	}{
		{"her own Birth Plan", birthPlanType, false, 1},
		{"her own Care Plan", carePlanType, false, 0},
		{"another Client's Birth Plan", birthPlanType, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := testdb.New(t)
			practice := fixture.Practice(t, db)
			own := fixture.Engagement(t, db, practice)
			other := fixture.Engagement(t, db, practice)
			seeded := own
			if c.others {
				seeded = other
			}
			seedInstance(t, db, seeded.ID, c.planType, locationNoOptions, `{}`)

			tx := db.BeginAs(t, testdb.Client(own.Client.ID))

			count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = $2`, seeded.ID, c.planType)
			if count != c.want {
				t.Fatalf("a Client sees %d rows, want %d", count, c.want)
			}
		})
	}
}

// TestRLS_PlanInstancesClientUpdateRejected proves the policy is
// SELECT-only: a Client-portal session can't UPDATE their own Birth Plan
// instance, even though it can SELECT it -- the same shape as
// TestRLS_PlanInstancesUpdateRejectedAcrossPractice.
func TestRLS_PlanInstancesClientUpdateRejected(t *testing.T) {
	db := testdb.New(t)
	engagement := fixture.Engagement(t, db, fixture.Practice(t, db))
	seedInstance(t, db, engagement.ID, birthPlanType, locationNoOptions, `{}`)

	tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))

	rows := testdb.RowsAffected(t, tx,
		`UPDATE plan_instances SET answers = '{"location":"hacked"}' WHERE engagement_id = $1 AND plan_type = 'birth_plan'`,
		engagement.ID,
	)
	if rows != 0 {
		t.Fatalf("expected 0 rows affected updating a Birth Plan as a Client-portal session, got %d", rows)
	}
}
