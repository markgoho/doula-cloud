package portal_test

import (
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestActivityHandler_ReadsOnlyWhatSheMaySee proves GLOSSARY.md's Activity
// entry at the reader, one case per rule:
//
//   - her own Engagement's rows reach her (#486 AC4/AC5);
//   - her money stays: a Client keeps every Contract and Invoice entry,
//     since this reader applies no money filter (unlike a contractor
//     Doula under ADR-0008's money tier);
//   - "never who inside the Practice did what": an Offer is which Doula
//     was asked, accepted or bumped, a Practice-roster fact;
//   - a Care Plan is "never shown in the Client portal" (#2066, and #2133
//     for each Plan's start), while a Birth Plan save, her own document,
//     stays;
//   - the Practice deliberating with itself (#1096) stays out, while
//     contract_voided, the outcome she reads, stays in;
//   - the three facts that lived in engagement_events (#1423, ADR-0015)
//     stay out, beside engagement_completed, which she already reads.
func TestActivityHandler_ReadsOnlyWhatSheMaySee(t *testing.T) {
	tests := []struct {
		name   string
		seeded []activity.EngagementAction
		want   []activity.EngagementAction
	}{
		{
			"her own Engagement's activity",
			[]activity.EngagementAction{activity.ActionContractSent},
			[]activity.EngagementAction{activity.ActionContractSent},
		},
		{
			"every money entry",
			[]activity.EngagementAction{activity.ActionInvoiceRaised},
			[]activity.EngagementAction{activity.ActionInvoiceRaised},
		},
		{
			"offer and reassignment hidden",
			[]activity.EngagementAction{activity.ActionVisitLogged, activity.ActionOfferSent, activity.ActionVisitReassigned},
			[]activity.EngagementAction{activity.ActionVisitLogged},
		},
		{
			"the Care Plan's start and save hidden",
			[]activity.EngagementAction{
				activity.ActionCarePlanStarted, activity.ActionCarePlanEdited,
				activity.ActionBirthPlanStarted, activity.ActionBirthPlanEdited,
			},
			[]activity.EngagementAction{activity.ActionBirthPlanEdited, activity.ActionBirthPlanStarted},
		},
		{
			"the void deliberation hidden",
			[]activity.EngagementAction{
				activity.ActionContractVoidRequested, activity.ActionContractVoidDeclined, activity.ActionContractVoided,
			},
			[]activity.EngagementAction{activity.ActionContractVoided},
		},
		{
			"the Engagement audit hidden",
			[]activity.EngagementAction{
				activity.ActionEngagementCompleted, activity.ActionEngagementReopened,
				activity.ActionKindChanged, activity.ActionBirthOutcomeRecorded,
			},
			[]activity.EngagementAction{activity.ActionEngagementCompleted},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.seedActivity(tt.seeded...)

			want := make([]string, 0, len(tt.want))
			for _, action := range tt.want {
				want = append(want, string(action))
			}
			slices.Sort(want)
			if got := e.activityActions(); !slices.Equal(got, want) {
				t.Fatalf("actions = %v, want %v", got, want)
			}
		})
	}
}

// TestActivityHandler_RedactsStaffActorNames proves GLOSSARY.md's Activity
// entry -- "she reads her own Activity ... never who inside the Practice
// did what" -- for the half staffingActionsNotIn's own action exclusion
// cannot cover: a Contract sent by a named Staff member is still hers to
// see (it is about her own Engagement), but who at the Practice sent it is
// not. A Client actor's own name (her signature) and the system actor's
// name are both left exactly as resolved -- only a Staff actor's own name
// is replaced.
func TestActivityHandler_RedactsStaffActorNames(t *testing.T) {
	e := newEnv(t)
	e.seedActivity(activity.ActionContractSent)
	testdb.SeedActivity(t, e.DB, e.Practice.ID, activity.SubjectEngagement, e.Engagement.ID, string(activity.ActionPortalInviteSent), activity.SystemActor())

	byAction := map[string]activityfeed.Entry{}
	for _, entry := range e.activityPage("").Items {
		byAction[entry.Action] = entry
	}
	if got := byAction[string(activity.ActionContractSent)]; got.ActorName != "Your practice" {
		t.Fatalf("staff actor name = %q, want the redacted \"Your practice\", not the Staff member's own name", got.ActorName)
	}
	if got := byAction[string(activity.ActionPortalInviteSent)]; got.ActorName != activity.SystemActorName {
		t.Fatalf("system actor name = %q, want %q unredacted", got.ActorName, activity.SystemActorName)
	}
}

// TestActivityHandler_PaginatesNewestFirst mirrors
// engagement.TestListActivityHandler_PaginatesNewestFirst.
func TestActivityHandler_PaginatesNewestFirst(t *testing.T) {
	e := newEnv(t)
	const total = 31 // activityPageSize (30) + 1, to force a second page
	for range total {
		e.seedActivity(activity.ActionVisitLogged)
	}

	requireFullPageThenLast(t, func(query string) pageInfo {
		page := e.activityPage(query)
		return pageInfo{len(page.Items), page.HasMore, page.NextCursor}
	})
}

// TestActivityHandler_InvalidCursorRejected mirrors
// engagement.TestListActivityHandler_InvalidCursorRejected.
func TestActivityHandler_InvalidCursorRejected(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.engagementURL("/activity?cursor=not!valid!base64!"), http.StatusBadRequest, e.AsClient)
}
