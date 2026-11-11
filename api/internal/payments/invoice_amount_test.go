package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"sync"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// The seeded Contract's price is 15000 (seedContractWithStatus).
const seededPriceCents = 15000

func (e *env) billing(engagementID string) payments.ContractBilling {
	e.T.Helper()
	got := e.listInvoices(engagementID).ContractBilling
	if got == nil {
		e.T.Fatal("contractBilling is absent, want the Contract's billing")
	}
	return *got
}

func (e *env) requireBilling(engagementID string, billed, unbilled int64) {
	e.T.Helper()
	want := payments.ContractBilling{PriceCents: seededPriceCents, BilledCents: billed, UnbilledCents: unbilled}
	if got := e.billing(engagementID); got != want {
		e.T.Fatalf("contractBilling = %+v, want %+v", got, want)
	}
}

func (e *env) setInvoiceStatus(invoiceID, status string) {
	e.T.Helper()
	if _, err := e.DB.Admin.ExecContext(e.T.Context(), `UPDATE invoices SET status = $2::invoice_status WHERE id = $1`, invoiceID, status); err != nil {
		e.T.Fatalf("set invoice status: %v", err)
	}
}

// TestPostInvoiceHandler_RetainerThenBalance proves #2829's core: a
// retainer and a balance are two Invoices on one Contract, each for the
// amount chosen, each with its own due day, and the balance defaults to
// what the retainer left.
func TestPostInvoiceHandler_RetainerThenBalance(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	retainerDue, balanceDue := "2099-01-05", "2099-02-20"
	retainerCents := int64(5000)
	retainer := reqtest.Expect[payments.InvoiceView](t, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(payments.CreateInvoiceRequest{AmountCents: &retainerCents, DueOn: &retainerDue}))
	if retainer.AmountCents != 5000 {
		t.Fatalf("retainer amountCents = %d, want 5000", retainer.AmountCents)
	}
	e.requireBilling(engagementID, 5000, 10000)

	balance := reqtest.Expect[payments.InvoiceView](t, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(payments.CreateInvoiceRequest{DueOn: &balanceDue}))
	if balance.AmountCents != 10000 {
		t.Fatalf("balance amountCents = %d, want 10000 (what the retainer left)", balance.AmountCents)
	}
	if retainer.DueOn != retainerDue || balance.DueOn != balanceDue {
		t.Fatalf("due days = %q and %q, want %q and %q", retainer.DueOn, balance.DueOn, retainerDue, balanceDue)
	}
	if calls := e.Stripe.CreateInvoiceCalls; len(calls) != 2 || calls[0].AmountCents != 5000 || calls[1].AmountCents != 10000 {
		t.Fatalf("CreateInvoice calls = %v, want one at 5000 then one at 10000", calls)
	}
	e.requireBilling(engagementID, seededPriceCents, 0)
}

