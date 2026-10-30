package portal_test

import (
	"encoding/json"
	"slices"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// seedBirthPlanHistory writes one row of each Birth Plan action, and one
// Visit beside them -- the row that stays whatever the Engagement's
// outcome, so a test can tell "the Birth Plan rows left" from "the whole
// ledger went empty".
func seedBirthPlanHistory(t *testing.T, db *testdb.DB, practiceID, engagementID, staffUID string) {
	t.Helper()
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, staffUID, []string{ownerRole}, "employee")
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionBirthPlanStarted), activity.StaffActor(staffID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionBirthPlanEdited), activity.StaffActor(staffID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionBirthPlanAcknowledged), activity.StaffActor(staffID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(staffID))
}

// setEngagementFacts sets the two facts engagement.OffersBirthPlan reads,
// with 00093's correction door open for the transaction: moving a
// recorded outcome back to a live birth is exactly the correction the
// freeze trigger otherwise refuses.
func setEngagementFacts(t *testing.T, db *testdb.DB, engagementID, kind string, birthOutcome *string) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		t.Fatalf("open the correction door: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE engagements SET kind = $2::engagement_kind, birth_outcome = $3::birth_outcome,
		        pregnancy_ended_on = CASE WHEN $3::text IS NULL THEN NULL ELSE '2027-03-04'::date END
		 WHERE id = $1`,
		engagementID, kind, birthOutcome); err != nil {
		t.Fatalf("set engagement facts: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit engagement facts: %v", err)
	}
}

func portalActions(t *testing.T, url, session string) []string {
	t.Helper()
	resp := authedActivityGet(t, session, url)
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	actions := make([]string, 0, len(got.Items))
	for _, item := range got.Items {
		actions = append(actions, item.Action)
	}
	slices.Sort(actions)
	return actions
}

// TestActivityHandler_BirthPlanRowsFollowOffersBirthPlan proves #2219 and
// the portal half of #2277 at the reader: wherever
// engagement.OffersBirthPlan is false -- a loss, an outcome the Practice
// never learned, a postpartum Engagement -- her ledger names no Birth
// Plan, and correcting the outcome back to a live birth brings every row
// back, because nothing was stored to hide them.
func TestActivityHandler_BirthPlanRowsFollowOffersBirthPlan(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-birth-plan-offer"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Birth Plan Offer Practice")
	seedBirthPlanHistory(t, db, practiceID, engagementID, "portal-activity-birth-plan-offer-staff")

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()
	url := srv.URL + "/api/portal/engagements/" + engagementID + "/activity"

	every := []string{
		string(activity.ActionBirthPlanAcknowledged),
		string(activity.ActionBirthPlanEdited),
		string(activity.ActionBirthPlanStarted),
		string(activity.ActionVisitLogged),
	}
	onlyVisit := []string{string(activity.ActionVisitLogged)}
	loss, unknown, liveBirth := engagement.OutcomeLoss, engagement.OutcomeUnknown, engagement.OutcomeLiveBirth

	cases := []struct {
		name    string
		kind    engagement.Kind
		outcome *string
		want    []string
	}{
		{"expected birth", engagement.KindBirth, nil, every},
		{"after a loss", engagement.KindBirth, &loss, onlyVisit},
		{"outcome unknown", engagement.KindBirth, &unknown, onlyVisit},
		{"corrected to a live birth", engagement.KindBirth, &liveBirth, every},
		{"postpartum", engagement.KindPostpartum, &liveBirth, onlyVisit},
	}
	for _, tc := range cases {
		setEngagementFacts(t, db, engagementID, string(tc.kind), tc.outcome)
		if got := portalActions(t, url, session); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: actions = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestActivityHandler_BirthPlanOmissionKeepsPagesWhole proves #2219's
// cursor criterion: the Birth Plan rows leave in the query, not after
// it, so a page left out of them is still a full page and the next
// cursor still finds the rest.
func TestActivityHandler_BirthPlanOmissionKeepsPagesWhole(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-activity-birth-plan-paginate"
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, "Activity Birth Plan Paginate Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "portal-activity-birth-plan-paginate-staff", []string{ownerRole}, "employee")

	// 31 Visits with a Birth Plan row after each: newest first, a filter
	// applied after the page was cut would leave 15 Visits on page one.
	for range 31 {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(staffID))
		testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionBirthPlanEdited), activity.StaffActor(staffID))
	}
	loss := engagement.OutcomeLoss
	setEngagementFacts(t, db, engagementID, string(engagement.KindBirth), &loss)

	srv, session := activityServer(t, db, identityUID)
	defer srv.Close()
	base := srv.URL + "/api/portal/engagements/" + engagementID + "/activity"

	firstResp := authedActivityGet(t, session, base)
	defer firstResp.Body.Close()
	var first activityfeed.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v; want 30/true", len(first.Items), first.HasMore)
	}
	secondResp := authedActivityGet(t, session, base+"?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second activityfeed.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page = %d items, hasMore=%v; want 1/false", len(second.Items), second.HasMore)
	}
	for _, item := range slices.Concat(first.Items, second.Items) {
		if item.Action != string(activity.ActionVisitLogged) {
			t.Fatalf("page carried %q after a loss, want only visit_logged", item.Action)
		}
	}
}
