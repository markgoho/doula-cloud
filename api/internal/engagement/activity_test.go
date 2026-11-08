package engagement_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// seedActivity writes one entry on the Engagement's ledger.
func (e *env) seedActivity(action activity.EngagementAction, actor activity.Actor) {
	e.t.Helper()
	testdb.SeedActivity(e.t, e.db, e.Practice.ID, activity.SubjectEngagement, e.Engagement.ID, string(action), actor)
}

// seedActivityWithDiff is seedActivity carrying diff.
func (e *env) seedActivityWithDiff(action activity.EngagementAction, actor activity.Actor, diff jsontext.Value) {
	e.t.Helper()
	testdb.SeedActivityWithDiff(e.t, e.db, e.Practice.ID, activity.SubjectEngagement, e.Engagement.ID, string(action), actor, diff)
}

// activityAs reads the Engagement's ledger as who.
func (e *env) activityAs(who fixture.StaffRecord) engagement.ActivityListResponse {
	e.t.Helper()
	return reqtest.Expect[engagement.ActivityListResponse](e.t, http.MethodGet, e.engagementURL("/activity"), http.StatusOK, e.as(who))
}

// actionsSeen is the set of actions the ledger shows who.
func (e *env) actionsSeen(who fixture.StaffRecord) map[string]bool {
	e.t.Helper()
	seen := map[string]bool{}
	for _, item := range e.activityAs(who).Items {
		seen[item.Action] = true
	}
	return seen
}

// TestListActivityHandler_WhoSeesWhichEntries is ADR-0008's read table
// for the ledger, as amended by #282: an Owner and an employee Doula see
// every entry, money included, since employment type -- not role -- is the
// boundary; a contractor sees neither Invoice/payment nor the Practice's
// Contract price, per "her own agreed fee only ... never the Practice's
// price", while her own Offer acceptance and the Contract's own lifecycle
// (#972: no longer in the money set) stay visible. Staff still see a
// void request and its refusal: this is a Client-portal filter, not a
// Staff one (#1096). And a Birth Plan row stays on the Practice's own
// ledger after a loss, which the portal leaves out (#2219).
func TestListActivityHandler_WhoSeesWhichEntries(t *testing.T) {
	staff, client, system := "staff", "client", "system"
	type entry struct {
		action activity.EngagementAction
		by     string
	}
	cases := []struct {
		name      string
		viewer    []fixture.StaffOption
		attached  bool
		seeded    []entry
		afterLoss bool
		want      []activity.EngagementAction
	}{
		{
			name:   "an Owner sees every entry, money included",
			viewer: []fixture.StaffOption{fixture.Owner()},
			seeded: []entry{{activity.ActionEngagementCreated, staff}, {activity.ActionInvoiceRaised, staff}, {activity.ActionInvoicePaid, client}},
			want:   []activity.EngagementAction{activity.ActionEngagementCreated, activity.ActionInvoiceRaised, activity.ActionInvoicePaid},
		},
		{
			name:   "an employee Doula sees money entries the same as an Owner",
			viewer: []fixture.StaffOption{fixture.Doula()},
			seeded: []entry{
				{activity.ActionEngagementCreated, staff}, {activity.ActionVisitLogged, staff}, {activity.ActionInvoiceRaised, staff},
				{activity.ActionInvoicePaid, system}, {activity.ActionContractSent, staff}, {activity.ActionContractPriced, staff},
			},
			want: []activity.EngagementAction{
				activity.ActionEngagementCreated, activity.ActionVisitLogged, activity.ActionInvoiceRaised,
				activity.ActionInvoicePaid, activity.ActionContractSent, activity.ActionContractPriced,
			},
		},
		{
			name:     "a contractor sees no Invoice, payment or Practice price",
			viewer:   []fixture.StaffOption{fixture.Contractor()},
			attached: true,
			seeded: []entry{
				{activity.ActionOfferAccepted, staff}, {activity.ActionContractSigned, client},
				{activity.ActionContractPriced, staff}, {activity.ActionInvoicePaid, system},
			},
			want: []activity.EngagementAction{activity.ActionOfferAccepted, activity.ActionContractSigned},
		},
		{
			name:   "an Owner sees a void request and its refusal",
			viewer: []fixture.StaffOption{fixture.Owner()},
			seeded: []entry{{activity.ActionContractVoidRequested, staff}, {activity.ActionContractVoidDeclined, staff}},
			want:   []activity.EngagementAction{activity.ActionContractVoidRequested, activity.ActionContractVoidDeclined},
		},
		{
			name:      "an Owner sees every Birth Plan row after a loss",
			viewer:    []fixture.StaffOption{fixture.Owner()},
			afterLoss: true,
			seeded: func() []entry {
				var out []entry
				for _, a := range activity.BirthPlanActions() {
					out = append(out, entry{a, staff})
				}
				return out
			}(),
			want: activity.BirthPlanActions(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.viewer...), withEngagement(fixture.Status(fixture.StatusActive)))
			if tc.attached {
				testdb.SeedGrantedAttachment(t, e.db, e.Engagement.ID, e.Staff.ID)
			}
			if tc.afterLoss {
				e.exec(`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`, e.Engagement.ID)
			}
			for _, s := range tc.seeded {
				actor := activity.StaffActor(e.Staff.ID)
				switch s.by {
				case client:
					actor = activity.ClientActor(e.Engagement.Client.ID)
				case system:
					actor = activity.SystemActor()
				}
				e.seedActivity(s.action, actor)
			}

			got := e.actionsSeen(e.Staff)

			if len(got) != len(tc.want) {
				t.Fatalf("saw %v, want exactly %v", got, tc.want)
			}
			for _, a := range tc.want {
				if !got[string(a)] {
					t.Fatalf("saw %v, want exactly %v", got, tc.want)
				}
			}
		})
	}
}

