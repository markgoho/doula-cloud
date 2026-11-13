package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// kindRefund is the payments.kind value, and also Stripe's own object
// name and credit-note refund type, which the webhook tests spell too.
const kindRefund = "refund"

// seedStripePayment writes the row the Connect webhook's invoice.paid
// handler writes for a card Payment -- kind 'stripe', no method, the
// PaymentIntent id as its reference -- and moves the Invoice to 'paid'.
// Seeded rather than driven through the webhook, because what these
// tests exercise is what a Refund does to a row that already exists.
func seedStripePayment(t *testing.T, db *testdb.DB, invoiceID string, amountCents int64, paymentIntentID string) (paymentID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, stripe_payment_reference)
		 VALUES ($1, $2, now(), 'stripe', $3) RETURNING id`,
		invoiceID, amountCents, paymentIntentID,
	).Scan(&paymentID); err != nil {
		t.Fatalf("seed stripe payment: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE invoices SET status = 'paid', paid_at = now() WHERE id = $1`, invoiceID,
	); err != nil {
		t.Fatalf("mark invoice paid: %v", err)
	}
	return paymentID
}

// paymentRowCount counts every payments row against invoiceID, of any
// kind. webhook_test.go's paymentsForInvoice cannot stand in for it: it
// scans stripe_payment_reference as a string, which a by-hand row leaves
// NULL.
func paymentRowCount(t *testing.T, db *testdb.DB, invoiceID string) int {
	t.Helper()
	n := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM payments WHERE invoice_id = $1`, invoiceID)
	return n
}

// refundRow is a payments row read via the superuser connection, with
// the columns a Refund sets.
type refundRow struct {
	kind            string
	amountCents     int64
	targetPaymentID *string
	method          *string
	note            *string
	stripeReference *string
}

func refundRowByID(t *testing.T, db *testdb.DB, paymentID string) refundRow {
	t.Helper()
	var row refundRow
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, amount_cents, target_payment_id::text, method::text, note, stripe_payment_reference
		   FROM payments WHERE id = $1`, paymentID,
	).Scan(&row.kind, &row.amountCents, &row.targetPaymentID, &row.method, &row.note, &row.stripeReference); err != nil {
		t.Fatalf("query payment %s: %v", paymentID, err)
	}
	return row
}

