package plans_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the plan_instances_client_birth_plan_visibility RLS
// policy from 00013_plan_instances_client_birth_plan_read.sql directly via
// db.App and set_config, bypassing the Go handler -- the same shape as
// rls_test.go's practice-tier tests.

// TestRLS_PlanInstancesClientCanReadOwnBirthPlan proves a Client-portal
// session can read the Birth Plan instance on their own Engagement.
func TestRLS_PlanInstancesClientCanReadOwnBirthPlan(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`, `{}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'birth_plan'`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("query plan_instances as Client: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected Client to see their own Birth Plan instance, got count = %d", count)
	}
}

// TestRLS_PlanInstancesClientCannotReadCarePlan proves the policy's
// plan_type = 'birth_plan' condition, not just the Engagement-ownership
// EXISTS: a Client-portal session sees zero rows for a Care Plan instance
// on their own Engagement -- zero rows, not an error, per the ticket's
// acceptance criteria.
func TestRLS_PlanInstancesClientCannotReadCarePlan(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedInstance(t, db, engagementID, carePlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{"f1":"secret"}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'care_plan'`, engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("query plan_instances as Client: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows for a Care Plan instance from a Client-portal session, got %d", count)
	}
}

// TestRLS_PlanInstancesClientCannotReadOtherClientsBirthPlan proves the
// policy's Engagement-ownership EXISTS subquery, not just the plan_type
// condition: a Client-portal session sees zero rows for a Birth Plan
// instance on an Engagement belonging to a different Client.
func TestRLS_PlanInstancesClientCannotReadOtherClientsBirthPlan(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientA, _ := seedClientEngagement(t, db, practiceID, "Client A", "a@example.com")
	_, engagementB := seedClientEngagement(t, db, practiceID, "Client B", "b@example.com")
	seedInstance(t, db, engagementB, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`, `{}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM plan_instances WHERE engagement_id = $1 AND plan_type = 'birth_plan'`, engagementB,
	).Scan(&count); err != nil {
		t.Fatalf("query plan_instances as Client A: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected Client A to see 0 rows for Client B's Birth Plan, got %d", count)
	}
}

// TestRLS_PlanInstancesClientUpdateRejected proves the policy is
// SELECT-only: a Client-portal session can't UPDATE their own Birth Plan
// instance, even though it can SELECT it -- the same shape as
// TestRLS_PlanInstancesUpdateRejectedAcrossPractice.
func TestRLS_PlanInstancesClientUpdateRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`, `{}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE plan_instances SET answers = '{"location":"hacked"}' WHERE engagement_id = $1 AND plan_type = 'birth_plan'`,
		engagementID,
	)
	if err != nil {
		t.Fatalf("update plan_instances: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected updating a Birth Plan as a Client-portal session, got %d", rows)
	}
}