// TestListActivityHandler_SystemActorRendersAsDoulaCloud asserts
// ADR-0022's exact display string -- never "System".
func TestListActivityHandler_SystemActorRendersAsDoulaCloud(t *testing.T) {
	e := newOwnerEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	e.seedActivity(activity.ActionPortalInviteSent, activity.SystemActor())

	got := e.activityAs(e.Staff)

	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	if got.Items[0].ActorName != "DoulaCloud" {
		t.Fatalf("system actor ActorName = %q, want %q", got.Items[0].ActorName, "DoulaCloud")
	}
}

// TestListActivityHandler_RefusesWhatItCannotServe covers the refusals
// of the ledger read: an engagement id that is not a uuid mirrors
// DetailHandler's own handling, a cursor that is not a cursor mirrors
// visit.TestListHandler_InvalidCursorRejected, and a contractor with no
// open, granted attachment gets a 404, the same as the Engagement detail
// read, mirroring visit.ListHandler's gate.
func TestListActivityHandler_RefusesWhatItCannotServe(t *testing.T) {
	cases := []struct {
		name   string
		viewer []fixture.StaffOption
		path   func(e *env) string
		want   int
	}{
		{"an engagement id that is not a uuid", []fixture.StaffOption{fixture.Owner()}, func(e *env) string { return e.urlOf("not-a-uuid", "/activity") }, http.StatusBadRequest},
		{"a cursor that is not valid", []fixture.StaffOption{fixture.Owner()}, func(e *env) string { return e.engagementURL("/activity?cursor=not!valid!base64!") }, http.StatusBadRequest},
		{"a contractor not attached to it", []fixture.StaffOption{fixture.Contractor()}, func(e *env) string { return e.engagementURL("/activity") }, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.viewer...), withEngagement(fixture.Status(fixture.StatusActive)))

			reqtest.ExpectStatus(t, http.MethodGet, tc.path(e), tc.want, e.signedIn)
		})
	}
}

// TestListActivityHandler_PaginatesNewestFirst mirrors
// visit.TestListHandler_PaginatesNewestFirst.
func TestListActivityHandler_PaginatesNewestFirst(t *testing.T) {
	e := newOwnerEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	const total = 31 // activityPageSize (30) + 1, to force a second page
	for range total {
		e.seedActivity(activity.ActionVisitLogged, activity.StaffActor(e.Staff.ID))
	}

	first := e.activityAs(e.Staff)
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
			len(first.Items), first.HasMore, first.NextCursor)
	}

	second := reqtest.Expect[engagement.ActivityListResponse](t, http.MethodGet, e.engagementURL("/activity?cursor="+*first.NextCursor), http.StatusOK, e.signedIn)
	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
			len(second.Items), second.HasMore, second.NextCursor)
	}
}

// reassignDiff is the diff a reassignment writes, built through the same
// constants the write side uses so this fixture cannot drift from it.
func reassignDiff(t *testing.T, before, after string) jsontext.Value {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		activity.DiffKeyAssignedStaffIDBefore: before,
		activity.DiffKeyAssignedStaffIDAfter:  after,
	})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly
		t.Fatalf("marshal diff: %v", err)
	}
	return raw
}

