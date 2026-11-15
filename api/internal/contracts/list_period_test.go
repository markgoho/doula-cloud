package contracts_test

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// seedPeriodEdges seeds one Sent Contract on each side of both edges of
// October 25 in the Practice's zone, New York, which is four hours behind UTC
// on that day, and answers their ids in that order.
// jscpd:ignore-start -- the four instants either side of a Practice-zone day's edges are the Invoices list's, seeded through this list's own fixtures
func (e *env) seedPeriodEdges() []string {
	e.T.Helper()
	var seeded []string
	for _, at := range []time.Time{
		time.Date(2026, 10, 25, 3, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 25, 4, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 26, 3, 59, 59, 0, time.UTC),
		time.Date(2026, 10, 26, 4, 0, 0, 0, time.UTC),
	} {
		_, id := e.seedListed(statusSent, at)
		seeded = append(seeded, id)
	}
	return seeded
}

// jscpd:ignore-end

// TestListContracts_PeriodIsTheSentDayInThePracticesZone proves the period's
// edges are the Practice's midnights, not UTC's, and that both entry points
// read it alike.
func TestListContracts_PeriodIsTheSentDayInThePracticesZone(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seeded := e.seedPeriodEdges()
	want := []string{seeded[1], seeded[2]}

	get := e.list("?view=sent&sentFrom=" + dayOct25 + "&sentTo=" + dayOct25 + "&order=oldest")
	post := e.search(contracts.ListRequest{View: statusSent, Order: orderOldest, SentFrom: dayOct25, SentTo: dayOct25})

	for name, out := range map[string]contracts.ListResponse{"GET": get, "POST": post} {
		if !slices.Equal(ids(out), want) || out.Total != 2 {
			t.Fatalf("%s ids = %v (total %d), want %v", name, ids(out), out.Total, want)
		}
	}
}

// TestListContracts_ASignedContractIsInThePeriodItWasSentIn proves the day is
// the day sent, not the day signed or voided.
func TestListContracts_ASignedContractIsInThePeriodItWasSentIn(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	stepped := time.Date(2026, 11, 3, 16, 0, 0, 0, time.UTC)
	_, id := e.seedListed(statusSigned, stepped)

	inSentMonth := e.list("?view=signed&sentFrom=2026-11-03&sentTo=2026-11-03")
	if !slices.Equal(ids(inSentMonth), []string{id}) {
		t.Fatalf("ids = %v, want %s on the day it was sent", ids(inSentMonth), id)
	}
}

// TestListContracts_APeriodLeavesOutADraft proves a Contract never sent has
// no day to be in, whatever the period.
func TestListContracts_APeriodLeavesOutADraft(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedListed(statusDraft, listBase)

	out := e.list("?view=draft&sentFrom=2026-01-01&sentTo=2026-12-31")
	if len(out.Items) != 0 || out.Total != 0 {
		t.Fatalf("items = %v, want none: a Draft was never sent", ids(out))
	}
}

// TestListContracts_APeriodPagesWithoutLosingOrRepeatingARow proves the
// cursor and the period work together.
func TestListContracts_APeriodPagesWithoutLosingOrRepeatingARow(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	for i := range listPageSize + 3 {
		e.seedListed(statusSent, time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC).Add(time.Duration(i)*time.Minute))
	}
	e.seedListed(statusSent, time.Date(2026, 9, 1, 16, 0, 0, 0, time.UTC))

	query := "?view=sent&sentFrom=" + dayOct25 + "&sentTo=" + dayOct25
	first := e.list(query)
	if len(first.Items) != listPageSize || !first.HasMore || first.Total != listPageSize+3 || first.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore %v, total %d", len(first.Items), first.HasMore, first.Total)
	}
	second := e.list(query + "&cursor=" + *first.NextCursor)
	if len(second.Items) != 3 || second.HasMore {
		t.Fatalf("second page = %d items, hasMore %v, want the last 3", len(second.Items), second.HasMore)
	}
}

// TestListContracts_RefusesAPeriodThatIsNotOne proves the same four refusals
// the Invoices list makes.
func TestListContracts_RefusesAPeriodThatIsNotOne(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	for query, message := range map[string]string{
		"?sentFrom=" + dayOct25:                   "Send both sentFrom and sentTo, or neither.",
		"?sentFrom=soon&sentTo=" + dayOct25:       "Use a date like 2026-10-31 for sentFrom.",
		"?sentFrom=" + dayOct25 + "&sentTo=soon":  "Use a date like 2026-10-31 for sentTo.",
		"?sentFrom=2026-10-26&sentTo=" + dayOct25: "Use a sentTo that is not before sentFrom.",
	} {
		e.refusedList(query, message)
	}
	apierrtest.Expect(t, http.MethodPost, e.searchURL(), http.StatusBadRequest, e.SignedIn,
		reqtest.JSON(contracts.ListRequest{SentFrom: dayOct25}))
}
