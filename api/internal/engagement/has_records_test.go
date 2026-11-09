package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// #2591: the Engagement read says which of its Care Plan, Birth Plan and
// Contract exist, so the hub asks only for those and never for one that
// would answer 404.
func TestDetailHandler_SaysWhichRecordsExist(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	active := fixture.Status(fixture.StatusActive)
	care := e.AnotherEngagement(active)
	full := e.AnotherEngagement(active)
	for _, plan := range []struct{ engagementID, planType string }{
		{care.ID, "care_plan"},
		{full.ID, "care_plan"},
		{full.ID, "birth_plan"},
	} {
		e.Exec(`INSERT INTO plan_instances (engagement_id, plan_type, fields, changed_at) VALUES ($1, $2::plan_type, '[]'::jsonb, now())`,
			plan.engagementID, plan.planType)
	}
	e.Exec(`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', '', 15000)`, full.ID)

	for _, tc := range []struct {
		name                        string
		engagementID                string
		hasCare, hasBirth, hasContr bool
	}{
		{"nothing started", e.Engagement.ID, false, false, false},
		{"only a Care Plan", care.ID, true, false, false},
		{"all three", full.ID, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := e.detailOf(tc.engagementID, e.Staff)
			if got.HasCarePlan != tc.hasCare || got.HasBirthPlan != tc.hasBirth || got.HasContract != tc.hasContr {
				t.Errorf("hasCarePlan/hasBirthPlan/hasContract = %v/%v/%v, want %v/%v/%v",
					got.HasCarePlan, got.HasBirthPlan, got.HasContract, tc.hasCare, tc.hasBirth, tc.hasContr)
			}
		})
	}
}
