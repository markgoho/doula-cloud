package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

func (s *systemStack) arrival(route, outcome, eventType string, at time.Time) {
	s.t.Helper()
	var typ any
	if eventType != "" {
		typ = eventType
	}
	s.exec(`INSERT INTO webhook_arrivals (route, outcome, event_type, arrived_at) VALUES ($1, $2, $3, $4)`,
		route, outcome, typ, at)
}

func routeOf(t *testing.T, v systemView, route string) webhookRoute {
	t.Helper()
	for _, r := range v.Webhooks {
		if r.Route == route {
			return r
		}
	}
	t.Fatalf("no route %q in %+v", route, v.Webhooks)
	return webhookRoute{}
}

func TestSystem_ListsTheFourWebhookRoutesInOrderEvenBeforeAnythingArrives(t *testing.T) {
	s := newSystemStack(t)

	v := s.view()

	var got []string
	for _, r := range v.Webhooks {
		got = append(got, r.Route)
		if r.LastArrivalAt != nil || r.Refusals != (webhookRefusals{}) {
			t.Errorf("%s = %+v, want no arrival and no refusals", r.Route, r)
		}
	}
	if want := []string{"stripe_platform", "stripe_connect", "stripe_account", "mailgun"}; !slices.Equal(got, want) {
		t.Errorf("routes = %v, want %v", got, want)
	}
}

func TestSystem_ALastArrivalIsTheLatestVerifiedRequestAndARefusalNeverMovesIt(t *testing.T) {
	s := newSystemStack(t)
	accepted := opsStart.Add(-30 * time.Hour)
	dropped := opsStart.Add(-20 * time.Hour)
	s.arrival("stripe_connect", "accepted", "invoice.paid", accepted)
	s.arrival("stripe_connect", "dropped", "account.updated", dropped)
	s.arrival("stripe_connect", "signature_refused", "invoice.paid", opsStart.Add(-time.Hour))

	r := routeOf(t, s.view(), "stripe_connect")

	if r.LastArrivalAt == nil || !r.LastArrivalAt.Equal(dropped) {
		t.Errorf("last arrival = %v, want %v", r.LastArrivalAt, dropped)
	}
}

func TestSystem_CountsEachKindOfRefusalInTheLastSevenDaysOnly(t *testing.T) {
	s := newSystemStack(t)
	recent := opsStart.Add(-24 * time.Hour)
	s.arrival("stripe_platform", "signature_refused", "", recent)
	s.arrival("stripe_platform", "signature_refused", "", recent)
	s.arrival("stripe_platform", "livemode_refused", "checkout.session.completed", recent)
	s.arrival("stripe_platform", "body_refused", "", recent)
	s.arrival("stripe_platform", "refused", "", recent)
	s.arrival("stripe_platform", "failed", "checkout.session.completed", recent)
	s.arrival("stripe_platform", "signature_refused", "", opsStart.AddDate(0, 0, -8))
	s.arrival("mailgun", "signature_refused", "", recent)

	v := s.view()

	if got, want := routeOf(t, v, "stripe_platform").Refusals, (webhookRefusals{Signature: 2, Livemode: 1, Body: 1, Other: 1, Failed: 1}); got != want {
		t.Errorf("stripe_platform refusals = %+v, want %+v", got, want)
	}
	if got := routeOf(t, v, "mailgun").Refusals; got != (webhookRefusals{Signature: 1}) {
		t.Errorf("mailgun refusals = %+v, want one signature refusal", got)
	}
}

func findType(t *testing.T, v systemView, route, typ string) eventTypeView {
	t.Helper()
	for _, e := range v.EventTypes {
		if e.Route == route && e.Type == typ {
			return e
		}
	}
	t.Fatalf("no %s on %s in %+v", typ, route, v.EventTypes)
	return eventTypeView{}
}

func TestSystem_ListsEveryHandledStripeEventTypeAndMarksOnesNeverSeen(t *testing.T) {
	s := newSystemStack(t)
	seen := opsStart.Add(-5 * time.Hour)
	s.arrival("stripe_connect", "accepted", "invoice.paid", opsStart.Add(-50*time.Hour))
	s.arrival("stripe_connect", "accepted", "invoice.paid", seen)
	s.arrival("stripe_connect", "dropped", "account.updated", seen)
	s.arrival("stripe_platform", "signature_refused", "checkout.session.completed", seen)

	v := s.view()

	if e := findType(t, v, "stripe_connect", "invoice.paid"); e.LastArrivalAt == nil || !e.LastArrivalAt.Equal(seen) {
		t.Errorf("invoice.paid = %+v, want last seen %v", e, seen)
	}
	if e := findType(t, v, "stripe_connect", "refund.created"); e.LastArrivalAt != nil {
		t.Errorf("refund.created = %+v, want never seen", e)
	}
	if e := findType(t, v, "stripe_platform", "checkout.session.completed"); e.LastArrivalAt != nil {
		t.Errorf("checkout.session.completed = %+v, want never seen: a refused request is not an arrival", e)
	}
	findType(t, v, "stripe_account", "v2.core.account[configuration.merchant].capability_status_updated")
	got := map[string]int{}
	for _, e := range v.EventTypes {
		got[e.Route]++
		if e.Route == "mailgun" || e.Type == "account.updated" {
			t.Errorf("%+v listed, want only handled Stripe event types", e)
		}
	}
	if got["stripe_platform"] != 1 || got["stripe_connect"] != 6 || got["stripe_account"] != 1 {
		t.Errorf("event types per route = %v, want 1, 6 and 1", got)
	}
}

