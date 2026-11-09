package message_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// awaiting reads the Practice-wide roll-up of threads awaiting a reply as
// the person at the screen.
func (e *env) awaiting(query string) message.AwaitingReplyResponse {
	e.T.Helper()
	return reqtest.Expect[message.AwaitingReplyResponse](e.T, http.MethodGet, e.PracticeURL("/messages/awaiting-reply"+query), http.StatusOK, e.SignedIn)
}

// waitingEngagement is an Engagement whose thread's latest Message came
// from its Client.
func (e *env) waitingEngagement(body string, opts ...fixture.EngagementOption) fixture.EngagementRecord {
	e.T.Helper()
	engagement := e.AnotherEngagement(opts...)
	seedMessage(e.T, e.DB, engagement.ID, "client", engagement.Client.ID, body)
	return engagement
}

// TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting proves the
// zero-data case renders an empty, terminal page rather than an error --
// #455's AC1, and the block's own zero-Client-message state.
func TestAwaitingReplyHandler_EmptyPracticeHasNoEngagementsWaiting(t *testing.T) {
	e := newEnv(t)

	got := e.awaiting("")
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
	e := newEnv(t)

	waiting := e.AnotherEngagement(fixture.NewClient(fixture.ClientName("Priya"), fixture.ClientEmail("priya@example.com")))
	// #2111: the Overview's table sets every waiting Client side by side,
	// so it names her in full.
	e.Exec(`UPDATE clients SET family_name = 'Shah' WHERE id = $1`, waiting.Client.ID)
	seedMessage(t, e.DB, waiting.ID, "staff", e.Staff.ID, "How are you feeling?")
	seedMessage(t, e.DB, waiting.ID, "client", waiting.Client.ID, "A little tired today.")

	replied := e.waitingEngagement("Question about my plan.", fixture.NewClient(fixture.ClientName("Renata"), fixture.ClientEmail("renata@example.com")))
	seedMessage(t, e.DB, replied.ID, "staff", e.Staff.ID, "Answered -- let me know if that helps.")

	got := e.awaiting("")
	if len(got.Items) != 1 {
		t.Fatalf("Items = %+v, want exactly the one Engagement still awaiting a reply", got.Items)
	}
	if got.Items[0].EngagementID != waiting.ID {
		t.Fatalf("EngagementID = %q, want %q", got.Items[0].EngagementID, waiting.ID)
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
	e := newEnv(t, signedInAs(fixture.Contractor()))
	attached := e.waitingEngagement("Waiting on you.", fixture.WithDoula(e.Staff))
	e.waitingEngagement("Also waiting.")

	got := e.awaiting("")
	if len(got.Items) != 1 || got.Items[0].EngagementID != attached.ID {
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
			e := newEnv(t, signedInAs(fixture.Roles(tc.roles...), fixture.Contractor()))
			first := e.waitingEngagement("Waiting on you.")
			if tc.attached {
				testdb.SeedGrantedAttachment(t, e.DB, first.ID, e.Staff.ID)
			}
			second := e.waitingEngagement("Also waiting.")

			out := e.awaiting("")

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
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.PracticeURL("/messages/awaiting-reply?cursor=not!valid!base64!"), http.StatusBadRequest, e.SignedIn)
}

// TestAwaitingReplyHandler_PaginatesAcrossPages proves the "one request,
// no per-Engagement fan-out" AC holds across more Engagements than one
// page: a second, cursor-driven request reaches the rest, newest first,
// with no gap or duplicate at the boundary. For a contractor it also
// exercises the attached-Engagements query's own cursor branch
// (listAttachedAwaitingReply's `after != nil` path).
func TestAwaitingReplyHandler_PaginatesAcrossPages(t *testing.T) {
	for _, tc := range []struct {
		name       string
		contractor bool
	}{
		{"every Engagement at the Practice", false},
		{"a contractor's attached Engagements", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts []envOption
			if tc.contractor {
				opts = append(opts, signedInAs(fixture.Contractor()))
			}
			e := newEnv(t, opts...)

			const total = 35 // awaitingPageSize (30) + 5, to force a second page
			engagementIDs := make([]string, total)
			for i := range total {
				var engagementOpts []fixture.EngagementOption
				if tc.contractor {
					engagementOpts = append(engagementOpts, fixture.WithDoula(e.Staff))
				}
				engagementIDs[i] = e.waitingEngagement("Waiting.", engagementOpts...).ID
			}

			first := e.awaiting("")
			if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
				t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
					len(first.Items), first.HasMore, first.NextCursor)
			}
			second := e.awaiting("?cursor=" + *first.NextCursor)
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
		})
	}
}
