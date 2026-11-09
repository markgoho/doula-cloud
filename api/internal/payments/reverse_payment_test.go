package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func postReversal(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID, paymentID, reason string) *http.Response {
	t.Helper()
	resp := reqtest.Post(t, srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/payments/"+paymentID+"/reverse", reqtest.JSON(payments.ReversePaymentRequest{Reason: reason}), reqtest.Session(session))
	return resp
}

// recordPayment records a manual Payment against invoiceID and returns
// its id, failing the test on anything but 201.
func recordPayment(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID string) (paymentID string) {
	t.Helper()
	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCheck, "check #1", todayInSeededZone(t))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("record payment: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PaymentView
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		t.Fatalf("decode recorded payment: %v", err)
	}
	return out.ID
}

// reversalRow is a payments row read directly via the superuser Admin
// connection, carrying only the columns #945's reversal tests need.
type reversalRow struct {
	kind              string
	amountCents       int64
	reversedPaymentID *string
	reason            *string
}

func paymentRowByID(t *testing.T, db *testdb.DB, paymentID string) reversalRow {
	t.Helper()
	var row reversalRow
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, amount_cents, target_payment_id::text, reason FROM payments WHERE id = $1`, paymentID,
	).Scan(&row.kind, &row.amountCents, &row.reversedPaymentID, &row.reason); err != nil {
		t.Fatalf("query payment %s: %v", paymentID, err)
	}
	return row
}

// record records a manual check Payment for the whole amount against
// invoiceID as the Staff member at the screen and returns its id.
func (e *env) record(invoiceID string) (paymentID string) {
	e.t.Helper()
	return reqtest.Expect[payments.PaymentView](e.t, http.MethodPost, e.url(paymentsPath(invoiceID)), http.StatusCreated,
		e.signedIn, paymentRequest(payments.PaymentMethodCheck, "check #1", todayInSeededZone(e.t))).ID
}

func reversalPath(invoiceID, paymentID string) string {
	return paymentsPath(invoiceID) + "/" + paymentID + "/reverse"
}

// reverse reverses paymentID as the Staff member at the screen.
func (e *env) reverse(invoiceID, paymentID, reason string) payments.PaymentView {
	e.t.Helper()
	return reqtest.Expect[payments.PaymentView](e.t, http.MethodPost, e.url(reversalPath(invoiceID, paymentID)), http.StatusCreated,
		e.signedIn, reqtest.JSON(payments.ReversePaymentRequest{Reason: reason}))
}

// TestPostReversePaymentHandler_ByHandHappyPath proves the core path: an
// Owner reverses a manually recorded Payment against a by-hand Invoice,
// netting it to zero, returning the Invoice to 'open', and recording who
// and why in the Activity log.
func TestPostReversePaymentHandler_ByHandHappyPath(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID := e.byHandInvoice()
	paymentID := e.record(invoiceID)

	out := e.reverse(invoiceID, paymentID, "logged against the wrong invoice")

	if out.AmountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("reversal.amountCents = %d, want %d", out.AmountCents, -seedByHandInvoiceAmountCents)
	}
	if out.TargetPaymentID == nil || *out.TargetPaymentID != paymentID {
		t.Fatalf("reversal.targetPaymentId = %v, want %q", out.TargetPaymentID, paymentID)
	}
	if out.Reason == nil || *out.Reason != "logged against the wrong invoice" {
		t.Fatalf("reversal.reason = %v, want the given reason", out.Reason)
	}
	if status := invoiceStatusFor(t, e.db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
	}
	row := paymentRowByID(t, e.db, out.ID)
	if row.kind != "reversal" || row.amountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("reversal row = %+v, want kind reversal amount %d", row, -seedByHandInvoiceAmountCents)
	}
	if got := e.activityCount("payment_reversed", engagementID); got != 1 {
		t.Fatalf("payment_reversed activity rows = %d, want 1", got)
	}
}

// TestPostReversePaymentHandler_Refusals proves what a reversal refuses
// and that the Invoice is left as it was.
func TestPostReversePaymentHandler_Refusals(t *testing.T) {
	// paidThenReversedAndPaidAgain leaves the Invoice 'paid' once more: the
	// state PostReversePaymentHandler requires before it will even look at
	// the target payment id, since otherwise the second call would 409 on
	// MsgInvoiceNotPaid before reaching the check a row targets.
	paidThenReversedAndPaidAgain := func(e *env) (invoiceID, paymentID, reversalID string) {
		_, invoiceID = e.byHandInvoice()
		paymentID = e.record(invoiceID)
		reversalID = e.reverse(invoiceID, paymentID, "first reversal").ID
		e.record(invoiceID)
		return invoiceID, paymentID, reversalID
	}
	paid := func(e *env) (string, string) {
		_, invoiceID := e.byHandInvoice()
		return invoiceID, e.record(invoiceID)
	}

	for _, tc := range []struct {
		name string
		// setup returns the Invoice and Payment ids the request names.
		setup func(e *env) (invoiceID, paymentID string)
		as    []fixture.StaffOption
		// reason is what the reversal states; body, when set, replaces it.
		reason string
		body   reqtest.Option
		want   int
		// invoiceStays is the status the Invoice must still have, when the
		// request names a real one.
		invoiceStays string
	}{
		{
			// Stripe's own detach_payment call cannot undo a paid_out_of_band
			// mark (verified in the Sandbox, #945's own issue comment).
			name: "a Payment against a Stripe-backed Invoice",
			setup: func(e *env) (string, string) {
				invoiceID := e.stripeInvoice("in_reverse_stripe", "")
				if _, err := e.db.Admin.ExecContext(e.t.Context(), `UPDATE invoices SET status = 'paid' WHERE id = $1`, invoiceID); err != nil {
					e.t.Fatalf("seed paid status: %v", err)
				}
				if _, err := e.db.Admin.ExecContext(e.t.Context(),
					`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 15000, now(), 'manual', 'check')`,
					invoiceID,
				); err != nil {
					e.t.Fatalf("seed manual payment on stripe-backed invoice: %v", err)
				}
				return invoiceID, testdb.Scalar[string](e.t, e.db.Admin, `SELECT id FROM payments WHERE invoice_id = $1`, invoiceID)
			},
			reason: "wrong invoice", want: http.StatusConflict, invoiceStays: invoiceStatusPaid,
		},
		{
			name: "an open by-hand Invoice with nothing to reverse",
			setup: func(e *env) (string, string) {
				_, invoiceID := e.byHandInvoice()
				return invoiceID, unknownID
			},
			reason: reasonGiven, want: http.StatusConflict, invoiceStays: invoiceStatusOpen,
		},
		{
			name: "a payment id that does not belong to the Invoice",
			setup: func(e *env) (string, string) {
				invoiceID, _ := paid(e)
				return invoiceID, unknownID
			},
			reason: reasonGiven, want: http.StatusNotFound, invoiceStays: invoiceStatusPaid,
		},
		{
			// A reversal row itself can never be the target of another
			// reversal -- #945's own acceptance criterion.
			name: "a reversal row as the target",
			setup: func(e *env) (string, string) {
				invoiceID, _, reversalID := paidThenReversedAndPaidAgain(e)
				return invoiceID, reversalID
			},
			reason: "reversing a reversal", want: http.StatusNotFound, invoiceStays: invoiceStatusPaid,
		},
		{
			name: "the same Payment twice",
			setup: func(e *env) (string, string) {
				invoiceID, paymentID, _ := paidThenReversedAndPaidAgain(e)
				return invoiceID, paymentID
			},
			reason: "second", want: http.StatusConflict, invoiceStays: invoiceStatusPaid,
		},
		{name: "a blank reason", setup: paid, reason: "", want: http.StatusBadRequest, invoiceStays: invoiceStatusPaid},
		{name: caseDoula, setup: paid, as: []fixture.StaffOption{fixture.Doula()}, reason: reasonGiven, want: http.StatusForbidden, invoiceStays: invoiceStatusPaid},
		{
			name:   "an Invoice that does not exist",
			setup:  func(*env) (string, string) { return unknownID, unknownID },
			reason: reasonGiven, want: http.StatusNotFound,
		},
		{
			name: "a payment id that is not a uuid",
			setup: func(e *env) (string, string) {
				_, invoiceID := e.byHandInvoice()
				return invoiceID, notAUUID
			},
			reason: reasonGiven, want: http.StatusBadRequest,
		},
		{
			name:   caseInvoiceIDNotUUID,
			setup:  func(*env) (string, string) { return notAUUID, unknownID },
			reason: reasonGiven, want: http.StatusBadRequest,
		},
		{name: caseBodyNotJSON, setup: paid, body: reqtest.RawJSON([]byte("{not json")), want: http.StatusBadRequest, invoiceStays: invoiceStatusPaid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			invoiceID, paymentID := tc.setup(e)
			who := e.signedIn
			if len(tc.as) > 0 {
				who = e.as(e.staff(tc.as...))
			}
			body := tc.body
			if body == nil {
				body = reqtest.JSON(payments.ReversePaymentRequest{Reason: tc.reason})
			}

			reqtest.ExpectStatus(t, http.MethodPost, e.url(reversalPath(invoiceID, paymentID)), tc.want, who, body)

			if tc.invoiceStays != "" {
				if status := invoiceStatusFor(t, e.db, invoiceID); status != tc.invoiceStays {
					t.Fatalf("invoice status = %q, want unchanged %q", status, tc.invoiceStays)
				}
			}
		})
	}
}

// TestGetInvoicesHandler_ActivePaymentIDTracksReversal proves
// InvoiceView.ActivePaymentID (#945) names the covering manual Payment
// while the Invoice is paid, and goes away once it is reversed -- what
// the frontend needs to know which Payment a "reverse" action targets.
func TestGetInvoicesHandler_ActivePaymentIDTracksReversal(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID := e.byHandInvoice()
	items := func() []payments.InvoiceView {
		return reqtest.Expect[payments.ListInvoicesResponse](t, http.MethodGet,
			e.url("/engagements/"+engagementID+"/contract/invoices"), http.StatusOK, e.signedIn).Items
	}

	// Before any Payment: no active one.
	if before := items(); len(before) != 1 || before[0].ActivePaymentID != nil {
		t.Fatalf("items = %+v, want one item with no active payment", before)
	}

	paymentID := e.record(invoiceID)

	if afterPayment := items(); len(afterPayment) != 1 || afterPayment[0].ActivePaymentID == nil || *afterPayment[0].ActivePaymentID != paymentID {
		t.Fatalf("items = %+v, want activePaymentId %q", afterPayment, paymentID)
	}

	e.reverse(invoiceID, paymentID, "wrong invoice")

	if afterReversal := items(); len(afterReversal) != 1 || afterReversal[0].ActivePaymentID != nil {
		t.Fatalf("items = %+v, want no active payment after reversal", afterReversal)
	}
}
