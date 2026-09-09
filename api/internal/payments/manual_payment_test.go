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

func postPaymentBody(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/payments", bytes.NewBufferString(body))
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

func postPayment(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID string, method payments.PaymentMethod, note, paidOn string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.RecordPaymentRequest{Method: method, Note: note, PaidOn: paidOn})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return postPaymentBody(t, srv, session, practiceID, invoiceID, string(body))
}

func postInvoiceTransition(t *testing.T, srv *httptest.Server, session, practiceID, invoiceID, action string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/invoices/"+invoiceID+"/"+action, bytes.NewBufferString("{}"))
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

// seedByHandInvoiceAmountCents is the fixed amount every caller of
// seedByHandInvoice wants -- none of them are testing the amount, so it
// is not a parameter (golangci-lint's unparam).
const seedByHandInvoiceAmountCents = 15000

// seedByHandInvoice inserts an 'open' by-hand invoices row (no
// stripe_invoice_id) directly, bypassing PostInvoiceHandler.
func seedByHandInvoice(t *testing.T, db *testdb.DB, practiceID, contractID string) (invoiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, status, amount_cents, currency, reference, due_at)
		 VALUES ($1, $2, 'open', $3, 'usd', 'INV-TEST', now() + interval '30 days') RETURNING id`,
		practiceID, contractID, seedByHandInvoiceAmountCents,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("seed by-hand invoice: %v", err)
	}
	return invoiceID
}

func isoDate(t time.Time) string { return t.Format("2006-01-02") }

// TestPostManualPaymentHandler_ByHandCheckHappyPath proves the core path:
// an Owner records a check against an open by-hand Invoice for its full
// amount, the Invoice flips to paid, and the Activity log carries who and
// when.
func TestPostManualPaymentHandler_ByHandCheckHappyPath(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-happy"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCheck, "check #204", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PaymentView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.AmountCents != 15000 {
		t.Fatalf("payment.amountCents = %d, want 15000", out.AmountCents)
	}
	if out.Note == nil || *out.Note != "check #204" {
		t.Fatalf("payment.note = %v, want %q", out.Note, "check #204")
	}

	status := invoiceStatusFor(t, db, invoiceID)
	if status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}

	var actionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE action = 'payment_recorded' AND subject_id = $1`, engagementID,
	).Scan(&actionCount); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("payment_recorded activity rows = %d, want 1", actionCount)
	}
}

// TestPostManualPaymentHandler_OtherMethodRequiresNote proves "other"
// without a note is refused.
func TestPostManualPaymentHandler_OtherMethodRequiresNote(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-other-no-note"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodOther, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusOpen)
	}
}

// TestPostManualPaymentHandler_OtherMethodWithNoteSucceeds is the
// positive case beside the refusal above.
func TestPostManualPaymentHandler_OtherMethodWithNoteSucceeds(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-other-with-note"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodOther, "Venmo, screenshot on file", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

// TestPostManualPaymentHandler_InvalidMethodRefused proves a method
// outside the closed enum 400s.
func TestPostManualPaymentHandler_InvalidMethodRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-invalid-method"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethod("venmo"), "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostManualPaymentHandler_FutureDateRefused proves paidOn cannot be
// in the future.
func TestPostManualPaymentHandler_FutureDateRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-future-date"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCash, "", isoDate(time.Now().UTC().AddDate(0, 0, 1)))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostManualPaymentHandler_MalformedDateRefused proves a paidOn that
// does not parse as YYYY-MM-DD 400s.
func TestPostManualPaymentHandler_MalformedDateRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-bad-date"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCash, "", "not-a-date")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostManualPaymentHandler_DatePrecedingInvoiceCreationAllowed proves
// a deposit that arrived before the Invoice was raised is accepted --
// #271 sets no lower bound.
func TestPostManualPaymentHandler_DatePrecedingInvoiceCreationAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-early-date"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodBankTransfer, "", "2020-01-01")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

// TestPostManualPaymentHandler_NotOpenInvoiceRefused proves draft, void,
// uncollectible, and already-paid all refuse.
func TestPostManualPaymentHandler_NotOpenInvoiceRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-not-open"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	for _, status := range []string{invoiceStatusDraft, invoiceStatusVoid, invoiceStatusUncollectible, invoiceStatusPaid} {
		invoiceID := seedInvoice(t, db, practiceID, contractID, "in_not_open_"+status, status, 15000, time.Now())
		resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCash, "", isoDate(time.Now().UTC()))
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("status=%q: status = %d, want %d", status, resp.StatusCode, http.StatusConflict)
		}
		_ = resp.Body.Close()
	}
}

