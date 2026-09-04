package activityfeed_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/testdb"
)

// TestPracticeHandler_SpansEverySubjectKindNewestFirst proves AC1: the
// practice-scoped feed reads across every subject kind (not just one, the
// way engagement.ListActivityHandler and client.DetailHandler's own
// mergedHistory each read one), newest first.
func TestPracticeHandler_SpansEverySubjectKindNewestFirst(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-spans-kinds"
	practiceID := testdb.SeedPractice(t, db, "Feed Spans Kinds Practice")
	ownerID := seedOwnerAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Feed Client", "feed-spans-kinds@example.com")

	seedActivity(t, db, practiceID, activity.SubjectClient, clientID, "created", activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionEngagementCreated), activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(ownerID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 3 {
		t.Fatalf("Items = %d, want 3 (one per subject kind written)", len(got.Items))
	}
	// Newest first: the last seeded row (visit_logged) leads.
	if got.Items[0].Action != string(activity.ActionVisitLogged) {
		t.Fatalf("Items[0].Action = %q, want %q (newest first)", got.Items[0].Action, activity.ActionVisitLogged)
	}
	if got.Items[0].SubjectKind != activity.SubjectEngagement || got.Items[0].SubjectID != engagementID {
		t.Fatalf("Items[0] subject = %s/%s, want %s/%s", got.Items[0].SubjectKind, got.Items[0].SubjectID, activity.SubjectEngagement, engagementID)
	}
	if got.Items[2].SubjectKind != activity.SubjectClient {
		t.Fatalf("Items[2].SubjectKind = %q, want %q (oldest last)", got.Items[2].SubjectKind, activity.SubjectClient)
	}
	if got.Items[0].ActorName != "" && got.Items[0].ActorKind != "staff" {
		t.Fatalf("ActorKind = %q, want staff", got.Items[0].ActorKind)
	}
}

// TestPracticeHandler_ResolvesActorNameForEveryActorKind proves the feed
// never leaves a reader to resolve a bare actor id herself, for all three
// of ADR-0022's actor kinds -- not only the Staff-actor case the other
// tests above already exercise.
func TestPracticeHandler_ResolvesActorNameForEveryActorKind(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-actor-names"
	practiceID := testdb.SeedPractice(t, db, "Feed Actor Names Practice")
	seedOwnerAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Amara", "feed-actor-names@example.com")

	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionContractSigned), activity.ClientActor(clientID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionOfferSent), activity.SystemActor())

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("Items = %+v, want 2", got.Items)
	}
	byAction := map[string]activityfeed.Entry{}
	for _, entry := range got.Items {
		byAction[entry.Action] = entry
	}
	if got := byAction[string(activity.ActionContractSigned)]; got.ActorKind != "client" || got.ActorName != "Amara" {
		t.Fatalf("client actor = %+v, want kind=client name=Amara", got)
	}
	if got := byAction[string(activity.ActionOfferSent)]; got.ActorKind != "system" || got.ActorName != activity.SystemActorName {
		t.Fatalf("system actor = %+v, want kind=system name=%q", got, activity.SystemActorName)
	}
}

// TestPracticeHandler_BatchOverflowStillPaginatesCorrectly forces
// practiceBatchSize's own "+1 sentinel" (fetchPage's doc comment) rather
// than only practicePageSize's: enough rows that a single raw batch query
// cannot hold them all, so hasMore must come from the sentinel row rather
// than from anything examined in-loop.
func TestPracticeHandler_BatchOverflowStillPaginatesCorrectly(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-batch-overflow"
	practiceID := testdb.SeedPractice(t, db, "Feed Batch Overflow Practice")
	ownerID := seedOwnerAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "feed-batch-overflow@example.com")

	const total = 125 // > practiceBatchSize (120), to force the sentinel row
	for range total {
		seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(ownerID))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	seen := 0
	cursor := ""
	for page := range 10 {
		url := srv.URL + "/practices/" + practiceID + "/activity"
		if cursor != "" {
			url += "?cursor=" + cursor
		}
		resp := authedGet(t, session, url)
		var got activityfeed.ListResponse
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode page %d: %v", page, err)
		}
		_ = resp.Body.Close()
		seen += len(got.Items)
		if !got.HasMore {
			if seen != total {
				t.Fatalf("terminal page reached after %d items, want %d", seen, total)
			}
			return
		}
		if got.NextCursor == nil {
			t.Fatalf("page %d: hasMore=true but NextCursor is nil", page)
		}
		cursor = *got.NextCursor
	}
	t.Fatalf("did not reach a terminal page within 10 requests; saw %d of %d items", seen, total)
}

