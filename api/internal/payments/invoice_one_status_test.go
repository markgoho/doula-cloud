package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestAnOverdueInvoiceReadsOverdueInEveryView proves #2122: one overdue
// Invoice is late by the same count on every Staff read of it -- the
// Engagement's Invoice list as an Owner, an Admin, an employee Doula and
// a contractor Doula, and the Practice-wide list -- because each one is
// counted by overdue.Days at the one request instant. The Client portal
// reads the same stored status, open, and no lateness at all: ADR-0038
// keeps Overdue off the portal, so no daysOverdue key reaches her.
func TestAnOverdueInvoiceReadsOverdueInEveryView(t *testing.T) {
	// Due 2026-11-24T00:00Z is Nov 23 in New York; at 5pm UTC on Dec 10
	// it is 17 Practice days late.
	now := time.Date(2026, 12, 10, 17, 0, 0, 0, time.UTC)
	e := newEnv(t, atInstant(now), viaScene(fixture.WithClient(fixture.WithPortalLogin())))
	contractID := seedSignedContract(t, e.DB, e.Engagement.ID)
	invoiceID := seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_one_status", invoiceStatusOpen, 15000,
		time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC), time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC))

	counts := map[string]int{}

	book := e.practiceInvoices()
	if len(book.Items) != 1 || book.Items[0].ID != invoiceID || book.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("Practice-wide items = %+v, want the one open Invoice", book.Items)
	}
	counts["Practice-wide list"] = book.Items[0].DaysOverdue

	staff := []struct {
		name string
		who  reqtest.Option
	}{
		{"Owner", e.SignedIn},
		{"Admin", e.As(e.Colleague(fixture.Admin()))},
		{"employee Doula", e.As(e.Colleague(fixture.Doula()))},
	}
	for _, s := range staff {
		section := reqtest.Expect[payments.ListInvoicesResponse](t, http.MethodGet, e.PracticeURL(invoicesPath(e.Engagement.ID)), http.StatusOK, s.who)
		if len(section.Items) != 1 || section.Items[0].Status != invoiceStatusOpen {
			t.Fatalf("%s: items = %+v, want the one open Invoice", s.name, section.Items)
		}
		counts["Engagement list as "+s.name] = section.Items[0].DaysOverdue
	}

	contractor := e.Colleague(fixture.Doula(), fixture.Contractor())
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, contractor.ID)
	rows := reqtest.Expect[payments.ContractorInvoicesResponse](t, http.MethodGet, e.PracticeURL(invoicesPath(e.Engagement.ID)), http.StatusOK, e.As(contractor))
	if len(rows.Items) != 1 || rows.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("contractor: items = %+v, want the one open Invoice", rows.Items)
	}
	counts["Engagement list as contractor Doula"] = rows.Items[0].DaysOverdue

	for view, got := range counts {
		if got != 17 {
			t.Errorf("%s: daysOverdue = %d, want 17", view, got)
		}
	}

	body := reqtest.ExpectBody(t, http.MethodGet, e.URL(clientInvoicesPath(e.Engagement.ID, "")), http.StatusOK, e.AsClient)
	var portalList payments.ClientInvoicesResponse
	if err := json.Unmarshal(body, &portalList); err != nil {
		t.Fatalf("decode portal: %v", err)
	}
	if len(portalList.Items) != 1 || portalList.Items[0].ID != invoiceID || portalList.Items[0].Status != invoiceStatusOpen {
		t.Fatalf("portal items = %+v, want the one open Invoice", portalList.Items)
	}
	if strings.Contains(string(body), "daysOverdue") || strings.Contains(strings.ToLower(string(body)), "overdue") {
		t.Fatalf("the portal carries lateness, which ADR-0038 keeps off it: %s", body)
	}
}
