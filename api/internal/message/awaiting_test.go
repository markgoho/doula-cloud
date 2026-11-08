package message_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting proves the
// zero-data case renders an empty, terminal page rather than an error --
// #455's AC1, and the block's own zero-Client-message state.
func TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Empty Practice"))
	staff := fixture.Staff(t, db, p)

	srv, session := newServer(t, db, staff)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusOK)
	var got message.AwaitingReplyResponse
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
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
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Mixed Practice"))
	staff := fixture.Staff(t, db, p)

	waitingEngagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Priya"), fixture.ClientEmail("priya@example.com")))
	// #2111: the Overview's table sets every waiting Client side by side,
	// so it names her in full.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET family_name = 'Shah' WHERE id = $1`, waitingEngagement.Client.ID,
	); err != nil {
		t.Fatalf("seed family_name: %v", err)
	}
	seedMessage(t, db, waitingEngagement.ID, "staff", staff.ID, "How are you feeling?")
	seedMessage(t, db, waitingEngagement.ID, "client", waitingEngagement.Client.ID, "A little tired today.")

	repliedEngagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Renata"), fixture.ClientEmail("renata@example.com")))
	seedMessage(t, db, repliedEngagement.ID, "client", repliedEngagement.Client.ID, "Question about my plan.")
	seedMessage(t, db, repliedEngagement.ID, "staff", staff.ID, "Answered -- let me know if that helps.")

	srv, session := newServer(t, db, staff)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	var got message.AwaitingReplyResponse
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly the one Engagement still awaiting a reply", got.Items)
	}
	if got.Items[0].EngagementID != waitingEngagement.ID {
		t.Fatalf("EngagementID = %q, want %q", got.Items[0].EngagementID, waitingEngagement.ID)
	}
	if got.Items[0].ClientName != "Priya Shah" {
		t.Fatalf("ClientName = %q, want %q", got.Items[0].ClientName, "Priya Shah")
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
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Contractor Practice"))
	contractor := fixture.Staff(t, db, p, fixture.Contractor())

	attachedEngagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Attached Client"), fixture.ClientEmail("attached@example.com")), fixture.WithDoula(contractor))
	seedMessage(t, db, attachedEngagement.ID, "client", attachedEngagement.Client.ID, "Waiting on you.")

	unattachedEngagement := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Unattached Client"), fixture.ClientEmail("unattached@example.com")))
	seedMessage(t, db, unattachedEngagement.ID, "client", unattachedEngagement.Client.ID, "Also waiting.")

	srv, session := newServer(t, db, contractor)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
	defer resp.Body.Close()
	var got message.AwaitingReplyResponse
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].EngagementID != attachedEngagement.ID {
		t.Fatalf("Items = %+v, want exactly the one attached Engagement (no leak of the unattached one)", got.Items)
	}
}