// TestPracticeHandler_InvalidCursorRejected mirrors
// engagement.TestListActivityHandler_InvalidCursorRejected.
func TestPracticeHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-bad-cursor"
	practiceID := testdb.SeedPractice(t, db, "Feed Bad Cursor Practice")
	seedOwnerAtPractice(t, db, practiceID, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity?cursor=not!valid!base64!")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPracticeHandler_EmptyPracticeHasNoActivity proves the zero-data
// case renders an empty page rather than an error -- the hub's own
// "nothing has happened yet" state.
func TestPracticeHandler_EmptyPracticeHasNoActivity(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-empty"
	practiceID := testdb.SeedPractice(t, db, "Feed Empty Practice")
	seedOwnerAtPractice(t, db, practiceID, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 0 || got.HasMore || got.NextCursor != nil {
		t.Fatalf("got %+v, want an empty, terminal page", got)
	}
}

// TestPracticeHandler_UnregisteredSubjectKindNeverAppears proves the
// other half of AC2 outside a page boundary: a subject kind #485's
// registry has no Rule for (client_field_template, per activitygate's own
// doc comment) is refused by the gate itself, not by any filter this
// package adds -- the row simply never survives fetchPage's loop.
func TestPracticeHandler_UnregisteredSubjectKindNeverAppears(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-feed-unregistered-kind"
	practiceID := testdb.SeedPractice(t, db, "Feed Unregistered Kind Practice")
	ownerID := seedOwnerAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Feed Client", "feed-unregistered-kind@example.com")

	seedActivity(t, db, practiceID, "client_field_template", practiceID, "template_saved", activity.StaffActor(ownerID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionEngagementCreated), activity.StaffActor(ownerID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].SubjectKind != activity.SubjectEngagement {
		t.Fatalf("Items = %+v, want exactly one engagement row (client_field_template unregistered)", got.Items)
	}
}

// TestPracticeHandler_MoneyTierAppliedPerRow proves ADR-0008's money tier
// is enforced row by row in the cross-subject feed (via
// activitygate.CanSeeAction), not only by engagement.ListActivityHandler's
// own single-subject SQL exclusion: an employed Doula sees an ordinary
// Engagement event but never an Invoice one.
func TestPracticeHandler_MoneyTierAppliedPerRow(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "doula-feed-money-tier"
	practiceID := testdb.SeedPractice(t, db, "Feed Money Tier Practice")
	doulaID := seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Feed Client", "feed-money-tier@example.com")

	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionVisitLogged), activity.StaffActor(doulaID))
	seedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, string(activity.ActionInvoiceRaised), activity.StaffActor(doulaID))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer resp.Body.Close()
	var got activityfeed.ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Action != string(activity.ActionVisitLogged) {
		t.Fatalf("Items = %+v, want exactly the non-money visit_logged row", got.Items)
	}
}

// TestPracticeHandler_ContractorPageBoundaryNeverLeaksForbiddenRows is
// AC2's own proof: "a test proves a subject kind a reader may not see
// never appears, including on a page boundary." A contractor attached to
// one Engagement only pages through a feed mixing her own allowed rows
// with another Engagement's forbidden ones and an unregistered subject
// kind's rows -- interleaved so the raw batch's own page cut (30 allowed
// items collected) does not necessarily land on an allowed row. Every
// row is given a distinct synthetic action so the union of both pages can
// be checked for an exact set match: no forbidden row, no duplicate, no
// skipped allowed row.
func TestPracticeHandler_ContractorPageBoundaryNeverLeaksForbiddenRows(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-feed-page-boundary"
	practiceID := testdb.SeedPractice(t, db, "Feed Page Boundary Practice")
	contractorID := seedContractorAtPractice(t, db, practiceID, contractorUID)
	_, allowedEngagementID := seedClientEngagement(t, db, practiceID, "Allowed Client", "feed-boundary-allowed@example.com")
	_, forbiddenEngagementID := seedClientEngagement(t, db, practiceID, "Forbidden Client", "feed-boundary-forbidden@example.com")
	seedGrantedAttachment(t, db, allowedEngagementID, contractorID)

	// Interleaved, oldest to newest: two allowed rows, then one not-allowed
	// row (alternating forbidden-Engagement and unregistered-kind),
	// repeated, with a further run of not-allowed rows at the very end.
	// The feed itself reads newest-first, so this same sequence comes back
	// reversed -- which puts several not-allowed rows at the very head of
	// page 1 (many raw rows examined before the first is collected) and
	// lands the exact 30-allowed-item cut mid-run rather than cleanly on
	// an allowed row, the case #486 AC2 actually asks a test to prove:
	// "including on a page boundary." Seeding every not-allowed row at one
	// end (as an earlier version of this test did) never exercises either.
	const allowedTotal = 35 // activityPageSize (30) + 5, to force a second page
	wantAllowed := map[string]bool{}
	allowedSeeded, notAllowedSeeded := 0, 0
	seedNotAllowed := func() {
		if notAllowedSeeded%2 == 0 {
			seedActivity(t, db, practiceID, activity.SubjectEngagement, forbiddenEngagementID, fmt.Sprintf("forbidden_visit_%d", notAllowedSeeded), activity.StaffActor(contractorID))
		} else {
			seedActivity(t, db, practiceID, "client_field_template", practiceID, fmt.Sprintf("hidden_template_%d", notAllowedSeeded), activity.StaffActor(contractorID))
		}
		notAllowedSeeded++
	}
	for allowedSeeded < allowedTotal {
		for range 2 {
			if allowedSeeded == allowedTotal {
				break
			}
			action := fmt.Sprintf("allowed_visit_%d", allowedSeeded)
			seedActivity(t, db, practiceID, activity.SubjectEngagement, allowedEngagementID, action, activity.StaffActor(contractorID))
			wantAllowed[action] = true
			allowedSeeded++
		}
		seedNotAllowed()
	}
	for range 5 {
		seedNotAllowed()
	}

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity")
	defer firstResp.Body.Close()
	var first activityfeed.ListResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/activity?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second activityfeed.ListResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page hasMore=%v, cursor=%v; want false/nil", second.HasMore, second.NextCursor)
	}

	got := map[string]bool{}
	for _, entry := range append(first.Items, second.Items...) {
		if entry.SubjectID != allowedEngagementID {
			t.Fatalf("forbidden row leaked: %+v", entry)
		}
		if got[entry.Action] {
			t.Fatalf("duplicate row across pages: %+v", entry)
		}
		got[entry.Action] = true
	}
	if len(got) != allowedTotal {
		t.Fatalf("union of both pages has %d allowed rows, want %d: %v", len(got), allowedTotal, got)
	}
	for action := range wantAllowed {
		if !got[action] {
			t.Fatalf("allowed row %q missing from both pages combined", action)
		}
	}
}