func attachedDiff(t *testing.T, staffID string) jsontext.Value {
	t.Helper()
	raw, err := json.Marshal(map[string]string{activity.DiffKeyAttachedStaffID: staffID})
	if err != nil {
		// coverage:ignore reason: a map of strings always marshals cleanly
		t.Fatalf("marshal diff: %v", err)
	}
	return raw
}

// TestListActivityHandler_DetailNamesPeopleOutOfTheDiff is #887's, #1596's
// and #2271's read side: the ledger reads the ids out of an entry's diff
// and resolves them to names server-side, the way actorName is already
// resolved, so a reader is told which Doula a Visit left and reached, who
// was attached and by whom, and whose fee was paid -- and when one end no
// longer resolves to a Staff row it is told a colleague has gone, never
// shown a bare uuid.
func TestListActivityHandler_DetailNamesPeopleOutOfTheDiff(t *testing.T) {
	const departedID = "11111111-2222-3333-4444-555555555555"
	cases := []struct {
		name   string
		action activity.EngagementAction
		diff   func(t *testing.T, from, to fixture.StaffRecord) jsontext.Value
		want   string
	}{
		{"a reassignment names both people", activity.ActionVisitReassigned,
			func(t *testing.T, from, to fixture.StaffRecord) jsontext.Value {
				return reassignDiff(t, from.ID, to.ID)
			},
			"Visit reassigned from Hana Kim to Mei Tan"},
		{"a reassignment from somebody gone", activity.ActionVisitReassigned,
			func(t *testing.T, _, to fixture.StaffRecord) jsontext.Value {
				return reassignDiff(t, departedID, to.ID)
			},
			"Visit reassigned from a former colleague to Mei Tan"},
		{"an attachment names the Doula", activity.ActionDoulaAttached,
			func(t *testing.T, from, _ fixture.StaffRecord) jsontext.Value { return attachedDiff(t, from.ID) },
			"Put on this work as the Doula: Hana Kim"},
		{"an attachment of somebody gone", activity.ActionDoulaAttached,
			func(t *testing.T, _, _ fixture.StaffRecord) jsontext.Value { return attachedDiff(t, departedID) },
			"Put on this work as the Doula: a former colleague"},
		{"a fee payment names whose fee it was", activity.ActionContractorFeePaid,
			func(t *testing.T, from, _ fixture.StaffRecord) jsontext.Value { return attachedDiff(t, from.ID) },
			"Fee paid to Hana Kim"},
		{"a fee payment reversal names whose fee it was", activity.ActionContractorFeePaymentReversed,
			func(t *testing.T, from, _ fixture.StaffRecord) jsontext.Value { return attachedDiff(t, from.ID) },
			"Fee payment removed for Hana Kim"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOwnerEnv(t, signedInAs(fixture.StaffName("Renata Alvarez")), withEngagement(fixture.Status(fixture.StatusActive)))
			from := e.staff(fixture.StaffName("Hana Kim"))
			to := e.staff(fixture.StaffName("Mei Tan"))
			e.seedActivityWithDiff(tc.action, activity.StaffActor(e.Staff.ID), tc.diff(t, from, to))

			got := e.activityAs(e.Staff)

			if len(got.Items) != 1 {
				t.Fatalf("got %d items, want 1", len(got.Items))
			}
			entry := got.Items[0]
			if entry.Detail != tc.want {
				t.Fatalf("detail = %q, want %q", entry.Detail, tc.want)
			}
			if strings.Contains(entry.Detail, departedID) {
				t.Fatalf("detail = %q, which shows a bare uuid", entry.Detail)
			}
			if entry.ActorName != "Renata Alvarez" || entry.CreatedAt.IsZero() {
				t.Fatalf("entry = %+v, want who did it and when", entry)
			}
		})
	}
}

// TestListActivityHandler_OtherActionsCarryNoDetail holds every other
// action to the rendering it has today: no detail field at all, so the
// app falls back to its own generic description (#887).
func TestListActivityHandler_OtherActionsCarryNoDetail(t *testing.T) {
	e := newOwnerEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	e.seedActivity(activity.ActionVisitLogged, activity.StaffActor(e.Staff.ID))

	// Decoded loosely rather than into ActivityEntry, because what is
	// asserted is the key's absence, which the struct cannot show.
	raw := reqtest.Expect[struct {
		Items []map[string]jsontext.Value `json:"items"`
	}](t, http.MethodGet, e.engagementURL("/activity"), http.StatusOK, e.signedIn)

	if len(raw.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(raw.Items))
	}
	if _, has := raw.Items[0]["detail"]; has {
		t.Fatalf("a visit_logged entry carried a detail field: %v", raw.Items[0])
	}
}
