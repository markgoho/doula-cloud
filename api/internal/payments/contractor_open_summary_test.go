package payments_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
)

// #2991: a contractor Doula is told how many Invoices are open, how many of
// those are overdue and when the next falls due, over every one of them and
// with no amount.
func TestGetInvoicesHandler_AContractorIsToldHowManyAreOpen(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), atInstant(owedNow))
	engagementID, contractID := e.signedContract(fixture.WithDoula(e.Staff))
	created := owedNow.Add(-30 * 24 * time.Hour)
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_late", invoiceStatusOpen, 40000, created, time.Date(2026, time.November, 1, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_soon", invoiceStatusOpen, 120000, created, time.Date(2026, time.December, 1, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_later", invoiceStatusOpen, 10000, created, time.Date(2026, time.December, 20, 17, 0, 0, 0, time.UTC))
	seedInvoiceDue(t, e.DB, e.Practice.ID, contractID, "in_paid", invoiceStatusPaid, 99900, created, time.Date(2026, time.November, 2, 17, 0, 0, 0, time.UTC))
	url := e.PracticeURL(invoicesPath(engagementID))

	got := reqtest.Expect[payments.ContractorInvoicesResponse](t, http.MethodGet, url, http.StatusOK, e.SignedIn)

	if got.OpenCount != 3 || got.OverdueCount != 1 {
		t.Errorf("counts = %d open, %d overdue, want 3 and 1", got.OpenCount, got.OverdueCount)
	}
	if got.NextDueOn == nil || *got.NextDueOn != "2026-12-01" {
		t.Errorf("next due = %v, want 2026-12-01", got.NextDueOn)
	}
	if body := string(reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, e.SignedIn)); strings.Contains(body, "Cents") {
		t.Errorf("a contractor's response carries an amount: %s", body)
	}
}
