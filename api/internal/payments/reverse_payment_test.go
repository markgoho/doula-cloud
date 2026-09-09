package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

func postReversal(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID, paymentID, reason string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.ReversePaymentRequest{Reason: reason})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/payments/"+paymentID+"/reverse", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// recordPayment records a manual Payment against invoiceID and returns
// its id, failing the test on anything but 201.
func recordPayment(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID string) (paymentID string) {
	t.Helper()
	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCheck, "check #1", isoDate(time.Now()))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("record payment: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PaymentView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
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
		`SELECT kind::text, amount_cents, reversed_payment_id::text, reason FROM payments WHERE id = $1`, paymentID,
	).Scan(&row.kind, &row.amountCents, &row.reversedPaymentID, &row.reason); err != nil {
		t.Fatalf("query payment %s: %v", paymentID, err)
	}
	return row
}

// TestPostReversePaymentHandler_ByHandHappyPath proves the core path: an
// Owner reverses a manually recorded Payment against a by-hand Invoice,
// netting it to zero, returning the Invoice to 'open', and recording who
// and why in the Activity log.
func TestPostReversePaymentHandler_ByHandHappyPath(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-happy"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	resp := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "logged against the wrong invoice")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PaymentView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.AmountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("reversal.amountCents = %d, want %d", out.AmountCents, -seedByHandInvoiceAmountCents)
	}
	if out.ReversedPaymentID == nil || *out.ReversedPaymentID != paymentID {
		t.Fatalf("reversal.reversedPaymentId = %v, want %q", out.ReversedPaymentID, paymentID)
	}
	if out.Reason == nil || *out.Reason != "logged against the wrong invoice" {
		t.Fatalf("reversal.reason = %v, want the given reason", out.Reason)
	}

	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
	}

	row := paymentRowByID(t, db, out.ID)
	if row.kind != "reversal" || row.amountCents != -seedByHandInvoiceAmountCents {
		t.Fatalf("reversal row = %+v, want kind reversal amount %d", row, -seedByHandInvoiceAmountCents)
	}

	var actionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE action = 'payment_reversed' AND subject_id = $1`, engagementID,
	).Scan(&actionCount); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("payment_reversed activity rows = %d, want 1", actionCount)
	}
}

// TestPostReversePaymentHandler_RefusedOnStripeBacked proves a Payment
// against a Stripe-backed Invoice cannot be reversed here -- Stripe's own
// detach_payment call cannot undo a paid_out_of_band mark (verified in
// the Sandbox, #945's own issue comment).
func TestPostReversePaymentHandler_RefusedOnStripeBacked(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-stripe-backed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_reverse_stripe", invoiceStatusPaid, 15000, time.Now())
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, amount_cents, paid_at, kind, method) VALUES ($1, 15000, now(), 'manual', 'check')`,
		invoiceID,
	); err != nil {
		t.Fatalf("seed manual payment on stripe-backed invoice: %v", err)
	}
	var paymentID string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT id FROM payments WHERE invoice_id = $1`, invoiceID).Scan(&paymentID); err != nil {
		t.Fatalf("read seeded payment id: %v", err)
	}
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "wrong invoice")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusPaid)
	}
}

// TestPostReversePaymentHandler_RefusedWhenNotPaid proves an open by-hand
// Invoice has nothing to reverse.
func TestPostReversePaymentHandler_RefusedWhenNotPaid(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-not-paid"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postReversal(t, srv, session, practiceID, invoiceID, "00000000-0000-0000-0000-000000000000", "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestPostReversePaymentHandler_UnknownPaymentNotFound proves a payment
// id that does not belong to the Invoice 404s.
func TestPostReversePaymentHandler_UnknownPaymentNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-unknown"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	_ = recordPayment(t, srv, session, practiceID, invoiceID)

	resp := postReversal(t, srv, session, practiceID, invoiceID, "00000000-0000-0000-0000-000000000000", "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostReversePaymentHandler_ReversalCannotBeReversed proves a
// reversal row itself can never be the target of another reversal --
// #945's own acceptance criterion.
func TestPostReversePaymentHandler_ReversalCannotBeReversed(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-double"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	firstReversal := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "first reversal")
	defer firstReversal.Body.Close()
	if firstReversal.StatusCode != http.StatusCreated {
		t.Fatalf("first reversal status = %d, want %d", firstReversal.StatusCode, http.StatusCreated)
	}
	var reversal payments.PaymentView
	if err := json.NewDecoder(firstReversal.Body).Decode(&reversal); err != nil {
		t.Fatalf("decode first reversal: %v", err)
	}

	// The Invoice is 'open' again, so a fresh Payment restores 'paid' --
	// the state PostReversePaymentHandler requires before it will even
	// look at the target payment id.
	_ = recordPayment(t, srv, session, practiceID, invoiceID)

	secondReversal := postReversal(t, srv, session, practiceID, invoiceID, reversal.ID, "reversing a reversal")
	defer secondReversal.Body.Close()
	if secondReversal.StatusCode != http.StatusNotFound {
		t.Fatalf("reversing a reversal row: status = %d, want %d", secondReversal.StatusCode, http.StatusNotFound)
	}
}

// TestPostReversePaymentHandler_AlreadyReversedRefused proves the same
// Payment cannot be reversed twice.
func TestPostReversePaymentHandler_AlreadyReversedRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-twice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	first := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "first")
	defer first.Body.Close()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first reversal status = %d, want %d", first.StatusCode, http.StatusCreated)
	}

	// Pay it again so the Invoice is 'paid' once more -- otherwise the
	// second call would 409 on MsgInvoiceNotPaid before ever reaching the
	// already-reversed check this test targets.
	_ = recordPayment(t, srv, session, practiceID, invoiceID)

	second := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "second")
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second reversal status = %d, want %d", second.StatusCode, http.StatusConflict)
	}
}

// TestPostReversePaymentHandler_BlankReasonRefused proves a reversal
// always needs a stated why.
func TestPostReversePaymentHandler_BlankReasonRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-blank-reason"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	resp := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusPaid)
	}
}

// TestPostReversePaymentHandler_DoulaForbidden proves only Owner and
// Admin may reverse a Payment.
func TestPostReversePaymentHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const ownerUID = "reverse-payment-owner"
	const doulaUID = "reverse-payment-doula-forbidden"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{ownerRole}, "employee")
	testdb.SeedStaffAtPractice(t, db, practiceID, doulaUID, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, ownerSession := newInvoiceServer(t, db, ownerUID, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, ownerSession, practiceID, invoiceID)

	doulaSession := authntest.SeedSession(t, db.App, doulaUID)
	resp := postReversal(t, srv, doulaSession, practiceID, invoiceID, paymentID, "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPostReversePaymentHandler_InvoiceNotFound proves a nonexistent
// Invoice id 404s.
func TestPostReversePaymentHandler_InvoiceNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-invoice-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postReversal(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000", "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostReversePaymentHandler_MalformedPaymentIDReturns400 proves a
// non-UUID :paymentId 400s.
func TestPostReversePaymentHandler_MalformedPaymentIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-bad-payment-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postReversal(t, srv, session, practiceID, invoiceID, "not-a-uuid", "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostReversePaymentHandler_MalformedInvoiceIDReturns400 proves a
// non-UUID :invoiceId 400s.
func TestPostReversePaymentHandler_MalformedInvoiceIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-bad-invoice-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postReversal(t, srv, session, practiceID, "not-a-uuid", "00000000-0000-0000-0000-000000000000", "reason")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestGetInvoicesHandler_ActivePaymentIDTracksReversal proves
// InvoiceView.ActivePaymentID (#945) names the covering manual Payment
// while the Invoice is paid, and goes away once it is reversed -- what
// the frontend needs to know which Payment a "reverse" action targets.
func TestGetInvoicesHandler_ActivePaymentIDTracksReversal(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-active-payment"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	// Before any Payment: no active one.
	beforeResp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer beforeResp.Body.Close()
	var before payments.ListInvoicesResponse
	if err := json.NewDecoder(beforeResp.Body).Decode(&before); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(before.Items) != 1 || before.Items[0].ActivePaymentID != nil {
		t.Fatalf("items = %+v, want one item with no active payment", before.Items)
	}

	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	afterPaymentResp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer afterPaymentResp.Body.Close()
	var afterPayment payments.ListInvoicesResponse
	if err := json.NewDecoder(afterPaymentResp.Body).Decode(&afterPayment); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(afterPayment.Items) != 1 || afterPayment.Items[0].ActivePaymentID == nil || *afterPayment.Items[0].ActivePaymentID != paymentID {
		t.Fatalf("items = %+v, want activePaymentId %q", afterPayment.Items, paymentID)
	}

	reversalResp := postReversal(t, srv, session, practiceID, invoiceID, paymentID, "wrong invoice")
	defer reversalResp.Body.Close()
	if reversalResp.StatusCode != http.StatusCreated {
		t.Fatalf("reversal status = %d, want %d", reversalResp.StatusCode, http.StatusCreated)
	}

	afterReversalResp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer afterReversalResp.Body.Close()
	var afterReversal payments.ListInvoicesResponse
	if err := json.NewDecoder(afterReversalResp.Body).Decode(&afterReversal); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(afterReversal.Items) != 1 || afterReversal.Items[0].ActivePaymentID != nil {
		t.Fatalf("items = %+v, want no active payment after reversal", afterReversal.Items)
	}
}

// TestPostReversePaymentHandler_MalformedBodyReturns400 proves invalid
// JSON 400s.
func TestPostReversePaymentHandler_MalformedBodyReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "reverse-payment-bad-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()
	paymentID := recordPayment(t, srv, session, practiceID, invoiceID)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/payments/"+paymentID+"/reverse", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
