package plans_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the plan_templates_practice_visibility RLS policy
// from 00011_plan_templates.sql and the plan_instances_practice_visibility
// policy from 00012_plan_instances.sql directly via db.App and set_config,
// bypassing the Go handlers -- the same shape as staffauth's rls_test.go
// and visit/rls_test.go's EXISTS-against-engagements policy tests.

// TestRLS_PlanTablesFailClosedWithNoSessionVarSet proves plan_templates and
// plan_instances deny all rows when app.current_practice_id is never set.
func TestRLS_PlanTablesFailClosedWithNoSessionVarSet(t *testing.T) {
	e := newEnv(t)
	e.seedTemplate(carePlanType, f1Fields)
	e.seedInstance(carePlanType, f1Fields, `{}`)

	testdb.RequireFailsClosed(t, e.db, "plan_templates")
	testdb.RequireFailsClosed(t, e.db, "plan_instances")
}

// TestRLS_PlanTemplatesVisibilityIsScopedToCurrentPractice proves the
// plain-column-comparison policy narrows plan_templates to rows for
// app.current_practice_id, not every row globally.
func TestRLS_PlanTemplatesVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db)
	practiceB := fixture.Practice(t, db)
	seedPlanTemplate(t, db, practiceA.ID, carePlanType, f1Fields)
	seedPlanTemplate(t, db, practiceB.ID, carePlanType, f1Fields)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	visiblePracticeIDs := testdb.Column[string](t, tx, `SELECT practice_id FROM plan_templates`)

	if len(visiblePracticeIDs) != 1 || visiblePracticeIDs[0] != practiceA.ID {
		t.Fatalf("visible practice_ids = %v, want only %q", visiblePracticeIDs, practiceA.ID)
	}
}

// TestRLS_PlanInstancesSelectIsScopedViaEngagementsExistsSubquery proves the
// policy narrows plan_instances to rows whose Engagement belongs to
// app.current_practice_id, not every row globally.
func TestRLS_PlanInstancesSelectIsScopedViaEngagementsExistsSubquery(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db)
	engagementA := fixture.Engagement(t, db, practiceA)
	engagementB := fixture.Engagement(t, db, fixture.Practice(t, db))
	seedInstance(t, db, engagementA.ID, carePlanType, f1Fields, `{}`)
	seedInstance(t, db, engagementB.ID, carePlanType, f1Fields, `{}`)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	visibleEngagementIDs := testdb.Column[string](t, tx, `SELECT engagement_id FROM plan_instances`)

	if len(visibleEngagementIDs) != 1 || visibleEngagementIDs[0] != engagementA.ID {
		t.Fatalf("visible engagement_ids = %v, want only %q", visibleEngagementIDs, engagementA.ID)
	}
}

// TestRLS_PlanInstancesUpdateRejectedAcrossPractice proves the policy
// scopes UPDATE, not just SELECT: a session acting as Practice A can't
// edit a Plan Instance that belongs to Practice B's Engagement, even with
// the correct engagement id, because the row isn't visible to update in
// the first place.
func TestRLS_PlanInstancesUpdateRejectedAcrossPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db)
	engagementB := fixture.Engagement(t, db, fixture.Practice(t, db))
	seedInstance(t, db, engagementB.ID, carePlanType, f1Fields, `{}`)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	rows := testdb.RowsAffected(t, tx,
		`UPDATE plan_instances SET answers = '{"f1":"hacked"}' WHERE engagement_id = $1 AND plan_type = $2`,
		engagementB.ID, carePlanType,
	)
	if rows != 0 {
		t.Fatalf("expected 0 rows affected editing a Plan Instance at a different Practice, got %d", rows)
	}
}
