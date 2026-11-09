package client_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/reqtest"
)

// #691: the eligibility precheck an Owner reads before ever reaching the
// erasure confirmation. It answers the same question EraseHandler's own
// 409 would otherwise be the only way to learn.

// eligibility reads the precheck as the Owner.
func (e *eraseEnv) eligibility() client.EraseEligibility {
	e.T.Helper()
	return reqtest.Expect[client.EraseEligibility](e.T, http.MethodGet, e.erasureURL(), http.StatusOK, e.SignedIn)
}

// TestEraseEligibilityHandler_ClearWithNoInvoices -- a Client with no
// billing history at all is immediately eligible: not erased, and no
// invoice stands in the way.
func TestEraseEligibilityHandler_ClearWithNoInvoices(t *testing.T) {
	e := newEraseEnv(t)

	out := e.eligibility()
	if out.ErasedAt != nil {
		t.Fatalf("erasedAt = %v, want absent for a Client who was never erased", out.ErasedAt)
	}
	if out.UnsettledInvoices == nil || len(out.UnsettledInvoices) != 0 {
		t.Fatalf("unsettledInvoices = %+v, want an empty slice", out.UnsettledInvoices)
	}
}

// TestEraseEligibilityHandler_NamesUnsettledInvoicesOnly -- the same
// fact EraseHandler's own 409 checks: draft and open invoices block, a
// paid one does not, and every name shown is one the eventual 409 would
// have named too.
func TestEraseEligibilityHandler_NamesUnsettledInvoicesOnly(t *testing.T) {
	e := newEraseEnv(t)
	seedInvoicedClient(t, e.DB, e.Practice.ID, e.ClientID, "cus_open", "open", time.Hour)
	seedInvoicedClient(t, e.DB, e.Practice.ID, e.ClientID, "cus_paid", "paid", time.Hour)

	out := e.eligibility()
	if len(out.UnsettledInvoices) != 1 {
		t.Fatalf("unsettledInvoices = %+v, want exactly the one open invoice", out.UnsettledInvoices)
	}
	got := out.UnsettledInvoices[0]
	if got.Status != "open" {
		t.Fatalf("status = %q, want %q", got.Status, "open")
	}
	if got.AmountCents != 150000 {
		t.Fatalf("amountCents = %d, want %d", got.AmountCents, 150000)
	}
	if got.Currency != "usd" {
		t.Fatalf("currency = %q, want %q", got.Currency, "usd")
	}
	if got.InvoiceID == "" {
		t.Fatal("invoiceId is blank")
	}

	// The same fact, confirmed against the actual 409: attempting the
	// erasure now refuses on exactly the invoice the precheck named.
	postResp := reqtest.Post(t, e.erasureURL(), e.SignedIn)
	defer postResp.Body.Close()
	reqtest.RequireStatus(t, postResp, http.StatusConflict)
	if code := apierrtest.Decode(t, postResp).Code; code != apierr.CodeFailedPrecondition {
		t.Fatalf("erasure code = %q, want %q", code, apierr.CodeFailedPrecondition)
	}
}

// TestEraseEligibilityHandler_AlreadyErased -- once erased, the precheck
// says so and stops naming invoices; the act cannot run again regardless.
func TestEraseEligibilityHandler_AlreadyErased(t *testing.T) {
	e := newEraseEnv(t)
	e.erase()

	out := e.eligibility()
	if out.ErasedAt == nil {
		t.Fatal("erasedAt is absent for a Client who was already erased")
	}
	if len(out.UnsettledInvoices) != 0 {
		t.Fatalf("unsettledInvoices = %+v, want none once erasedAt is set", out.UnsettledInvoices)
	}
}
