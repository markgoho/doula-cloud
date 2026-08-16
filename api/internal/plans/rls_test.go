package plans_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise the plan_templates_practice_visibility RLS policy
// from 00011_plan_templates.sql directly via db.App and set_config,
// bypassing the Go handlers -- the same shape as staffauth's rls_test.go.

// TestRLS_PlanTemplatesFailsClosedWithNoSessionVarSet proves plan_templates
// denies all rows when app.current_practice_id is never set.
func TestRLS_PlanTemplatesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	seedTemplate(t, db, practiceID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM plan_templates`).Scan(&count); err != nil {
		t.Fatalf("query plan_templates with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_PlanTemplatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows plan_templates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_PlanTemplatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	seedTemplate(t, db, practiceA, carePlanType, `[{"id":"a","type":"short_text","label":"A field","order":0}]`)
	seedTemplate(t, db, practiceB, carePlanType, `[{"id":"b","type":"short_text","label":"B field","order":0}]`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visiblePracticeIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT practice_id FROM plan_templates`)
	if err != nil {
		t.Fatalf("query plan_templates: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visiblePracticeIDs = append(visiblePracticeIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visiblePracticeIDs) != 1 || visiblePracticeIDs[0] != practiceA {
		t.Fatalf("visible practice_ids = %v, want only %q", visiblePracticeIDs, practiceA)
	}
}

// These tests exercise the plan_instances_practice_visibility RLS policy
// from 00012_plan_instances.sql directly via db.App and set_config,
// bypassing the Go handlers -- the same shape as visit/rls_test.go's
// EXISTS-against-engagements policy tests.

// TestRLS_PlanInstancesFailsClosedWithNoSessionVarSet proves plan_instances
// denies all rows when app.current_practice_id is never set.
func TestRLS_PlanInstancesFailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Some Practice")
	engagementID := seedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{}`)

	var count int
	if err := db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM plan_instances`).Scan(&count); err != nil {
		t.Fatalf("query plan_instances with no session vars set: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows with no session variable set, got %d", count)
	}
}

// TestRLS_PlanInstancesSelectIsScopedViaEngagementsExistsSubquery proves the
// policy narrows plan_instances to rows whose Engagement belongs to
// app.current_practice_id, not every row globally.
func TestRLS_PlanInstancesSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementA := seedEngagement(t, db, practiceA)
	engagementB := seedEngagement(t, db, practiceB)
	seedInstance(t, db, engagementA, carePlanType, `[{"id":"a","type":"short_text","label":"A field","order":0}]`, `{}`)
	seedInstance(t, db, engagementB, carePlanType, `[{"id":"b","type":"short_text","label":"B field","order":0}]`, `{}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visibleEngagementIDs []string
	rows, err := tx.QueryContext(t.Context(), `SELECT engagement_id FROM plan_instances`)
	if err != nil {
		t.Fatalf("query plan_instances: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visibleEngagementIDs = append(visibleEngagementIDs, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(visibleEngagementIDs) != 1 || visibleEngagementIDs[0] != engagementA {
		t.Fatalf("visible engagement_ids = %v, want only %q", visibleEngagementIDs, engagementA)
	}
}

// TestRLS_PlanInstancesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as Practice A can't
// edit a Plan Instance that belongs to Practice B's Engagement, even with
// the correct engagement id, because the row isn't visible to update in
// the first place.
func TestRLS_PlanInstancesUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	engagementB := seedEngagement(t, db, practiceB)
	seedInstance(t, db, engagementB, carePlanType, `[{"id":"f1","type":"short_text","label":"Name","order":0}]`, `{}`)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	result, err := tx.ExecContext(t.Context(),
		`UPDATE plan_instances SET answers = '{"f1":"hacked"}' WHERE engagement_id = $1 AND plan_type = $2`,
		engagementB, carePlanType,
	)
	if err != nil {
		t.Fatalf("update plan_instances: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Plan Instance at a different Practice, got %d", rows)
	}
}