// TestPostManualPaymentHandler_DoulaForbidden proves only Owner and Admin
// may record a Payment.
func TestPostManualPaymentHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-doula-forbidden"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCash, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPostManualPaymentHandler_InvoiceNotFound proves a nonexistent (or
// another Practice's) Invoice id 404s.
func TestPostManualPaymentHandler_InvoiceNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", payments.PaymentMethodCash, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostManualPaymentHandler_StripeBackedCallsPayOutOfBandThenRecords
// proves recording a Payment against a Stripe-backed Invoice calls
// PayOutOfBand and, on success, writes the manual Payment locally.
func TestPostManualPaymentHandler_StripeBackedCallsPayOutOfBandThenRecords(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-stripe-backed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_pay_oob", invoiceStatusOpen, 15000, time.Now())
	client := payments.NewFakeClient()
	seedConnectAccount(t, db, practiceID, "acct_pay_oob")
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodBankTransfer, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if len(client.PayOutOfBandCalls) != 1 {
		t.Fatalf("PayOutOfBand calls = %d, want 1", len(client.PayOutOfBandCalls))
	}
	if got := client.PayOutOfBandCalls[0]; got.AccountID != "acct_pay_oob" || got.InvoiceID != "in_pay_oob" {
		t.Fatalf("PayOutOfBand call = %+v, want account acct_pay_oob invoice in_pay_oob", got)
	}
	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
}

// TestPostManualPaymentHandler_StripePayOutOfBandFailureRecordsNothing
// proves #271's fail-closed rule: a Stripe refusal on the out-of-band
// call leaves the Invoice open and writes no payments row.
func TestPostManualPaymentHandler_StripePayOutOfBandFailureRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-stripe-fails"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_pay_oob_fail", invoiceStatusOpen, 15000, time.Now())
	client := payments.NewFakeClient()
	client.PayOutOfBandErr = errStripeFake
	seedConnectAccount(t, db, practiceID, "acct_pay_oob_fail")
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, invoiceID, payments.PaymentMethodCheck, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
	if status := invoiceStatusFor(t, db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want unchanged %q", status, invoiceStatusOpen)
	}
	if got := paymentsForInvoice(t, db, invoiceID); len(got) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(got))
	}
}

