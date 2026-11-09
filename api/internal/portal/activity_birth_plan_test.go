package portal_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/engagement"
)

// seedBirthPlanHistory writes one row of each Birth Plan action, and one
// Visit beside them -- the row that stays whatever the Engagement's
// outcome, so a test can tell "the Birth Plan rows left" from "the whole
// ledger went empty".
func (e *env) seedBirthPlanHistory() {
	e.T.Helper()
	e.seedActivity(
		activity.ActionBirthPlanStarted,
		activity.ActionBirthPlanEdited,
		activity.ActionBirthPlanAcknowledged,
		activity.ActionVisitLogged,
	)
}

// setEngagementFacts sets the two facts engagement.OffersBirthPlan reads,
// with 00093's correction door open for the transaction: moving a
// recorded outcome back to a live birth is exactly the correction the
// freeze trigger otherwise refuses.
func (e *env) setEngagementFacts(kind string, birthOutcome *string) {
	e.T.Helper()
	tx, err := e.DB.Admin.BeginTx(e.T.Context(), nil)
	if err != nil {
		e.T.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(e.T.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		e.T.Fatalf("open the correction door: %v", err)
	}
	if _, err := tx.ExecContext(e.T.Context(),
		`UPDATE engagements SET kind = $2::engagement_kind, birth_outcome = $3::birth_outcome,
		        pregnancy_ended_on = CASE WHEN $3::text IS NULL THEN NULL ELSE '2027-03-04'::date END
		 WHERE id = $1`,
		e.Engagement.ID, kind, birthOutcome); err != nil {
		e.T.Fatalf("set engagement facts: %v", err)
	}
	if err := tx.Commit(); err != nil {
		e.T.Fatalf("commit engagement facts: %v", err)
	}
}

// TestActivityHandler_BirthPlanRowsFollowOffersBirthPlan proves #2219 and
// the portal half of #2277 at the reader: wherever
// engagement.OffersBirthPlan is false -- a loss, an outcome the Practice
// never learned, a postpartum Engagement -- her ledger names no Birth
// Plan, and correcting the outcome back to a live birth brings every row
// back, because nothing was stored to hide them.
func TestActivityHandler_BirthPlanRowsFollowOffersBirthPlan(t *testing.T) {
	e := newEnv(t)
	e.seedBirthPlanHistory()

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
		e.setEngagementFacts(string(tc.kind), tc.outcome)
		if got := e.activityActions(); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: actions = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestActivityHandler_BirthPlanOmissionKeepsPagesWhole proves #2219's
// cursor criterion: the Birth Plan rows leave in the query, not after
// it, so a page left out of them is still a full page and the next
// cursor still finds the rest.
func TestActivityHandler_BirthPlanOmissionKeepsPagesWhole(t *testing.T) {
	e := newEnv(t)

	// 31 Visits with a Birth Plan row after each: newest first, a filter
	// applied after the page was cut would leave 15 Visits on page one.
	for range 31 {
		e.seedActivity(activity.ActionVisitLogged, activity.ActionBirthPlanEdited)
	}
	loss := engagement.OutcomeLoss
	e.setEngagementFacts(string(engagement.KindBirth), &loss)

	first := e.activityPage("")
	if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v; want 30/true", len(first.Items), first.HasMore)
	}
	second := e.activityPage("?cursor=" + *first.NextCursor)
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page = %d items, hasMore=%v; want 1/false", len(second.Items), second.HasMore)
	}
	for _, item := range slices.Concat(first.Items, second.Items) {
		if item.Action != string(activity.ActionVisitLogged) {
			t.Fatalf("page carried %q after a loss, want only visit_logged", item.Action)
		}
	}
}
