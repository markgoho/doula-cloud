package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// #2591: the Engagement read says which of its Care Plan, Birth Plan and
// Contract exist, so the hub asks only for those and never for one that
// would answer 404.
func TestDetailHandler_SaysWhichRecordsExist(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "has-records-owner"
	practiceID := testdb.SeedPractice(t, db, "Has Records Practice")
	testdb.SeedNamedStaffAtPractice(t, db, practiceID, ownerUID, "Renata Owner", []string{ownerRole}, employeeType)
	_, bareID := testdb.SeedEngagementInStatus(t, db, practiceID, "Amara", "has-records-bare@example.com", engagement.StatusActive)
	_, careID := testdb.SeedEngagementInStatus(t, db, practiceID, "Bea", "has-records-care@example.com", engagement.StatusActive)
	_, fullID := testdb.SeedEngagementInStatus(t, db, practiceID, "Cleo", "has-records-full@example.com", engagement.StatusActive)
	for _, seed := range []struct{ engagementID, planType string }{
		{careID, "care_plan"},
		{fullID, "care_plan"},
		{fullID, "birth_plan"},
	} {
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO plan_instances (engagement_id, plan_type, fields, changed_at) VALUES ($1, $2::plan_type, '[]'::jsonb, now())`,
			seed.engagementID, seed.planType); err != nil {
			t.Fatalf("seed plan: %v", err)
		}
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', '', 15000)`, fullID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	srv, _ := newServer(t, db, ownerUID)
	defer srv.Close()

	for _, tc := range []struct {
		name                        string
		engagementID                string
		hasCare, hasBirth, hasContr bool
	}{
		{"nothing started", bareID, false, false, false},
		{"only a Care Plan", careID, true, false, false},
		{"all three", fullID, true, true, true},
	} {
		got := readDetail(t, db, srv, ownerUID, practiceID, tc.engagementID)
		if got.HasCarePlan != tc.hasCare || got.HasBirthPlan != tc.hasBirth || got.HasContract != tc.hasContr {
			t.Errorf("%s: hasCarePlan/hasBirthPlan/hasContract = %v/%v/%v, want %v/%v/%v", tc.name,
				got.HasCarePlan, got.HasBirthPlan, got.HasContract, tc.hasCare, tc.hasBirth, tc.hasContr)
		}
	}
}
