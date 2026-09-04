package message_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/testdb"
)

// TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting proves the
// zero-data case renders an empty, terminal page rather than an error --
// #455's AC1, and the block's own zero-Client-message state.
func TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-awaiting-empty"
	practiceID := seedPractice(t, db, "Awaiting Empty Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got message.AwaitingReplyResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 0 || got.HasMore || got.NextCursor != nil {
		t.Fatalf("got %+v, want an empty, terminal page", got)
	}
}

// TestAwaitingReplyHandler_OnlyEngagementsAwaitingReplyAppear proves the
// crux of #455: computed from thread authorship. An Engagement whose
// thread's latest Message came from the Client appears; a sibling
// Engagement where staff already replied after the Client's own last word
// does not, even though the Client wrote there too.
func TestAwaitingReplyHandler_OnlyEngagementsAwaitingReplyAppear(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-awaiting-mixed"
	practiceID := seedPractice(t, db, "Awaiting Mixed Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, identityUID)

	waitingClientID, waitingEngagementID := seedClientEngagement(t, db, practiceID, "Priya", "priya@example.com")
	seedMessage(t, db, waitingEngagementID, "staff", staffID, "How are you feeling?")
	seedMessage(t, db, waitingEngagementID, "client", waitingClientID, "A little tired today.")

	repliedClientID, repliedEngagementID := seedClientEngagement(t, db, practiceID, "Renata", "renata@example.com")
	seedMessage(t, db, repliedEngagementID, "client", repliedClientID, "Question about my plan.")
	seedMessage(t, db, repliedEngagementID, "staff", staffID, "Answered -- let me know if that helps.")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	var got message.AwaitingReplyResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly the one Engagement still awaiting a reply", got.Items)
	}
	if got.Items[0].EngagementID != waitingEngagementID {
		t.Fatalf("EngagementID = %q, want %q", got.Items[0].EngagementID, waitingEngagementID)
	}
	if got.Items[0].ClientName != "Priya" {
		t.Fatalf("ClientName = %q, want %q", got.Items[0].ClientName, "Priya")
	}
}

// TestAwaitingReplyHandler_ContractorSeesOnlyHerAttachedEngagements is
// #455's own security boundary: "a contractor doula cannot see an entry
// for an Engagement she is not attached to" -- the same
// staffauth.Reader.CanAccessEngagement narrowing message.ListHandler
// already applies, proven here at the Practice-wide roll-up rather than a
// single Engagement's thread.
func TestAwaitingReplyHandler_ContractorSeesOnlyHerAttachedEngagements(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-awaiting-narrowed"
	practiceID := seedPractice(t, db, "Awaiting Contractor Practice")
	contractorID := seedContractorAtPractice(t, db, practiceID, contractorUID)

	attachedClientID, attachedEngagementID := seedClientEngagement(t, db, practiceID, "Attached Client", "attached@example.com")
	seedGrantedAttachment(t, db, attachedEngagementID, contractorID)
	seedMessage(t, db, attachedEngagementID, "client", attachedClientID, "Waiting on you.")

	unattachedClientID, unattachedEngagementID := seedClientEngagement(t, db, practiceID, "Unattached Client", "unattached@example.com")
	seedMessage(t, db, unattachedEngagementID, "client", unattachedClientID, "Also waiting.")

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	var got message.AwaitingReplyResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].EngagementID != attachedEngagementID {
		t.Fatalf("Items = %+v, want exactly the one attached Engagement (no leak of the unattached one)", got.Items)
	}
}

// TestAwaitingReplyHandler_InvalidCursorRejected mirrors
// activityfeed.TestPracticeHandler_InvalidCursorRejected.
func TestAwaitingReplyHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-awaiting-bad-cursor"
	practiceID := seedPractice(t, db, "Awaiting Bad Cursor Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply?cursor=not!valid!base64!")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestAwaitingReplyHandler_PaginatesAcrossPages proves the "one request,
// no per-Engagement fan-out" AC holds across more Engagements than one
// page: a second, cursor-driven request reaches the rest, newest first,
// with no gap or duplicate at the boundary.
func TestAwaitingReplyHandler_PaginatesAcrossPages(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-awaiting-pages"
	practiceID := seedPractice(t, db, "Awaiting Pages Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)

	const total = 35 // awaitingPageSize (30) + 5, to force a second page
	engagementIDs := make([]string, total)
	for i := range total {
		clientID, engagementID := seedClientEngagement(t, db, practiceID, fmt.Sprintf("Client %d", i), fmt.Sprintf("client-%d@example.com", i))
		seedMessage(t, db, engagementID, "client", clientID, "Waiting.")
		engagementIDs[i] = engagementID
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply")
	defer firstResp.Body.Close()
	var first message.AwaitingReplyResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second message.AwaitingReplyResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 5 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 5/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}

	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		if seen[item.EngagementID] {
			t.Fatalf("duplicate Engagement across pages: %s", item.EngagementID)
		}
		seen[item.EngagementID] = true
	}
	for _, id := range engagementIDs {
		if !seen[id] {
			t.Fatalf("Engagement %s missing from both pages combined", id)
		}
	}
}

// TestAwaitingReplyHandler_ContractorPaginatesAcrossPages exercises the
// attached-Engagements query's own cursor branch (listAttachedAwaitingReply's
// `after != nil` path), the contractor mirror of
// TestAwaitingReplyHandler_PaginatesAcrossPages above.
func TestAwaitingReplyHandler_ContractorPaginatesAcrossPages(t *testing.T) {
	db := testdb.New(t)
	const contractorUID = "contractor-awaiting-pages"
	practiceID := seedPractice(t, db, "Awaiting Contractor Pages Practice")
	contractorID := seedContractorAtPractice(t, db, practiceID, contractorUID)

	const total = 35 // awaitingPageSize (30) + 5, to force a second page
	engagementIDs := make([]string, total)
	for i := range total {
		clientID, engagementID := seedClientEngagement(t, db, practiceID, fmt.Sprintf("Attached Client %d", i), fmt.Sprintf("attached-%d@example.com", i))
		seedGrantedAttachment(t, db, engagementID, contractorID)
		seedMessage(t, db, engagementID, "client", clientID, "Waiting.")
		engagementIDs[i] = engagementID
	}

	srv, session := newServer(t, db, contractorUID)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply")
	defer firstResp.Body.Close()
	var first message.AwaitingReplyResponse
	if err := json.NewDecoder(firstResp.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/practices/"+practiceID+"/messages/awaiting-reply?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second message.AwaitingReplyResponse
	if err := json.NewDecoder(secondResp.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 5 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 5/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}

	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		if seen[item.EngagementID] {
			t.Fatalf("duplicate Engagement across pages: %s", item.EngagementID)
		}
		seen[item.EngagementID] = true
	}
	for _, id := range engagementIDs {
		if !seen[id] {
			t.Fatalf("Engagement %s missing from both pages combined", id)
		}
	}
}
