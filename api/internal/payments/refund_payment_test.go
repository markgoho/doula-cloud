package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// kindRefund is the payments.kind value, and also Stripe's own object
// name and credit-note refund type, which the webhook tests spell too.
const kindRefund = "refund"

func postRefund(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID, paymentID string, req payments.RefundPaymentRequest) *http.Response {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	httpReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/payments/"+paymentID+"/refund", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(httpReq, session)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// assertRefusal checks both halves of a refusal: the status, and the
// exact message a caller reads -- two refusals that share a status must
// not be able to swap messages unnoticed.
func assertRefusal(t *testing.T, resp *http.Response, wantStatus int, wantMessage string) {
	t.Helper()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
	if got := apierrtest.Decode(t, resp).Message; got != wantMessage {
		t.Fatalf("message = %q, want %q", got, wantMessage)
	}
}

// decodePaymentView decodes a 201's PaymentView, failing the test on
// any other status so a refusal never reads as a zero-valued success.
func decodePaymentView(t *testing.T, resp *http.Response) payments.PaymentView {
	t.Helper()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PaymentView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode payment view: %v", err)
	}
	return out
}

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
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM payments WHERE invoice_id = $1`, invoiceID,
	).Scan(&n); err != nil {
		t.Fatalf("count payments: %v", err)
	}
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

// byHandPaidInvoice is the fixture most of these tests start from: an
// Owner at a Practice, a by-hand Invoice for seedByHandInvoiceAmountCents,
// and a check recorded against it in full.
type byHandPaidInvoice struct {
	db           *testdb.DB
	srv          *httptest.Server
	session      string
	practiceID   string
	engagementID string
	invoiceID    string
	paymentID    string
	client       *payments.FakeClient
}

func newByHandPaidInvoice(t *testing.T, uid string, roles []string) byHandPaidInvoice {
	t.Helper()
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, roles, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	t.Cleanup(srv.Close)
	f := byHandPaidInvoice{db: db, srv: srv, session: session, practiceID: practiceID, engagementID: engagementID, invoiceID: invoiceID, client: client}
	return f
}

func (f byHandPaidInvoice) withRecordedPayment(t *testing.T) byHandPaidInvoice {
	t.Helper()
	f.paymentID = recordPayment(t, f.srv, f.session, f.practiceID, f.invoiceID)
	return f
}

// TestPostRefundPaymentHandler_ByHandFullRefundLeavesInvoicePaid proves
// the core of #1009: an Owner returns a by-hand Payment in full, and the
// result is an additive negative row pointing at that Payment -- never an
// UPDATE of it -- while the Invoice stays 'paid'. No Stripe call, since
// the by-hand rail has no Stripe object to keep in step.
func TestPostRefundPaymentHandler_ByHandFullRefundLeavesInvoicePaid(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-by-hand-full", []string{ownerRole}).withRecordedPayment(t)

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, payments.RefundPaymentRequest{
		AmountCents: seedByHandInvoiceAmountCents,
		Method:      payments.PaymentMethodCheck,
		Note:        "deposit returned, engagement canceled",
	})
	defer resp.Body.Close()
	out := decodePaymentView(t, resp)

	if out.AmountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("refund.amountCents = %d, want %d", out.AmountCents, -seedByHandInvoiceAmountCents)
	}
	if out.TargetPaymentID == nil || *out.TargetPaymentID != f.paymentID {
		t.Fatalf("refund.targetPaymentId = %v, want %q", out.TargetPaymentID, f.paymentID)
	}
	if out.Kind != kindRefund {
		t.Fatalf("refund.kind = %q, want refund", out.Kind)
	}

	row := refundRowByID(t, f.db, out.ID)
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

	original := refundRowByID(t, f.db, f.paymentID)
	if original.kind != "manual" || original.amountCents != seedByHandInvoiceAmountCents {
		t.Fatalf("original payment = %+v, want it untouched", original)
	}
	if status := invoiceStatusFor(t, f.db, f.invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want it to stay %q", status, invoiceStatusPaid)
	}
	if len(f.client.RefundCreditNoteCalls) != 0 {
		t.Fatalf("credit note calls = %d, want 0 on the by-hand rail", len(f.client.RefundCreditNoteCalls))
	}

	diffs := refundActivityDiffs(t, f.db, f.engagementID)
	if len(diffs) != 1 {
		t.Fatalf("payment_refunded activity rows = %d, want 1", len(diffs))
	}
	if diffs[0]["_actorKind"] != "staff" {
		t.Fatalf("activity actor = %v, want staff", diffs[0]["_actorKind"])
	}
	if diffs[0]["amountCents"] != float64(-seedByHandInvoiceAmountCents) || diffs[0]["targetPaymentId"] != f.paymentID {
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
	f := newByHandPaidInvoice(t, "refund-partials", []string{ownerRole}).withRecordedPayment(t)
	refund := func(amount int64) *http.Response {
		return postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, payments.RefundPaymentRequest{
			AmountCents: amount, Method: payments.PaymentMethodBankTransfer,
		})
	}

	first := refund(10000)
	defer first.Body.Close()
	decodePaymentView(t, first)

	second := refund(5000)
	defer second.Body.Close()
	decodePaymentView(t, second)

	over := refund(1)
	defer over.Body.Close()
	assertRefusal(t, over, http.StatusConflict, payments.MsgRefundExceedsPayment)

	if got := len(refundActivityDiffs(t, f.db, f.engagementID)); got != 2 {
		t.Fatalf("payment_refunded activity rows = %d, want 2", got)
	}
	if status := invoiceStatusFor(t, f.db, f.invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_SingleRefundOverPaymentRefused proves the
// cap holds on the very first Refund, not only in combination.
func TestPostRefundPaymentHandler_SingleRefundOverPaymentRefused(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-over-payment", []string{ownerRole}).withRecordedPayment(t)

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, payments.RefundPaymentRequest{
		AmountCents: seedByHandInvoiceAmountCents + 1, Method: payments.PaymentMethodCheck,
	})
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusConflict, payments.MsgRefundExceedsPayment)
	if got := paymentRowCount(t, f.db, f.invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
}

// TestPostRefundPaymentHandler_RequestShapeRefusals covers the request
// rules the handler owns: a positive amount, and a method on a by-hand
// Refund, from the same closed set a recorded Payment uses.
func TestPostRefundPaymentHandler_RequestShapeRefusals(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-shape", []string{ownerRole}).withRecordedPayment(t)
	cases := []struct {
		name string
		req  payments.RefundPaymentRequest
	}{
		{"zero amount", payments.RefundPaymentRequest{AmountCents: 0, Method: payments.PaymentMethodCheck}},
		{"negative amount", payments.RefundPaymentRequest{AmountCents: -100, Method: payments.PaymentMethodCheck}},
		{"missing method on a by-hand refund", payments.RefundPaymentRequest{AmountCents: 100}},
		{"unknown method", payments.RefundPaymentRequest{AmountCents: 100, Method: "carrier_pigeon"}},
		{"other without a note", payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodOther}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, tc.req)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
	if got := paymentRowCount(t, f.db, f.invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
}

// TestPostRefundPaymentHandler_MalformedInputs covers the path and body
// guards shared with every other write on this surface.
func TestPostRefundPaymentHandler_MalformedInputs(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-malformed", []string{ownerRole}).withRecordedPayment(t)
	req := payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck}

	badInvoice := postRefund(t, f.srv, f.session, f.practiceID, "not-a-uuid", f.paymentID, req)
	defer badInvoice.Body.Close()
	if badInvoice.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed invoice id status = %d, want %d", badInvoice.StatusCode, http.StatusBadRequest)
	}
	badPayment := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, "not-a-uuid", req)
	defer badPayment.Body.Close()
	if badPayment.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed payment id status = %d, want %d", badPayment.StatusCode, http.StatusBadRequest)
	}

	httpReq, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		f.srv.URL+"/api/practices/"+f.practiceID+"/invoices/"+f.invoiceID+"/payments/"+f.paymentID+"/refund", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(httpReq, f.session)
	httpReq.Header.Set("Content-Type", "application/json")
	badBody, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer badBody.Body.Close()
	if badBody.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed body status = %d, want %d", badBody.StatusCode, http.StatusBadRequest)
	}
}

// TestPostRefundPaymentHandler_NotFound proves an Invoice or Payment the
// caller cannot reach reads as absent, not as a different refusal.
func TestPostRefundPaymentHandler_NotFound(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-not-found", []string{ownerRole}).withRecordedPayment(t)
	req := payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck}
	const absent = "00000000-0000-0000-0000-000000000000"

	noInvoice := postRefund(t, f.srv, f.session, f.practiceID, absent, f.paymentID, req)
	defer noInvoice.Body.Close()
	if noInvoice.StatusCode != http.StatusNotFound {
		t.Fatalf("absent invoice status = %d, want %d", noInvoice.StatusCode, http.StatusNotFound)
	}
	noPayment := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, absent, req)
	defer noPayment.Body.Close()
	assertRefusal(t, noPayment, http.StatusNotFound, payments.MsgPaymentNotRefundable)
}

// TestPostRefundPaymentHandler_RefusedWhenInvoiceNotPaid proves there is
// nothing to return against an Invoice that was never settled.
func TestPostRefundPaymentHandler_RefusedWhenInvoiceNotPaid(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-not-paid", []string{ownerRole})

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, "00000000-0000-0000-0000-000000000000",
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck})
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusConflict, payments.MsgInvoiceNotPaidForRefund)
}

// TestPostRefundPaymentHandler_DoulaForbidden proves the gate matches
// recording a Payment: Owner and Admin, nobody else.
func TestPostRefundPaymentHandler_DoulaForbidden(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-doula", []string{doulaRole})
	const absent = "00000000-0000-0000-0000-000000000000"

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, absent,
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPostRefundPaymentHandler_AdminAllowed proves the other half of the
// gate: an Admin, not only an Owner, may return money.
func TestPostRefundPaymentHandler_AdminAllowed(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-admin", []string{adminRole}).withRecordedPayment(t)

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCash})
	defer resp.Body.Close()
	decodePaymentView(t, resp)
}

// TestPostRefundPaymentHandler_ReversedPaymentCannotBeRefunded and its
// mirror below prove the two acts exclude each other on one Payment. A
// reversed Payment's money never arrived, so there is nothing to return;
// a refunded Payment's money did arrive, so undoing it as though it had
// not would put the Client back in debt for money she was just given.
func TestPostRefundPaymentHandler_ReversedPaymentCannotBeRefunded(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-after-reversal", []string{ownerRole}).withRecordedPayment(t)
	reversal := postReversal(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, "bounced")
	defer reversal.Body.Close()
	decodePaymentView(t, reversal)
	// The reversal moved the Invoice back to 'open'; put a fresh Payment
	// on it so the refusal below is about the reversed target, not about
	// the Invoice's status.
	recordPayment(t, f.srv, f.session, f.practiceID, f.invoiceID)

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck})
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusConflict, payments.MsgReversedPaymentCannotBeRefunded)
}

func TestPostReversePaymentHandler_RefundedPaymentCannotBeReversed(t *testing.T) {
	f := newByHandPaidInvoice(t, "reversal-after-refund", []string{ownerRole}).withRecordedPayment(t)
	refund := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck})
	defer refund.Body.Close()
	decodePaymentView(t, refund)

	resp := postReversal(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID, "wrong invoice")
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusConflict, payments.MsgRefundedPaymentCannotBeReversed)
	if status := invoiceStatusFor(t, f.db, f.invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want it to stay %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_RefundRowIsNotATarget proves a Refund can
// only name an original Payment -- never another Refund.
func TestPostRefundPaymentHandler_RefundRowIsNotATarget(t *testing.T) {
	f := newByHandPaidInvoice(t, "refund-of-refund", []string{ownerRole}).withRecordedPayment(t)
	first := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 100, Method: payments.PaymentMethodCheck})
	defer first.Body.Close()
	refundID := decodePaymentView(t, first).ID

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, refundID,
		payments.RefundPaymentRequest{AmountCents: 50, Method: payments.PaymentMethodCheck})
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusNotFound, payments.MsgPaymentNotRefundable)
}

// stripePaidInvoice seeds a Stripe-backed Invoice at a Practice with a
// Connect account, for the two Stripe-rail cases below.
func stripePaidInvoice(t *testing.T, uid, stripeInvoiceID, accountID string) byHandPaidInvoice {
	t.Helper()
	db := testdb.New(t)
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, stripeInvoiceID, invoiceStatusOpen, 15000, time.Now())
	seedConnectAccount(t, db, practiceID, accountID)
	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	t.Cleanup(srv.Close)
	return byHandPaidInvoice{db: db, srv: srv, session: session, practiceID: practiceID, engagementID: engagementID, invoiceID: invoiceID, client: client}
}

// TestPostRefundPaymentHandler_StripeCardPaymentIssuesRefundCreditNote
// proves a card Payment Stripe still holds is returned by a credit note
// carrying refund_amount -- not out_of_band -- with no method, since
// Stripe moved the money and there is no way-it-went-back to name; and
// that the row stores the Refund object's id as its echo key.
func TestPostRefundPaymentHandler_StripeCardPaymentIssuesRefundCreditNote(t *testing.T) {
	f := stripePaidInvoice(t, "refund-stripe-card", "in_refund_card", "acct_refund_card")
	f.paymentID = seedStripePayment(t, f.db, f.invoiceID, 15000, "pi_refund_card")

	withMethod := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 5000, Method: payments.PaymentMethodCheck})
	defer withMethod.Body.Close()
	if withMethod.StatusCode != http.StatusBadRequest {
		t.Fatalf("method on a card refund status = %d, want %d", withMethod.StatusCode, http.StatusBadRequest)
	}

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 5000})
	defer resp.Body.Close()
	out := decodePaymentView(t, resp)

	if len(f.client.RefundCreditNoteCalls) != 1 {
		t.Fatalf("credit note calls = %d, want 1", len(f.client.RefundCreditNoteCalls))
	}
	want := payments.FakeRefundCreditNoteCall{AccountID: "acct_refund_card", InvoiceID: "in_refund_card", AmountCents: 5000, OutOfBand: false}
	if got := f.client.RefundCreditNoteCalls[0]; got != want {
		t.Fatalf("credit note call = %+v, want %+v", got, want)
	}
	row := refundRowByID(t, f.db, out.ID)
	if row.method != nil {
		t.Fatalf("card refund method = %q, want none", *row.method)
	}
	if row.stripeReference == nil || len(*row.stripeReference) < 3 || (*row.stripeReference)[:3] != "re_" {
		t.Fatalf("card refund reference = %v, want the Refund object's re_ id", row.stripeReference)
	}
	if status := invoiceStatusFor(t, f.db, f.invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostRefundPaymentHandler_StripeOutOfBandPaymentRecordsCreditNote
// proves a Payment recorded against a Stripe-backed Invoice with
// paid_out_of_band -- money Stripe never held -- is returned by a credit
// note carrying out_of_band_amount, still names how the Practice sent it
// back, and stores the credit note's own id.
func TestPostRefundPaymentHandler_StripeOutOfBandPaymentRecordsCreditNote(t *testing.T) {
	f := stripePaidInvoice(t, "refund-stripe-oob", "in_refund_oob", "acct_refund_oob")
	paid := postPayment(t, f.srv, f.session, f.practiceID, f.invoiceID, payments.PaymentMethodCheck, "", todayInSeededZone(t))
	defer paid.Body.Close()
	f.paymentID = decodePaymentView(t, paid).ID

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 15000, Method: payments.PaymentMethodCheck, Note: "check #2201 mailed back"})
	defer resp.Body.Close()
	out := decodePaymentView(t, resp)

	want := payments.FakeRefundCreditNoteCall{AccountID: "acct_refund_oob", InvoiceID: "in_refund_oob", AmountCents: 15000, OutOfBand: true}
	if len(f.client.RefundCreditNoteCalls) != 1 || f.client.RefundCreditNoteCalls[0] != want {
		t.Fatalf("credit note calls = %+v, want exactly %+v", f.client.RefundCreditNoteCalls, want)
	}
	row := refundRowByID(t, f.db, out.ID)
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
	f := stripePaidInvoice(t, "refund-stripe-fails", "in_refund_fail", "acct_refund_fail")
	f.paymentID = seedStripePayment(t, f.db, f.invoiceID, 15000, "pi_refund_fail")
	f.client.RefundCreditNoteErr = errStripeFake

	resp := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: 5000})
	defer resp.Body.Close()
	assertRefusal(t, resp, http.StatusBadGateway, payments.MsgStripeRefundFailed)
	if got := paymentRowCount(t, f.db, f.invoiceID); got != 1 {
		t.Fatalf("payments for invoice = %d, want only the original", got)
	}
	if got := len(refundActivityDiffs(t, f.db, f.engagementID)); got != 0 {
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
	f := newByHandPaidInvoice(t, "refund-book-figures", []string{ownerRole}).withRecordedPayment(t)
	refund := postRefund(t, f.srv, f.session, f.practiceID, f.invoiceID, f.paymentID,
		payments.RefundPaymentRequest{AmountCents: seedByHandInvoiceAmountCents, Method: payments.PaymentMethodCheck})
	defer refund.Body.Close()
	decodePaymentView(t, refund)

	book := getPracticeInvoicesNarrowed(t, f.srv, f.session, f.practiceID, "", "")
	defer book.Body.Close()
	var totals payments.PracticeInvoicesResponse
	if err := json.NewDecoder(book.Body).Decode(&totals); err != nil {
		t.Fatalf("decode practice invoices: %v", err)
	}
	if totals.OutstandingCents != 0 || totals.OutstandingCount != 0 {
		t.Fatalf("outstanding = %d cents over %d, want nothing -- a Refund is not a debt", totals.OutstandingCents, totals.OutstandingCount)
	}
	if totals.PaidCents != seedByHandInvoiceAmountCents {
		t.Fatalf("paidCents = %d, want %d -- the bill was settled", totals.PaidCents, seedByHandInvoiceAmountCents)
	}
	if totals.RefundedCents != seedByHandInvoiceAmountCents {
		t.Fatalf("refundedCents = %d, want %d", totals.RefundedCents, seedByHandInvoiceAmountCents)
	}

	list := getInvoices(t, f.srv, f.session, f.practiceID, f.engagementID, "")
	defer list.Body.Close()
	var page payments.ListInvoicesResponse
	if err := json.NewDecoder(list.Body).Decode(&page); err != nil {
		t.Fatalf("decode invoices: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("invoices = %d, want 1", len(page.Items))
	}
	inv := page.Items[0]
	if inv.Status != invoiceStatusPaid || inv.RefundedCents != seedByHandInvoiceAmountCents {
		t.Fatalf("invoice = status %q refunded %d, want paid and %d", inv.Status, inv.RefundedCents, seedByHandInvoiceAmountCents)
	}
	if inv.ActivePaymentID == nil || *inv.ActivePaymentID != f.paymentID {
		t.Fatalf("activePaymentId = %v, want %s -- a Refund does not undo the Payment", inv.ActivePaymentID, f.paymentID)
	}
	if inv.ActivePaymentKind != "manual" {
		t.Fatalf("activePaymentKind = %q, want manual", inv.ActivePaymentKind)
	}
}

// TestStripeCardPayment_IsOfferedAsActive proves #1009's widening of
// activePaymentId: a card Payment Stripe collected is now reachable, so
// a Refund can name it.
func TestStripeCardPayment_IsOfferedAsActive(t *testing.T) {
	f := stripePaidInvoice(t, "active-card", "in_active_card", "acct_active_card")
	f.paymentID = seedStripePayment(t, f.db, f.invoiceID, 15000, "pi_active_card")

	list := getInvoices(t, f.srv, f.session, f.practiceID, f.engagementID, "")
	defer list.Body.Close()
	var page payments.ListInvoicesResponse
	if err := json.NewDecoder(list.Body).Decode(&page); err != nil {
		t.Fatalf("decode invoices: %v", err)
	}
	if got := page.Items[0].ActivePaymentID; got == nil || *got != f.paymentID {
		t.Fatalf("activePaymentId = %v, want the card Payment %s", got, f.paymentID)
	}
	if got := page.Items[0].ActivePaymentKind; got != "stripe" {
		t.Fatalf("activePaymentKind = %q, want stripe", got)
	}
}
