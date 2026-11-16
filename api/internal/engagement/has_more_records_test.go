package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// seedVisit, seedMessage, seedInvoice and seedOffer put one of each record
// on engagementID.
func (e *env) seedVisit(engagementID string) {
	e.T.Helper()
	e.Exec(`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2)`, engagementID, e.Staff.ID)
}

func (e *env) seedMessage(engagementID string) {
	e.T.Helper()
	e.Exec(`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, 'staff', $2, 'Hello')`, engagementID, e.Staff.ID)
}

func (e *env) seedInvoice(engagementID string) {
	e.T.Helper()
	e.Exec(`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', '', 15000)`, engagementID)
	e.Exec(`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, reference, due_at)
	        SELECT $1, c.id, 'in_' || c.id, 'open', 5000, 'ref' || c.id, now() + interval '7 days' FROM contracts c WHERE c.engagement_id = $2`,
		e.Practice.ID, engagementID)
}

func (e *env) seedOffer(engagementID string) {
	e.T.Helper()
	lena := e.Colleague(fixture.Contractor())
	e.Exec(`INSERT INTO engagement_offers
	            (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
	             due_date, state, offered_by, expires_at)
	        VALUES ($1, $2, 'contractor', 45000, 'A', 'North side', '2027-02-14', 'offered', $3, now() + interval '7 days')`,
		engagementID, lena.ID, e.Staff.ID)
}

// #2755: the Engagement read says which of its Visits, Messages, Invoices
// and Offers exist, so the hub settles its section order on first paint.
func TestDetailHandler_SaysWhichVisitsMessagesInvoicesAndOffersExist(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	active := fixture.Status(fixture.StatusActive)
	empty := e.AnotherEngagement(active)
	full := e.AnotherEngagement(active)
	e.seedVisit(full.ID)
	e.seedMessage(full.ID)
	e.seedInvoice(full.ID)
	e.seedOffer(full.ID)
	onlyVisit := e.AnotherEngagement(active)
	e.seedVisit(onlyVisit.ID)

	type flags struct{ visits, messages, invoices, offers bool }
	for _, tc := range []struct {
		name         string
		engagementID string
		want         flags
	}{
		{"nothing yet", empty.ID, flags{}},
		{"only a Visit", onlyVisit.ID, flags{visits: true}},
		{"all four", full.ID, flags{true, true, true, true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := e.detailOf(tc.engagementID, e.Staff)
			if have := (flags{got.HasVisits, got.HasMessages, got.HasInvoices, got.HasOffers}); have != tc.want {
				t.Errorf("hasVisits/hasMessages/hasInvoices/hasOffers = %+v, want %+v", have, tc.want)
			}
		})
	}
}

// Only an Owner or Admin, the readers of Offers, is told one exists.
func TestDetailHandler_HasOffersForTheOffersReadersOnly(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	owner := e.Colleague(fixture.Owner())
	employee := e.Colleague()
	e.seedOffer(e.Engagement.ID)

	for _, tc := range []struct {
		name   string
		reader fixture.StaffRecord
		want   bool
	}{
		{"an Owner is told", owner, true},
		{"an employee Doula is not", employee, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.detailAs(tc.reader).HasOffers; got != tc.want {
				t.Errorf("hasOffers = %v, want %v", got, tc.want)
			}
		})
	}
}

// The window is the on-call panel's own: present for a birth with a Doula
// and a due date, absent for a postpartum Engagement.
func TestDetailHandler_CarriesTheOnCallWindow(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	doula := e.Colleague()
	birth := e.AnotherEngagement(fixture.Status(fixture.StatusActive), fixture.DueDate("2026-12-05"), fixture.WithDoula(doula))
	postpartum := e.AnotherEngagement(fixture.Status(fixture.StatusActive))
	e.Exec(`UPDATE engagements SET kind = 'postpartum', due_date = NULL WHERE id = $1`, postpartum.ID)

	got := e.detailOf(birth.ID, e.Staff).OnCallWindow
	if got == nil || got.To <= got.From || got.From == "" {
		t.Fatalf("onCallWindow = %+v, want a window ending after it starts", got)
	}
	if tonight := e.detailOf(birth.ID, e.Staff).Tonight; tonight != nil && tonight.IsNobody == (len(tonight.Holders) > 0) {
		t.Errorf("tonight = %+v, want isNobody to say whether anyone holds it", tonight)
	}
	if after := e.detailOf(postpartum.ID, e.Staff).OnCallWindow; after != nil {
		t.Errorf("onCallWindow = %+v, want none for postpartum care", after)
	}
}