func outboxOf(t *testing.T, v systemView, name string) outboxView {
	t.Helper()
	for _, o := range v.Outboxes {
		if o.Name == name {
			return o
		}
	}
	t.Fatalf("no outbox %q in %+v", name, v.Outboxes)
	return outboxView{}
}

func TestSystem_ListsEveryOutboxTableTheDatabaseHas(t *testing.T) {
	s := newSystemStack(t)
	rows, err := s.db.Admin.QueryContext(t.Context(),
		`SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		  WHERE c.relkind = 'r' AND n.nspname = 'public' AND c.relname LIKE '%\_outbox' ORDER BY 1`)
	if err != nil {
		t.Fatalf("list outbox tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var want []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		want = append(want, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list outbox tables: %v", err)
	}

	var got []string
	for _, o := range s.view().Outboxes {
		got = append(got, o.Name)
	}

	if !slices.Equal(got, want) {
		t.Errorf("outboxes = %v\nwant %v", got, want)
	}
}

func TestSystem_CountsPendingAndDeadLetteredRowsAndDatesTheOldestOneDue(t *testing.T) {
	s := newSystemStack(t)
	first := testdb.SeedPractice(t, s.db, "First")
	second := testdb.SeedPractice(t, s.db, "Second")
	third := testdb.SeedPractice(t, s.db, "Third")
	overdue := opsStart.Add(-2 * time.Hour)
	for _, p := range []struct {
		practice string
		next     time.Time
	}{{first, overdue}, {second, opsStart.Add(-time.Hour)}, {third, opsStart.AddDate(0, 0, 14)}} {
		s.exec(`INSERT INTO welcome_credits_outbox (practice_id, next_attempt_at, created_at) VALUES ($1, $2, $3)`,
			p.practice, p.next, opsStart.Add(-48*time.Hour))
	}
	s.exec(`UPDATE welcome_credits_outbox SET status = 'sent', sent_at = $2 WHERE practice_id = $1`, first, opsStart)
	s.exec(`UPDATE welcome_credits_outbox SET status = 'dead_lettered', dead_letter_reason = 'retries_exhausted' WHERE practice_id = $1`, second)
	fourth := testdb.SeedPractice(t, s.db, "Fourth")
	s.exec(`INSERT INTO welcome_credits_outbox (practice_id, next_attempt_at, created_at) VALUES ($1, $2, $3)`,
		fourth, overdue.Add(time.Minute), overdue)

	o := outboxOf(t, s.view(), "welcome_credits_outbox")

	if o.Pending != 2 || o.DeadLettered != 1 {
		t.Errorf("welcome_credits_outbox = %+v, want 2 pending and 1 dead-lettered", o)
	}
	if want := overdue.Add(time.Minute); o.OldestDueAt == nil || !o.OldestDueAt.Equal(want) {
		t.Errorf("oldest due = %v, want %v: the 14-day row is waiting by design", o.OldestDueAt, want)
	}
}

func TestSystem_AnOutboxWithNothingDueHasNoOldestDue(t *testing.T) {
	s := newSystemStack(t)
	if o := outboxOf(t, s.view(), "low_credit_outbox"); o.Pending != 0 || o.DeadLettered != 0 || o.OldestDueAt != nil {
		t.Errorf("empty outbox = %+v", o)
	}
}

func TestSystem_TheSiteBuildOutboxIsDueWhenItWasQueued(t *testing.T) {
	s := newSystemStack(t)
	queued := opsStart.Add(-90 * time.Minute)
	s.exec(`INSERT INTO site_build_outbox (practice_id, created_at) VALUES ($1, $2)`, testdb.SeedPractice(t, s.db, "Site"), queued)

	o := outboxOf(t, s.view(), "site_build_outbox")

	if o.Pending != 1 || o.OldestDueAt == nil || !o.OldestDueAt.Equal(queued) {
		t.Errorf("site_build_outbox = %+v, want 1 pending due %v", o, queued)
	}
}