func refundActivityDiffs(t *testing.T, db *testdb.DB, engagementID string) []map[string]any {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT diff, actor_kind::text FROM activity WHERE action = 'payment_refunded' AND subject_id = $1 ORDER BY created_at`, engagementID)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		var actorKind string
		if err := rows.Scan(&raw, &actorKind); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		diff := map[string]any{}
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatalf("unmarshal diff: %v", err)
		}
		diff["_actorKind"] = actorKind
		out = append(out, diff)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	return out
}

func refundPath(invoiceID, paymentID string) string {
	return paymentsPath(invoiceID) + "/" + paymentID + "/refund"
}

// paidByHandInvoice is what most of these tests start from: a by-hand
// Invoice for seedByHandInvoiceAmountCents with a check recorded against
// it in full.
func (e *env) paidByHandInvoice() (engagementID, invoiceID, paymentID string) {
	e.T.Helper()
	engagementID, invoiceID = e.byHandInvoice()
	return engagementID, invoiceID, e.record(invoiceID)
}

// refund returns amount of paymentID as the Staff member at the screen.
func (e *env) refund(invoiceID, paymentID string, req payments.RefundPaymentRequest) payments.PaymentView {
	e.T.Helper()
	return reqtest.Expect[payments.PaymentView](e.T, http.MethodPost, e.PracticeURL(refundPath(invoiceID, paymentID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(req))
}

// refuseRefund checks both halves of a refusal: the status, and the exact
// message a caller reads -- two refusals that share a status must not be
// able to swap messages unnoticed.
func (e *env) refuseRefund(invoiceID, paymentID string, req payments.RefundPaymentRequest, wantStatus int, wantMessage string) {
	e.T.Helper()
	got := e.postRefused(refundPath(invoiceID, paymentID), wantStatus, reqtest.JSON(req))
	if got.Message != wantMessage {
		e.T.Fatalf("message = %q, want %q", got.Message, wantMessage)
	}
}

func checkRefund(amountCents int64) payments.RefundPaymentRequest {
	return payments.RefundPaymentRequest{AmountCents: amountCents, Method: payments.PaymentMethodCheck}
}

// TestPostRefundPaymentHandler_ByHandFullRefundLeavesInvoicePaid proves
// the core of #1009: an Owner returns a by-hand Payment in full, and the
// result is an additive negative row pointing at that Payment -- never an
// UPDATE of it -- while the Invoice stays 'paid'. No Stripe call, since
// the by-hand rail has no Stripe object to keep in step.
func TestPostRefundPaymentHandler_ByHandFullRefundLeavesInvoicePaid(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID, paymentID := e.paidByHandInvoice()

	out := e.refund(invoiceID, paymentID, payments.RefundPaymentRequest{
		AmountCents: seedByHandInvoiceAmountCents,
		Method:      payments.PaymentMethodCheck,
		Note:        "deposit returned, engagement canceled",
	})

	if out.AmountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("refund.amountCents = %d, want %d", out.AmountCents, -seedByHandInvoiceAmountCents)
	}
	if out.TargetPaymentID == nil || *out.TargetPaymentID != paymentID {
		t.Fatalf("refund.targetPaymentId = %v, want %q", out.TargetPaymentID, paymentID)
	}
	if out.Kind != kindRefund {
		t.Fatalf("refund.kind = %q, want refund", out.Kind)
	}

	row := refundRowByID(t, e.DB, out.ID)
	if row.kind != kindRefund || row.amountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("refund row = %+v, want kind refund amount %d", row, -seedByHandInvoiceAmountCents)
	}
	if row.method == nil || *row.method != "check" {
		t.Fatalf("refund row method = %v, want check", row.method)
	}
	if row.note == nil || *row.note != "deposit returned, engagement canceled" {
		t.Fatalf("refund row note = %v, want the given note", row.note)
	}
	if row.stripeReference != nil {
		t.Fatalf("refund row stripe reference = %q, want none on the by-hand rail", *row.stripeReference)
	}

	original := refundRowByID(t, e.DB, paymentID)
	if original.kind != "manual" || original.amountCents != seedByHandInvoiceAmountCents {
		t.Fatalf("original payment = %+v, want it untouched", original)
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want it to stay %q", status, invoiceStatusPaid)
	}
	if len(e.Stripe.RefundCreditNoteCalls) != 0 {
		t.Fatalf("credit note calls = %d, want 0 on the by-hand rail", len(e.Stripe.RefundCreditNoteCalls))
	}

	diffs := refundActivityDiffs(t, e.DB, engagementID)
	if len(diffs) != 1 {
		t.Fatalf("payment_refunded activity rows = %d, want 1", len(diffs))
	}
	if diffs[0]["_actorKind"] != "staff" {
		t.Fatalf("activity actor = %v, want staff", diffs[0]["_actorKind"])
	}
	if diffs[0]["amountCents"] != float64(-seedByHandInvoiceAmountCents) || diffs[0]["targetPaymentId"] != paymentID {
		t.Fatalf("activity diff = %v, want the amount and the Payment it returns", diffs[0])
	}
	if _, leaked := diffs[0]["note"]; leaked {
		t.Fatalf("activity diff carries the note, which only the erasable payments column may hold: %v", diffs[0])
	}
}

// TestPostRefundPaymentHandler_PartialRefundsAreBoundedTogether proves a
// partial Refund is the same shape as a full one, and that two Refunds
// against one Payment together cannot exceed what it covered.
func TestPostRefundPaymentHandler_PartialRefundsAreBoundedTogether(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID, paymentID := e.paidByHandInvoice()
	transfer := func(amount int64) payments.RefundPaymentRequest {
		return payments.RefundPaymentRequest{AmountCents: amount, Method: payments.PaymentMethodBankTransfer}
	}

	e.refund(invoiceID, paymentID, transfer(10000))
	e.refund(invoiceID, paymentID, transfer(5000))
	e.refuseRefund(invoiceID, paymentID, transfer(1), http.StatusConflict, payments.MsgRefundExceedsPayment)

	if got := len(refundActivityDiffs(t, e.DB, engagementID)); got != 2 {
		t.Fatalf("payment_refunded activity rows = %d, want 2", got)
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_SingleRefundOverPaymentRefused proves the
// cap holds on the very first Refund, not only in combination.
func TestPostRefundPaymentHandler_SingleRefundOverPaymentRefused(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()

	e.refuseRefund(invoiceID, paymentID, checkRefund(seedByHandInvoiceAmountCents+1), http.StatusConflict, payments.MsgRefundExceedsPayment)

	if got := paymentRowCount(t, e.DB, invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
}

// TestPostRefundPaymentHandler_RequestShapeRefusals covers the request
// rules the handler owns: a positive amount, and a method on a by-hand
// Refund, from the same closed set a recorded Payment uses.
func TestPostRefundPaymentHandler_RequestShapeRefusals(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	cases := []struct {
		name string
		req  payments.RefundPaymentRequest
	}{
		{"zero amount", checkRefund(0)},
		{"negative amount", checkRefund(-100)},
		{"missing method on a by-hand refund", payments.RefundPaymentRequest{AmountCents: 100}},
		{"unknown method", payments.RefundPaymentRequest{AmountCents: 100, Method: "carrier_pigeon"}},
		{"other without a note", payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodOther}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.expect(http.MethodPost, refundPath(invoiceID, paymentID), http.StatusBadRequest, reqtest.JSON(tc.req))
		})
	}
	if got := paymentRowCount(t, e.DB, invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
}

// TestPostRefundPaymentHandler_MalformedInputs covers the path and body
// guards shared with every other write on this surface, and proves an
// Invoice or Payment the caller cannot reach reads as absent, not as a
// different refusal.
func TestPostRefundPaymentHandler_MalformedInputs(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	cases := []struct {
		name                  string
		invoiceID, paymentID  string
		body                  reqtest.Option
		want                  int
		wantMessageWhenAbsent string
	}{
		{caseInvoiceIDNotUUID, notAUUID, paymentID, reqtest.JSON(checkRefund(100)), http.StatusBadRequest, ""},
		{"a payment id that is not a uuid", invoiceID, notAUUID, reqtest.JSON(checkRefund(100)), http.StatusBadRequest, ""},
		{caseBodyNotJSON, invoiceID, paymentID, reqtest.RawJSON([]byte("{not json")), http.StatusBadRequest, ""},
		{"an Invoice that does not exist", unknownID, paymentID, reqtest.JSON(checkRefund(100)), http.StatusNotFound, ""},
		{"a Payment that does not exist", invoiceID, unknownID, reqtest.JSON(checkRefund(100)), http.StatusNotFound, payments.MsgPaymentNotRefundable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e.postRefused(refundPath(tc.invoiceID, tc.paymentID), tc.want, tc.body)
			if tc.wantMessageWhenAbsent != "" && got.Message != tc.wantMessageWhenAbsent {
				t.Fatalf("message = %q, want %q", got.Message, tc.wantMessageWhenAbsent)
			}
		})
	}
}

// TestPostRefundPaymentHandler_RefusedWhenInvoiceNotPaid proves there is
// nothing to return against an Invoice that was never settled.
func TestPostRefundPaymentHandler_RefusedWhenInvoiceNotPaid(t *testing.T) {
	e := newEnv(t)
	_, invoiceID := e.byHandInvoice()

	e.refuseRefund(invoiceID, unknownID, checkRefund(100), http.StatusConflict, payments.MsgInvoiceNotPaidForRefund)
}

// TestPostRefundPaymentHandler_DoulaForbidden proves the gate matches
// recording a Payment: Owner and Admin, nobody else.
func TestPostRefundPaymentHandler_DoulaForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	_, invoiceID := e.byHandInvoice()

	e.expect(http.MethodPost, refundPath(invoiceID, unknownID), http.StatusForbidden, reqtest.JSON(checkRefund(100)))
}

// TestPostRefundPaymentHandler_AdminAllowed proves the other half of the
// gate: an Admin, not only an Owner, may return money.
func TestPostRefundPaymentHandler_AdminAllowed(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	_, invoiceID, paymentID := e.paidByHandInvoice()

	e.refund(invoiceID, paymentID, payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCash})
}

// TestPostRefundPaymentHandler_ReversedPaymentCannotBeRefunded and its
// mirror below prove the two acts exclude each other on one Payment. A
// reversed Payment's money never arrived, so there is nothing to return;
// a refunded Payment's money did arrive, so undoing it as though it had
// not would put the Client back in debt for money she was just given.
func TestPostRefundPaymentHandler_ReversedPaymentCannotBeRefunded(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	e.reverse(invoiceID, paymentID, "bounced")
	// The reversal moved the Invoice back to 'open'; put a fresh Payment
	// on it so the refusal below is about the reversed target, not about
	// the Invoice's status.
	e.record(invoiceID)

	e.refuseRefund(invoiceID, paymentID, checkRefund(100), http.StatusConflict, payments.MsgReversedPaymentCannotBeRefunded)
}

func TestPostReversePaymentHandler_RefundedPaymentCannotBeReversed(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	e.refund(invoiceID, paymentID, checkRefund(100))

	got := e.postRefused(reversalPath(invoiceID, paymentID), http.StatusConflict, reqtest.JSON(payments.ReversePaymentRequest{Reason: "wrong invoice"}))

	if got.Message != payments.MsgRefundedPaymentCannotBeReversed {
		t.Fatalf("message = %q, want %q", got.Message, payments.MsgRefundedPaymentCannotBeReversed)
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want it to stay %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_RefundRowIsNotATarget proves a Refund can
// only name an original Payment -- never another Refund.
func TestPostRefundPaymentHandler_RefundRowIsNotATarget(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	refundID := e.refund(invoiceID, paymentID, checkRefund(100)).ID

	e.refuseRefund(invoiceID, refundID, checkRefund(50), http.StatusNotFound, payments.MsgPaymentNotRefundable)
}

// TestPostRefundPaymentHandler_StripeCardPaymentIssuesRefundCreditNote
// proves a card Payment Stripe still holds is returned by a credit note
// carrying refund_amount -- not out_of_band -- with no method, since
// Stripe moved the money and there is no way-it-went-back to name; and
// that the row stores the Refund object's id as its echo key.
func TestPostRefundPaymentHandler_StripeCardPaymentIssuesRefundCreditNote(t *testing.T) {
	e := newEnv(t, connected())
	_, invoiceID := e.seedStripeInvoice("in_refund_card", invoiceStatusOpen, 15000)
	paymentID := seedStripePayment(t, e.DB, invoiceID, 15000, "pi_refund_card")

	e.expect(http.MethodPost, refundPath(invoiceID, paymentID), http.StatusBadRequest, reqtest.JSON(checkRefund(5000)))

	out := e.refund(invoiceID, paymentID, payments.RefundPaymentRequest{AmountCents: 5000})

	if len(e.Stripe.RefundCreditNoteCalls) != 1 {
		t.Fatalf("credit note calls = %d, want 1", len(e.Stripe.RefundCreditNoteCalls))
	}
	want := payments.FakeRefundCreditNoteCall{AccountID: e.AccountID, InvoiceID: "in_refund_card", AmountCents: 5000, OutOfBand: false}
	if got := e.Stripe.RefundCreditNoteCalls[0]; got != want {
		t.Fatalf("credit note call = %+v, want %+v", got, want)
	}
	row := refundRowByID(t, e.DB, out.ID)
	if row.method != nil {
		t.Fatalf("card refund method = %q, want none", *row.method)
	}
	if row.stripeReference == nil || len(*row.stripeReference) < 3 || (*row.stripeReference)[:3] != "re_" {
		t.Fatalf("card refund reference = %v, want the Refund object's re_ id", row.stripeReference)
	}
	if status := invoiceStatusFor(t, e.DB, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_StripeOutOfBandPaymentRecordsCreditNote
// proves a Payment recorded against a Stripe-backed Invoice with
// paid_out_of_band -- money Stripe never held -- is returned by a credit
// note carrying out_of_band_amount, still names how the Practice sent it
// back, and stores the credit note's own id.
func TestPostRefundPaymentHandler_StripeOutOfBandPaymentRecordsCreditNote(t *testing.T) {
	e := newEnv(t, connected())
	_, invoiceID := e.seedStripeInvoice("in_refund_oob", invoiceStatusOpen, 15000)
	paymentID := e.record(invoiceID)

	out := e.refund(invoiceID, paymentID, payments.RefundPaymentRequest{AmountCents: 15000, Method: payments.PaymentMethodCheck, Note: "check #2201 mailed back"})

	want := payments.FakeRefundCreditNoteCall{AccountID: e.AccountID, InvoiceID: "in_refund_oob", AmountCents: 15000, OutOfBand: true}
	if len(e.Stripe.RefundCreditNoteCalls) != 1 || e.Stripe.RefundCreditNoteCalls[0] != want {
		t.Fatalf("credit note calls = %+v, want exactly %+v", e.Stripe.RefundCreditNoteCalls, want)
	}
	row := refundRowByID(t, e.DB, out.ID)
	if row.method == nil || *row.method != "check" {
		t.Fatalf("out-of-band refund method = %v, want check", row.method)
	}
	if row.stripeReference == nil || (*row.stripeReference)[:3] != "cn_" {
		t.Fatalf("out-of-band refund reference = %v, want the credit note's cn_ id", row.stripeReference)
	}
}

// TestPostRefundPaymentHandler_StripeFailureRecordsNothing proves the
// fail-closed rule the manual-payment path already follows: a Stripe
// refusal saves nothing locally, and the caller is told why.
func TestPostRefundPaymentHandler_StripeFailureRecordsNothing(t *testing.T) {
	stripe := payments.NewFakeClient()
	stripe.RefundCreditNoteErr = errStripeFake
	e := newEnv(t, withStripe(stripe), connected())
	engagementID, invoiceID := e.seedStripeInvoice("in_refund_fail", invoiceStatusOpen, 15000)
	paymentID := seedStripePayment(t, e.DB, invoiceID, 15000, "pi_refund_fail")

	e.refuseRefund(invoiceID, paymentID, payments.RefundPaymentRequest{AmountCents: 5000}, http.StatusBadGateway, payments.MsgStripeRefundFailed)

	if got := paymentRowCount(t, e.DB, invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
	if got := len(refundActivityDiffs(t, e.DB, engagementID)); got != 0 {
		t.Fatalf("payment_refunded activity rows = %d, want 0", got)
	}
}

// TestRefundedInvoice_BookFiguresTellTheTruth proves what a fully
// refunded Invoice does to the figures a Practice reads: it never joins
// the outstanding book (the Client does not owe it again), it still
// counts as a settled bill, and the money that went back is a number of
// its own on both the per-Engagement row and the whole book. The
// per-Engagement row also keeps offering the Payment, so a Staff screen
// can still show what was returned against it.
func TestRefundedInvoice_BookFiguresTellTheTruth(t *testing.T) {
	e := newEnv(t)
	engagementID, invoiceID, paymentID := e.paidByHandInvoice()
	e.refund(invoiceID, paymentID, checkRefund(seedByHandInvoiceAmountCents))

	totals := e.practiceInvoices()
	if totals.OutstandingCents != 0 || totals.OutstandingCount != 0 {
		t.Fatalf("outstanding = %d cents over %d, want nothing -- a Refund is not a debt", totals.OutstandingCents, totals.OutstandingCount)
	}
	if totals.PaidCents != seedByHandInvoiceAmountCents {
		t.Fatalf("paidCents = %d, want %d -- the bill was settled", totals.PaidCents, seedByHandInvoiceAmountCents)
	}
	if totals.RefundedCents != seedByHandInvoiceAmountCents {
		t.Fatalf("refundedCents = %d, want %d", totals.RefundedCents, seedByHandInvoiceAmountCents)
	}

	page := e.listInvoices(engagementID)
	if len(page.Items) != 1 {
		t.Fatalf("invoices = %d, want 1", len(page.Items))
	}
	inv := page.Items[0]
	if inv.Status != invoiceStatusPaid || inv.RefundedCents != seedByHandInvoiceAmountCents {
		t.Fatalf("invoice = status %q refunded %d, want paid and %d", inv.Status, inv.RefundedCents, seedByHandInvoiceAmountCents)
	}
	if inv.ActivePaymentID == nil || *inv.ActivePaymentID != paymentID {
		t.Fatalf("activePaymentId = %v, want %s -- a Refund does not undo the Payment", inv.ActivePaymentID, paymentID)
	}
	if inv.ActivePaymentKind != "manual" {
		t.Fatalf("activePaymentKind = %q, want manual", inv.ActivePaymentKind)
	}
}

// TestPracticeInvoices_RowSaysWhatWasRefunded proves #2746's per-row
// figure: a partly refunded Invoice's row on the Practice-wide list says
// how much went back, and the rows' figures add up to the book's.
func TestPracticeInvoices_RowSaysWhatWasRefunded(t *testing.T) {
	e := newEnv(t)
	_, invoiceID, paymentID := e.paidByHandInvoice()
	e.refund(invoiceID, paymentID, checkRefund(3000))

	book := e.practiceInvoices()
	var sum int64
	for _, row := range book.Items {
		sum += row.RefundedCents
		if row.ID == invoiceID && row.RefundedCents != 3000 {
			t.Fatalf("row refundedCents = %d, want 3000", row.RefundedCents)
		}
	}
	if sum != book.RefundedCents || sum != 3000 {
		t.Fatalf("rows' refunds = %d, book's = %d, want both 3000", sum, book.RefundedCents)
	}
}

// TestStripeCardPayment_IsOfferedAsActive proves #1009's widening of
// activePaymentId: a card Payment Stripe collected is now reachable, so
// a Refund can name it.
func TestStripeCardPayment_IsOfferedAsActive(t *testing.T) {
	e := newEnv(t, connected())
	engagementID, invoiceID := e.seedStripeInvoice("in_active_card", invoiceStatusOpen, 15000)
	paymentID := seedStripePayment(t, e.DB, invoiceID, 15000, "pi_active_card")

	page := e.listInvoices(engagementID)

	if got := page.Items[0].ActivePaymentID; got == nil || *got != paymentID {
		t.Fatalf("activePaymentId = %v, want the card Payment %s", got, paymentID)
	}
	if got := page.Items[0].ActivePaymentKind; got != "stripe" {
		t.Fatalf("activePaymentKind = %q, want stripe", got)
	}
}
