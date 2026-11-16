package payments_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

var owedNow = time.Date(2026, time.November, 10, 17, 0, 0, 0, time.UTC)

// #2755: the Invoices read says what every open Invoice adds up to and what
// part is overdue, by the Practice-wide list's own rule.
func TestGetInvoicesHandler_OwedSummarizesEveryOpenInvoice(t *testing.T) {
	e := newEnv(t, atInstant(owedNow))
	engagementID, contractID := e.contractWithStatus("signed")
	created := owedNow.Add(-30 * 24 * time.Hour)
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_late", invoiceStatusOpen, 40000, created, time.Date(2026, time.November, 1, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_later", invoiceStatusOpen, 10000, created, time.Date(2026, time.November, 5, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_soon", invoiceStatusOpen, 120000, created, time.Date(2026, time.December, 1, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_paid", invoiceStatusPaid, 99900, created, time.Date(2026, time.November, 2, 17, 0, 0, 0, time.UTC))

	owed := e.listInvoices(engagementID).Owed

	if owed == nil || owed.OwedCents != 170000 || owed.OverdueCents != 50000 {
		t.Fatalf("owed = %+v, want 170000 owed of which 50000 overdue", owed)
	}
	if owed.NextDueOn == nil || *owed.NextDueOn != "2026-12-01" || owed.OldestOverdueOn == nil || *owed.OldestOverdueOn != "2026-11-01" {
		t.Errorf("owed days = %v / %v, want next due 2026-12-01 and oldest overdue 2026-11-01", owed.NextDueOn, owed.OldestOverdueOn)
	}
}

func TestGetInvoicesHandler_OwedIsZeroWithNothingOpen(t *testing.T) {
	e := newEnv(t, atInstant(owedNow))
	engagementID, _ := e.contractWithStatus("signed")

	owed := e.listInvoices(engagementID).Owed

	if owed == nil || owed.OwedCents != 0 || owed.OverdueCents != 0 || owed.NextDueOn != nil || owed.OldestOverdueOn != nil {
		t.Errorf("owed = %+v, want zeros and no days", owed)
	}
}

// A contractor reads no money (ADR-0008): her response has no owed.
func TestGetInvoicesHandler_AContractorIsToldNothingOwed(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), atInstant(owedNow))
	engagementID, contractID := e.signedContract(fixture.WithDoula(e.Staff))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_c", invoiceStatusOpen, 40000, owedNow, owedNow.Add(24*time.Hour))

	body := string(reqtest.ExpectBody(t, http.MethodGet, e.PracticeURL(invoicesPath(engagementID)), http.StatusOK, e.SignedIn))

	if strings.Contains(body, `"owed`) {
		t.Errorf("a contractor's response carries owed: %s", body)
	}
}