func TestSystem_TheGateReadsClosedUntilItIsOpened(t *testing.T) {
	s := newSystemStack(t)
	testdb.ClosePilotGate(t, s.db)

	if g := s.view().Pilot.Gate; g.OpenedAt != nil || g.OpenedBy != nil {
		t.Errorf("gate = %+v, want closed", g)
	}
}

func TestSystem_AnOpenGateNamesWhenAndByWhomItWasOpened(t *testing.T) {
	s := newSystemStack(t)
	testdb.ClosePilotGate(t, s.db)
	opened := opsStart.Add(-72 * time.Hour)
	actID := testdb.SeedOperatorAct(t, s.db, opName)
	s.exec(`SELECT ops_open_pilot_gate($1, $2)`, actID, opened)

	g := s.view().Pilot.Gate

	if g.OpenedAt == nil || !g.OpenedAt.Equal(opened) || g.OpenedBy == nil || *g.OpenedBy != opName {
		t.Errorf("gate = %+v, want opened %v by Mark Goho", g, opened)
	}
}

func TestSystem_AGateOpenedBeforeOpeningsWereKeyedToAnActNamesNobody(t *testing.T) {
	s := newSystemStack(t)

	g := s.view().Pilot.Gate

	if g.OpenedAt == nil || g.OpenedBy != nil {
		t.Errorf("gate = %+v, want open with no opener on record", g)
	}
}

func TestSystem_TheAllowlistListsEachAddressNewestFirstWithWhoAddedIt(t *testing.T) {
	s := newSystemStack(t)
	actID := testdb.SeedOperatorAct(t, s.db, opName)
	older, newer := opsStart.Add(-48*time.Hour), opsStart.Add(-24*time.Hour)
	s.exec(`SELECT ops_add_pilot_address('older@example.com', $1, $2)`, actID, older)
	s.exec(`SELECT ops_add_pilot_address('newer@example.com', $1, $2)`, actID, newer)
	s.exec(`ALTER TABLE pilot_allowlist DROP CONSTRAINT pilot_allowlist_operator_act`)
	s.exec(`INSERT INTO pilot_allowlist (address, added_at) VALUES ('legacy@example.com', $1)`, older.Add(-time.Hour))

	a := s.view().Pilot.Allowlist

	if a.Total != 3 || len(a.Items) != 3 {
		t.Fatalf("allowlist = %+v, want 3 of 3", a)
	}
	if a.Items[0].Address != "newer@example.com" || a.Items[1].Address != "older@example.com" || a.Items[2].Address != "legacy@example.com" {
		t.Errorf("order = %+v", a.Items)
	}
	if a.Items[0].AddedBy == nil || *a.Items[0].AddedBy != opName || !a.Items[0].AddedAt.Equal(newer) {
		t.Errorf("first = %+v, want added by Mark Goho at %v", a.Items[0], newer)
	}
	if a.Items[2].AddedBy != nil {
		t.Errorf("legacy = %+v, want no adder on record", a.Items[2])
	}
}

func TestSystem_AnEmptyAllowlistIsAnEmptyListNotNull(t *testing.T) {
	s := newSystemStack(t)

	w := s.read(s.signedInCookie())

	if !strings.Contains(w.Body.String(), `"allowlist":{"total":0,"items":[]}`) {
		t.Errorf("body = %s, want an empty items array", w.Body)
	}
}

func TestSystem_TheAllowlistShowsNoMoreThanTheLimitAndSaysHowManyThereAre(t *testing.T) {
	s := newSystemStack(t)
	s.exec(`ALTER TABLE pilot_allowlist DROP CONSTRAINT pilot_allowlist_operator_act`)
	s.exec(`INSERT INTO pilot_allowlist (address, added_at)
	        SELECT 'a' || n || '@example.com', $1::timestamptz - n * interval '1 minute' FROM generate_series(1, $2::int) n`,
		opsStart, allowlistShown+2)

	a := s.view().Pilot.Allowlist

	if len(a.Items) != allowlistShown || a.Total != allowlistShown+2 {
		t.Errorf("allowlist = %d items of %d, want %d of %d", len(a.Items), a.Total, allowlistShown, allowlistShown+2)
	}
}

func TestSystem_WritesNoRowAndReadsNoClient(t *testing.T) {
	s := newSystemStack(t)
	cookie := s.signedInCookie()
	before := s.count(`SELECT count(*) FROM operator_acts`)

	if w := s.read(cookie); w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	if after := s.count(`SELECT count(*) FROM operator_acts`); after != before {
		t.Errorf("operator_acts grew from %d to %d, want a read that writes no row", before, after)
	}
}