// TestPostVoidInvoiceHandler_HappyPath proves an Owner can void an open
// by-hand Invoice.
func TestPostVoidInvoiceHandler_HappyPath(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invoice-happy"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "void")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.InvoiceTransitionView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != "void" {
		t.Fatalf("status = %q, want %q", out.Status, "void")
	}
	if got := invoiceStatusFor(t, db, invoiceID); got != "void" {
		t.Fatalf("persisted invoice status = %q, want %q", got, "void")
	}

	var actionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE action = 'invoice_voided' AND subject_id = $1`, engagementID,
	).Scan(&actionCount); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("invoice_voided activity rows = %d, want 1", actionCount)
	}
}

// TestPostVoidInvoiceHandler_RefusedOnStripeBacked proves a
// Stripe-backed Invoice cannot be voided here.
func TestPostVoidInvoiceHandler_RefusedOnStripeBacked(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invoice-stripe-backed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_void_stripe", invoiceStatusOpen, 15000, time.Now())
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "void")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := invoiceStatusFor(t, db, invoiceID); got != invoiceStatusOpen {
		t.Fatalf("persisted invoice status = %q, want unchanged %q", got, invoiceStatusOpen)
	}
}

// TestPostVoidInvoiceHandler_RefusedWhenNotOpen proves an already-paid
// by-hand Invoice cannot be voided.
func TestPostVoidInvoiceHandler_RefusedWhenNotOpen(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invoice-not-open"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET status = 'paid' WHERE id = $1`, invoiceID); err != nil {
		t.Fatalf("seed paid status: %v", err)
	}
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "void")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestPostVoidInvoiceHandler_DoulaForbidden proves only Owner and Admin
// may void.
func TestPostVoidInvoiceHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invoice-doula-forbidden"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "void")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPostWriteOffInvoiceHandler_HappyPath proves an Owner can write off
// an open by-hand Invoice.
func TestPostWriteOffInvoiceHandler_HappyPath(t *testing.T) {
	db := testdb.New(t)
	const uid = "write-off-invoice-happy"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "write-off")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := invoiceStatusFor(t, db, invoiceID); got != invoiceStatusUncollectible {
		t.Fatalf("persisted invoice status = %q, want %q", got, invoiceStatusUncollectible)
	}

	var actionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE action = 'invoice_written_off' AND subject_id = $1`, engagementID,
	).Scan(&actionCount); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("invoice_written_off activity rows = %d, want 1", actionCount)
	}
}

// TestPostWriteOffInvoiceHandler_RefusedOnStripeBacked proves a
// Stripe-backed Invoice cannot be written off here.
func TestPostWriteOffInvoiceHandler_RefusedOnStripeBacked(t *testing.T) {
	db := testdb.New(t)
	const uid = "write-off-invoice-stripe-backed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_write_off_stripe", invoiceStatusOpen, 15000, time.Now())
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "write-off")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestPostWriteOffInvoiceHandler_DoulaForbidden proves the write-off act
// stays Owner/Admin only under #282: an employed Doula is refused, the
// same as TestPostVoidInvoiceHandler_DoulaForbidden proves for void --
// AC7's "a test for each refusal" named write-off separately even though
// it shares transitionByHandInvoice with void.
func TestPostWriteOffInvoiceHandler_DoulaForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "write-off-invoice-doula-forbidden"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, invoiceID, "write-off")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestPostWriteOffInvoiceHandler_InvoiceNotFound proves a nonexistent
// Invoice id 404s.
func TestPostWriteOffInvoiceHandler_InvoiceNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "write-off-invoice-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", "write-off")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostManualPaymentHandler_MalformedInvoiceIDReturns400 proves a
// non-UUID :invoiceId 400s.
func TestPostManualPaymentHandler_MalformedInvoiceIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-bad-invoice-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPayment(t, srv, session, practiceID, "not-a-uuid", payments.PaymentMethodCash, "", isoDate(time.Now().UTC()))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostManualPaymentHandler_MalformedBodyReturns400 proves invalid
// JSON 400s.
func TestPostManualPaymentHandler_MalformedBodyReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "manual-payment-bad-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	invoiceID := seedByHandInvoice(t, db, practiceID, contractID)
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postPaymentBody(t, srv, session, practiceID, invoiceID, "{not json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostVoidInvoiceHandler_MalformedInvoiceIDReturns400 proves a
// non-UUID :invoiceId 400s -- shared code path with write-off.
func TestPostVoidInvoiceHandler_MalformedInvoiceIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invoice-bad-id"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := postInvoiceTransition(t, srv, session, practiceID, "not-a-uuid", "void")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// invoiceStatusFor is invoiceStatus with a name that does not collide
// with webhook_test.go's own helper of nearly the same shape (which also
// returns paid_at).
func invoiceStatusFor(t *testing.T, db *testdb.DB, invoiceID string) string {
	t.Helper()
	return invoiceStatus(t, db, invoiceID)
}
