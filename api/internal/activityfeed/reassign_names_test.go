package activityfeed_test

import (
	"encoding/json/jsontext"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/testdb"
)

func reassignment(fromID, toID string) jsontext.Value {
	return jsontext.Value(`{"assignedStaffIdBefore":"` + fromID + `","assignedStaffIdAfter":"` + toID + `"}`)
}

func practiceFeed(t *testing.T, db *testdb.DB, uid, practiceID string) []activityfeed.Entry {
	t.Helper()
	return readFeed(t, db, uid, practiceID).Items
}

// #2672: a visit_reassigned row names the Doula the Visit left and the
// one it went to, read as the Engagement ledger reads them: a colleague
// who has left still reads by name, a deleted login reads as a former
// colleague, and no other row carries either name.
func TestPracticeHandler_ReassignmentRowsNameBothDoulas(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-reassign-names"
	practiceID := testdb.SeedPractice(t, db, "Feed Reassign Names Practice")
	ownerID := testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole}, employeeType)
	priyaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-priya", priyaRaman, []string{doulaRole}, employeeType)
	deliaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-delia", "Delia Marchetti", []string{doulaRole}, employeeType)
	erasedID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-erased", renataAlvarez, []string{doulaRole}, employeeType)
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Feed Client", "feed-reassign-names@example.com", "active")
	actor := activity.StaffActor(ownerID)
	action := string(activity.ActionVisitReassigned)

	for _, id := range []string{deliaID, erasedID} {
		testdb.SeedActivity(t, db, practiceID, activity.SubjectMembership, id, string(activity.ActionMembershipRemoved), activity.StaffActor(id))
		testdb.RemoveMembership(t, db, id)
	}
	testdb.RedactDeletedLogin(t, db, erasedID)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, action, actor, reassignment(deliaID, priyaID))
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, engagementID, action, actor, reassignment(erasedID, priyaID))
	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), actor)

	items := practiceFeed(t, db, identityUID, practiceID)
	if len(items) != 5 {
		t.Fatalf("Items = %d, want 5", len(items))
	}
	logged, fromErased, fromDelia := items[0], items[1], items[2]
	if logged.ReassignedFromName != "" || logged.ReassignedToName != "" {
		t.Errorf("visit_logged row names %q / %q, want neither", logged.ReassignedFromName, logged.ReassignedToName)
	}
	if fromErased.ReassignedFromName != activity.DepartedStaffName || fromErased.ReassignedToName != priyaRaman {
		t.Errorf("erased row = %q to %q, want %q to Priya Raman", fromErased.ReassignedFromName, fromErased.ReassignedToName, activity.DepartedStaffName)
	}
	if fromDelia.ReassignedFromName != "Delia Marchetti" || fromDelia.ReassignedToName != priyaRaman {
		t.Errorf("departed row = %q to %q, want Delia Marchetti to Priya Raman", fromDelia.ReassignedFromName, fromDelia.ReassignedToName)
	}
}

// A contractor reads the names on a Visit of an Engagement she is
// attached to, as the Engagement page's own ledger already shows her, and
// no row at all on one she is not.
func TestPracticeHandler_ContractorReadsReassignmentNamesOnlyOnHerOwnEngagement(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-feed-reassign-names"
	practiceID := testdb.SeedPractice(t, db, "Feed Contractor Reassign Practice")
	contractorID := testdb.SeedContractorAtPractice(t, db, practiceID, contractorUID)
	priyaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "doula-feed-c-priya", priyaRaman, []string{doulaRole}, employeeType)
	_, hers := testdb.SeedEngagementInStatus(t, db, practiceID, "Her Client", "feed-c-hers@example.com", "active")
	_, other := testdb.SeedEngagementInStatus(t, db, practiceID, "Other Client", "feed-c-other@example.com", "active")
	testdb.SeedGrantedAttachment(t, db, hers, contractorID)
	actor := activity.StaffActor(priyaID)
	action := string(activity.ActionVisitReassigned)

	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, other, action, actor, reassignment(contractorID, priyaID))
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectEngagement, hers, action, actor, reassignment(priyaID, contractorID))

	items := practiceFeed(t, db, contractorUID, practiceID)
	if len(items) != 1 || items[0].SubjectID != hers {
		t.Fatalf("Items = %+v, want only the row on her own Engagement", items)
	}
	if items[0].ReassignedFromName != priyaRaman || items[0].ReassignedToName == "" || items[0].ReassignedToName == activity.DepartedStaffName {
		t.Errorf("row = %q to %q, want Priya Raman to her own name", items[0].ReassignedFromName, items[0].ReassignedToName)
	}
}