// TestPostInvoiceHandler_NamedAmountIsRecordedInTheActivity proves the
// Activity row for raising the Invoice records the amount chosen.
func TestPostInvoiceHandler_NamedAmountIsRecordedInTheActivity(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	e.raiseFor(engagementID, 4200)

	diff := testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT diff FROM activity WHERE subject_id = $1 AND action = 'invoice_raised'`, engagementID)
	var decoded struct {
		AmountCents int64 `json:"amountCents"`
	}
	if err := json.Unmarshal(diff, &decoded); err != nil {
		t.Fatalf("decode activity diff: %v", err)
	}
	if decoded.AmountCents != 4200 {
		t.Fatalf("activity diff amountCents = %d, want 4200", decoded.AmountCents)
	}
}

// TestPostInvoiceHandler_NoAmountBillsTheWholePrice proves the unchanged
// default: a request that names no amount, as every caller sent before
// #2829, bills the Contract's whole price.
func TestPostInvoiceHandler_NoAmountBillsTheWholePrice(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	if got := e.raise(engagementID).AmountCents; got != seededPriceCents {
		t.Fatalf("amountCents = %d, want %d", got, seededPriceCents)
	}
}

// TestPostInvoiceHandler_RefusesAnAmountOutsideTheContract proves each
// refusal of an amount, in the shape docs/api-design.md section 7 gives,
// and that none of them reaches Stripe or the table.
func TestPostInvoiceHandler_RefusesAnAmountOutsideTheContract(t *testing.T) {
	cases := []struct {
		name        string
		amountCents int64
		wantMessage string
		wantDetail  string
	}{
		{"zero", 0, msgAmountNotPositive, "Amount must be more than $0.00"},
		{"negative", -100, msgAmountNotPositive, "Amount must be more than $0.00"},
		{"past the price", seededPriceCents + 1, "That is more than the $150.00 still left to bill on this Contract.", "Amount cannot be more than $150.00, what is left to bill"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Doula()), connected())
			engagementID, _ := e.signedContract()

			refusal := e.refuseRaiseFor(engagementID, tc.amountCents, http.StatusBadRequest)

			if refusal.Code != "INVALID_ARGUMENT" || refusal.Message != tc.wantMessage || refusal.Details["amountCents"] != tc.wantDetail {
				t.Fatalf("refusal = %+v, want INVALID_ARGUMENT, %q, details amountCents %q", refusal, tc.wantMessage, tc.wantDetail)
			}
			e.requireNothingRaised()
		})
	}
}

const msgAmountNotPositive = "Enter an amount more than $0.00."

// TestPostInvoiceHandler_RefusesAnAmountPastWhatIsLeft proves the bound is
// on the total billed, not on each Invoice alone.
func TestPostInvoiceHandler_RefusesAnAmountPastWhatIsLeft(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()
	e.raiseFor(engagementID, 10000)

	refusal := e.refuseRaiseFor(engagementID, 5001, http.StatusBadRequest)

	if refusal.Details["amountCents"] != "Amount cannot be more than $50.00, what is left to bill" {
		t.Fatalf("details = %v, want the $50.00 left to bill", refusal.Details)
	}
	e.requireBilling(engagementID, 10000, 5000)
}

// TestPostInvoiceHandler_RefusesWhenNothingIsLeftToBill proves a fully
// billed Contract refuses a request that names no amount and one that
// names one.
func TestPostInvoiceHandler_RefusesWhenNothingIsLeftToBill(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()
	e.raise(engagementID)

	e.refuseRaise(engagementID, http.StatusConflict)
	e.refuseRaiseFor(engagementID, 100, http.StatusConflict)
	if got := len(e.Stripe.CreateInvoiceCalls); got != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1 -- a refusal must not reach Stripe", got)
	}
	if got := invoiceCount(t, e.DB); got != 1 {
		t.Fatalf("invoices = %d, want 1", got)
	}
}

// TestPostInvoiceHandler_VoidReturnsItsAmountToWhatCanBeBilled proves a
// void Invoice no longer counts as billed, and that a written-off one
// still does.
func TestPostInvoiceHandler_VoidReturnsItsAmountToWhatCanBeBilled(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()
	retainer := e.raiseFor(engagementID, 5000)
	balance := e.raise(engagementID)
	e.requireBilling(engagementID, seededPriceCents, 0)

	e.setInvoiceStatus(balance.ID, invoiceStatusVoid)
	e.requireBilling(engagementID, 5000, 10000)

	e.setInvoiceStatus(retainer.ID, invoiceStatusUncollectible)
	e.requireBilling(engagementID, 5000, 10000)

	if got := e.raise(engagementID).AmountCents; got != 10000 {
		t.Fatalf("re-raised amountCents = %d, want 10000 (the voided balance, the written-off retainer still billed)", got)
	}
}

// TestPostInvoiceHandler_ConcurrentRaisesCannotPassThePrice proves the
// bound holds under two raises at once: each asks for 10000 of a 15000
// Contract, and exactly one is taken.
func TestPostInvoiceHandler_ConcurrentRaisesCannotPassThePrice(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, _ := e.signedContract()
	first := e.raiseWithMode(engagementID, string(payments.BillingModeByHand))
	e.setInvoiceStatus(first.ID, invoiceStatusVoid)

	const raisers = 4
	statuses := make([]int, raisers)
	var wg sync.WaitGroup
	for i := range raisers {
		wg.Go(func() {
			amount := int64(10000)
			resp := reqtest.Do(t, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), e.SignedIn,
				reqtest.JSON(payments.CreateInvoiceRequest{AmountCents: &amount}))
			defer func() { _ = resp.Body.Close() }()
			statuses[i] = resp.StatusCode
		})
	}
	wg.Wait()

	created := 0
	for _, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusBadRequest:
		default:
			t.Fatalf("statuses = %v, want only 201 and 400", statuses)
		}
	}
	if created != 1 {
		t.Fatalf("statuses = %v, want exactly one 201 -- two 10000 Invoices pass the 15000 price", statuses)
	}
	e.requireBilling(engagementID, 10000, 5000)
}

// TestGetInvoicesHandler_ContractorLearnsOnlyWhetherAnythingIsLeft
// proves a contractor's list says whether the Contract has any of its
// price left to bill, and carries no figure.
func TestGetInvoicesHandler_ContractorLearnsOnlyWhetherAnythingIsLeft(t *testing.T) {
	e, engagementID, _ := attachedContractor(t)
	path := e.PracticeURL(invoicesPath(engagementID))

	before := reqtest.Expect[map[string]any](t, http.MethodGet, path, http.StatusOK, e.SignedIn)
	if before["hasUnbilled"] != true || before["contractBilling"] != nil {
		t.Fatalf("list = %v, want hasUnbilled true and no contractBilling", before)
	}

	e.raiseWithMode(engagementID, string(payments.BillingModeByHand))
	after := reqtest.Expect[map[string]any](t, http.MethodGet, path, http.StatusOK, e.SignedIn)
	if after["hasUnbilled"] != false {
		t.Fatalf("list = %v, want hasUnbilled false once the price is billed", after)
	}
}