// TestAwaitingReplyHandler_OwnerOrAdminContractorSeesTheWholePractice is
// #1635: the list narrows only for the contractor who holds neither the
// Owner role nor the Admin role (staffauth.Reader.IsAmbientContractor,
// the population ADR-0008 confines). Employment type is independent of
// roles (GLOSSARY.md), so an Owner or an Admin whose Membership reads
// "contractor" is real, and she reaches the whole Practice. She reads
// each Engagement whose thread awaits a reply, one she holds a granted
// Attachment on and one she does not. The same defect on the Clients
// list is #742.
func TestAwaitingReplyHandler_OwnerOrAdminContractorSeesTheWholePractice(t *testing.T) {
	db := testdb.New(t)

	cases := []struct {
		name     string
		roles    []string
		attached bool
	}{
		{"an Owner who is a contractor, with no Attachment", []string{ownerRole}, false},
		{"an Admin who is a contractor, with no Attachment", []string{adminRole}, false},
		{"an Owner who is a contractor Doula, attached to one Engagement", []string{ownerRole, doulaRole}, true},
		{"an Admin who is a contractor Doula, attached to one Engagement", []string{adminRole, doulaRole}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture.Practice(t, db)
			staff := fixture.Staff(t, db, p, fixture.Roles(tc.roles...), fixture.Contractor())

			first := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("First Client")))
			seedMessage(t, db, first.ID, "client", first.Client.ID, "Waiting on you.")
			if tc.attached {
				testdb.SeedGrantedAttachment(t, db, first.ID, staff.ID)
			}
			second := fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Second Client")))
			seedMessage(t, db, second.ID, "client", second.Client.ID, "Also waiting.")

			srv, session := newServer(t, db, staff)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
			defer resp.Body.Close()
			reqtest.RequireStatus(t, resp, http.StatusOK)
			var out message.AwaitingReplyResponse
			if err := json.UnmarshalRead(resp.Body, &out); err != nil {
				t.Fatalf("decode: %v", err)
			}

			got := map[string]bool{}
			for _, item := range out.Items {
				got[item.EngagementID] = true
			}
			if len(out.Items) != 2 || !got[first.ID] || !got[second.ID] {
				t.Fatalf("Items = %+v, want both Engagements that await a reply at the Practice -- an Owner or an Admin is not narrowed to her own Attachments", out.Items)
			}
		})
	}
}

// TestAwaitingReplyHandler_InvalidCursorRejected mirrors
// activityfeed.TestPracticeHandler_InvalidCursorRejected.
func TestAwaitingReplyHandler_InvalidCursorRejected(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Bad Cursor Practice"))
	staff := fixture.Staff(t, db, p)

	srv, session := newServer(t, db, staff)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply?cursor=not!valid!base64!")
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusBadRequest)
}

// TestAwaitingReplyHandler_PaginatesAcrossPages proves the "one request,
// no per-Engagement fan-out" AC holds across more Engagements than one
// page: a second, cursor-driven request reaches the rest, newest first,
// with no gap or duplicate at the boundary.
func TestAwaitingReplyHandler_PaginatesAcrossPages(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Pages Practice"))
	staff := fixture.Staff(t, db, p)

	const total = 35 // awaitingPageSize (30) + 5, to force a second page
	engagementIDs := make([]string, total)
	for i := range total {
		engagement := fixture.Engagement(t, db, p)
		seedMessage(t, db, engagement.ID, "client", engagement.Client.ID, "Waiting.")
		engagementIDs[i] = engagement.ID
	}

	srv, session := newServer(t, db, staff)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
	defer firstResp.Body.Close()
	var first message.AwaitingReplyResponse
	if err := json.UnmarshalRead(firstResp.Body, &first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second message.AwaitingReplyResponse
	if err := json.UnmarshalRead(secondResp.Body, &second); err != nil {
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
	p := fixture.Practice(t, db, fixture.PracticeName("Awaiting Contractor Pages Practice"))
	contractor := fixture.Staff(t, db, p, fixture.Contractor())

	const total = 35 // awaitingPageSize (30) + 5, to force a second page
	engagementIDs := make([]string, total)
	for i := range total {
		engagement := fixture.Engagement(t, db, p, fixture.WithDoula(contractor))
		seedMessage(t, db, engagement.ID, "client", engagement.Client.ID, "Waiting.")
		engagementIDs[i] = engagement.ID
	}

	srv, session := newServer(t, db, contractor)
	defer srv.Close()

	firstResp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply")
	defer firstResp.Body.Close()
	var first message.AwaitingReplyResponse
	if err := json.UnmarshalRead(firstResp.Body, &first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	secondResp := authedGet(t, session, srv.URL+"/api/practices/"+p.ID+"/messages/awaiting-reply?cursor="+*first.NextCursor)
	defer secondResp.Body.Close()
	var second message.AwaitingReplyResponse
	if err := json.UnmarshalRead(secondResp.Body, &second); err != nil {
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
