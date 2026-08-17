package payments_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

const (
	stripeObjectEvent                   = "event"
	stripeObjectAccount                 = "account"
	stripeObjectInvoice                 = "invoice"
	stripeEventTypeAccountUpdated       = "account.updated"
	stripeEventTypeInvoicePaid          = "invoice.paid"
	stripeEventTypeInvoicePaymentFailed = "invoice.payment_failed"
	stripeEventTypeUnhandled            = "customer.updated"
	stripeConnectWebhookSecret          = webhookTestSecret
	objectKey                           = "object"
	typeKey                             = "type"
	dataKey                             = "data"
	accountKey                          = "account"
)

func newConnectWebhookServer(db *testdb.DB) *httptest.Server {
	client := payments.NewStripeAPIClient("sk_test_unused", "https://app.test")
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/connect-webhook", payments.PostConnectWebhookHandler(db.App, client, stripeConnectWebhookSecret))
	return httptest.NewServer(mux)
}

// buildConnectEventPayload assembles a raw Stripe event envelope around
// dataObject, tagged with accountID as the event's top-level account
// field -- the same field every Connect event carries.
func buildConnectEventPayload(t *testing.T, eventID, eventType, accountID string, dataObject map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"id":       eventID,
		objectKey:  stripeObjectEvent,
		typeKey:    eventType,
		accountKey: accountID,
		dataKey:    map[string]any{objectKey: dataObject},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// accountUpdatedPayload builds a raw account.updated event body with all
// three capability booleans set to true on its data.object -- the only
// combination this file's tests need; partial-capability states are
// already covered by connect_test.go's GetConnectStatusHandler tests.
func accountUpdatedPayload(t *testing.T, eventID, accountID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeAccountUpdated, accountID, map[string]any{
		"id":                accountID,
		objectKey:           stripeObjectAccount,
		"charges_enabled":   true,
		"payouts_enabled":   true,
		"details_submitted": true,
	})
}

// otherConnectEventPayload builds a raw event of a type this endpoint
// does not handle at all (unlike invoice.paid/invoice.payment_failed,
// which #82 now processes) -- proves a genuinely unhandled event type is
// acknowledged and dropped.
func otherConnectEventPayload(t *testing.T, eventID, accountID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeUnhandled, accountID, map[string]any{"id": "cus_test", objectKey: "customer"})
}

// invoicePaidAmountCents is the amount every invoicePaidPayload reports
// as paid -- matches every seedInvoice call in this file's tests (5000),
// since no partial payment is modeled.
const invoicePaidAmountCents = 5000

// invoicePaidPayload builds a raw invoice.paid event body referencing
// stripeInvoiceID, with the amount actually paid, a payment reference,
// and the Unix paid-at timestamp on its data.object -- mirrors
// accountUpdatedPayload.
func invoicePaidPayload(t *testing.T, eventID, accountID, stripeInvoiceID, paymentReference string, paidAt time.Time) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoicePaid, accountID, map[string]any{
		"id":             stripeInvoiceID,
		objectKey:        stripeObjectInvoice,
		"amount_paid":    invoicePaidAmountCents,
		"payment_intent": paymentReference,
		"status_transitions": map[string]any{
			"paid_at": paidAt.Unix(),
		},
	})
}

// invoicePaymentFailedPayload builds a raw invoice.payment_failed event
// body referencing stripeInvoiceID -- no amount/paid-at fields, since
// #82's handler never creates a payments row on this event.
func invoicePaymentFailedPayload(t *testing.T, eventID, accountID, stripeInvoiceID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoicePaymentFailed, accountID, map[string]any{
		"id":      stripeInvoiceID,
		objectKey: stripeObjectInvoice,
	})
}

func postConnectWebhook(t *testing.T, srv *httptest.Server, payload []byte, signingSecret string) *http.Response {
	t.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/connect-webhook", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Stripe-Signature", signed.Header)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func capabilities(t *testing.T, db *testdb.DB, practiceID string) (charges, payouts, details bool) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_connect_charges_enabled, stripe_connect_payouts_enabled, stripe_connect_details_submitted
		FROM practices WHERE id = $1`, practiceID,
	).Scan(&charges, &payouts, &details); err != nil {
		t.Fatalf("query capabilities: %v", err)
	}
	return
}

// invoiceStatusAndPaidAt reads invoices.status/paid_at directly via the
// superuser Admin connection, bypassing RLS -- mirrors capabilities.
func invoiceStatusAndPaidAt(t *testing.T, db *testdb.DB, invoiceID string) (status string, paidAt *time.Time) {
	t.Helper()
	var paid sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, paid_at FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&status, &paid); err != nil {
		t.Fatalf("query invoice status: %v", err)
	}
	if paid.Valid {
		paidAt = &paid.Time
	}
	return status, paidAt
}

// paymentRow is what paymentsForInvoice reads back, for tests to assert
// against.
type paymentRow struct {
	stripePaymentReference string
	amountCents            int64
	paidAt                 time.Time
}

// paymentsForInvoice reads every payments row for invoiceID directly via
// the superuser Admin connection, bypassing RLS.
func paymentsForInvoice(t *testing.T, db *testdb.DB, invoiceID string) []paymentRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT stripe_payment_reference, amount_cents, paid_at FROM payments WHERE invoice_id = $1`, invoiceID,
	)
	if err != nil {
		t.Fatalf("query payments: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []paymentRow
	for rows.Next() {
		var p paymentRow
		if err := rows.Scan(&p.stripePaymentReference, &p.amountCents, &p.paidAt); err != nil {
			t.Fatalf("scan payment: %v", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate payments: %v", err)
	}
	return out
}

func seedConnectedPractice(t *testing.T, db *testdb.DB, name, accountID string) string {
	t.Helper()
	practiceID := seedPractice(t, db, name)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, practiceID,
	); err != nil {
		t.Fatalf("seed connect account id: %v", err)
	}
	return practiceID
}

// TestPostConnectWebhookHandler_UpdatesCapabilitiesForRecognizedAccount
// proves a validly-signed account.updated event for a known
// stripe_connect_account_id updates all three capability booleans.
func TestPostConnectWebhookHandler_UpdatesCapabilitiesForRecognizedAccount(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Webhook Practice", "acct_recognized")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_update_once", "acct_recognized")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if !charges || !payouts || !details {
		t.Fatalf("capabilities = (%v, %v, %v), want all true", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_ReplayedEventIsNoOp is the explicit
// idempotency test AC calls for: replaying the same Stripe event id must
// not re-apply the status transition (an earlier "false" state should
// stay false, not flip to whatever a stale duplicate delivery carries).
func TestPostConnectWebhookHandler_ReplayedEventIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Replay Practice", "acct_replay")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_replayed", "acct_replay")

	first := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	// Directly revert the capabilities to prove a second delivery of the
	// same event id is a genuine no-op, not just idempotent because the
	// values already matched.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_charges_enabled = false WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("revert capability: %v", err)
	}

	second := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	charges, _, _ := capabilities(t, db, practiceID)
	if charges {
		t.Fatal("charges_enabled = true after replay, want false (replay must be a no-op)")
	}
}

// TestPostConnectWebhookHandler_InvalidSignatureRejected proves a payload
// signed with the wrong secret is rejected without touching the DB.
func TestPostConnectWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Bad Signature Practice", "acct_bad_sig")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_bad_sig", "acct_bad_sig")
	resp := postConnectWebhook(t, srv, payload, "whsec_wrong_secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if charges || payouts || details {
		t.Fatalf("capabilities = (%v, %v, %v), want all false (DB untouched)", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_UnrecognizedAccountDroppedButAcknowledged
// proves an account.updated event for an account id no Practice has
// stored is logged and dropped, not treated as an error -- the webhook
// still returns 200 so Stripe doesn't retry indefinitely.
func TestPostConnectWebhookHandler_UnrecognizedAccountDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := accountUpdatedPayload(t, "evt_unrecognized", "acct_unrecognized")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed
// proves an event type other than account.updated is acknowledged with
// 200 but never updates any Practice's capabilities.
func TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Other Event Practice", "acct_other_event")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := otherConnectEventPayload(t, "evt_other_type", "acct_other_event")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	charges, payouts, details := capabilities(t, db, practiceID)
	if charges || payouts || details {
		t.Fatalf("capabilities = (%v, %v, %v), want all false (untouched)", charges, payouts, details)
	}
}

// TestPostConnectWebhookHandler_OversizedBodyRejected proves a request
// body over maxWebhookBodyBytes is rejected rather than read without
// bound.
func TestPostConnectWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/connect-webhook", bytes.NewReader(oversized))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Stripe-Signature", "t=1,v1=deadbeef")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostConnectWebhookHandler_MalformedAccountObjectRejected proves an
// account.updated event whose data.object can't be unmarshaled into the
// expected shape is rejected rather than silently applying zero values.
func TestPostConnectWebhookHandler_MalformedAccountObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	// charges_enabled as a string (instead of a bool) fails to unmarshal
	// into accountUpdatedObject.
	payload := buildConnectEventPayload(t, "evt_malformed", stripeEventTypeAccountUpdated, "acct_malformed",
		map[string]any{"charges_enabled": "not-a-bool"})

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidCreatesPaymentAndFlipsStatus
// proves a validly-signed invoice.paid event for a known Stripe invoice
// id creates exactly one payments row (amount paid, reference, paid-at)
// and flips the matching Invoice's status to paid with a matching
// paid_at.
func TestPostConnectWebhookHandler_InvoicePaidCreatesPaymentAndFlipsStatus(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Invoice Paid Practice", "acct_invoice_paid")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid_test", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	paidAt := time.Now().Truncate(time.Second).UTC()
	payload := invoicePaidPayload(t, "evt_invoice_paid", "acct_invoice_paid", "in_paid_test", "pi_test_1", paidAt)
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	status, gotPaidAt := invoiceStatusAndPaidAt(t, db, invoiceID)
	if status != "paid" {
		t.Fatalf("invoice status = %q, want %q", status, "paid")
	}
	if gotPaidAt == nil || !gotPaidAt.Equal(paidAt) {
		t.Fatalf("invoice paid_at = %v, want %v", gotPaidAt, paidAt)
	}

	payments := paymentsForInvoice(t, db, invoiceID)
	if len(payments) != 1 {
		t.Fatalf("payments for invoice = %d, want exactly 1", len(payments))
	}
	if payments[0].stripePaymentReference != "pi_test_1" || payments[0].amountCents != 5000 || !payments[0].paidAt.Equal(paidAt) {
		t.Fatalf("payment = %+v, want reference pi_test_1, amount 5000, paidAt %v", payments[0], paidAt)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidReplayIsNoOp is the explicit
// idempotency test AC calls for: replaying the same invoice.paid event id
// must not create a second payments row.
func TestPostConnectWebhookHandler_InvoicePaidReplayIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Invoice Paid Replay Practice", "acct_invoice_paid_replay")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid_replay", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaidPayload(t, "evt_invoice_paid_replay", "acct_invoice_paid_replay", "in_paid_replay", "pi_test_replay", time.Now())

	first := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	payments := paymentsForInvoice(t, db, invoiceID)
	if len(payments) != 1 {
		t.Fatalf("payments for invoice after replay = %d, want exactly 1", len(payments))
	}
}

// TestPostConnectWebhookHandler_InvoicePaymentFailedFlipsStatusWithoutPayment
// proves a validly-signed invoice.payment_failed event flips the matching
// Invoice's status to uncollectible without creating any payments row.
func TestPostConnectWebhookHandler_InvoicePaymentFailedFlipsStatusWithoutPayment(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Invoice Payment Failed Practice", "acct_invoice_failed")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_failed_test", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaymentFailedPayload(t, "evt_invoice_failed", "acct_invoice_failed", "in_failed_test")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	status, _ := invoiceStatusAndPaidAt(t, db, invoiceID)
	if status != "uncollectible" {
		t.Fatalf("invoice status = %q, want %q", status, "uncollectible")
	}
	if payments := paymentsForInvoice(t, db, invoiceID); len(payments) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(payments))
	}
}

// TestPostConnectWebhookHandler_InvoicePaymentFailedReplayIsNoOp proves
// replaying the same invoice.payment_failed event id twice is a no-op --
// the second delivery must not error or re-log the transition.
func TestPostConnectWebhookHandler_InvoicePaymentFailedReplayIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Invoice Payment Failed Replay Practice", "acct_invoice_failed_replay")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_failed_replay", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaymentFailedPayload(t, "evt_invoice_failed_replay", "acct_invoice_failed_replay", "in_failed_replay")

	first := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replayed delivery status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	status, _ := invoiceStatusAndPaidAt(t, db, invoiceID)
	if status != "uncollectible" {
		t.Fatalf("invoice status = %q, want %q", status, "uncollectible")
	}
}

// TestPostConnectWebhookHandler_InvoicePaymentFailedUnknownInvoiceDroppedButAcknowledged
// proves an invoice.payment_failed event referencing a Stripe invoice id
// Doula Cloud has no invoices row for is logged and dropped, not treated
// as an error.
func TestPostConnectWebhookHandler_InvoicePaymentFailedUnknownInvoiceDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "Unknown Invoice Failed Practice", "acct_unknown_invoice_failed")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaymentFailedPayload(t, "evt_unknown_invoice_failed", "acct_unknown_invoice_failed", "in_does_not_exist_failed")
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidUnknownInvoiceDroppedButAcknowledged
// proves an invoice.paid event referencing a Stripe invoice id Doula
// Cloud has no invoices row for is logged and dropped, not treated as an
// error.
func TestPostConnectWebhookHandler_InvoicePaidUnknownInvoiceDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "Unknown Invoice Practice", "acct_unknown_invoice")
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaidPayload(t, "evt_unknown_invoice", "acct_unknown_invoice", "in_does_not_exist", "pi_unknown", time.Now())
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidUnrecognizedAccountDroppedButAcknowledged
// proves an invoice.paid event whose account field matches no Practice is
// logged and dropped, even though the Stripe invoice id itself is known
// under a different account -- the Invoice's status stays untouched.
func TestPostConnectWebhookHandler_InvoicePaidUnrecognizedAccountDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Wrong Account Practice", "acct_right_owner")
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_wrong_account", invoiceStatusOpen, 5000, time.Now())
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaidPayload(t, "evt_wrong_account", "acct_unrecognized_for_invoice", "in_wrong_account", "pi_wrong_account", time.Now())
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	status, _ := invoiceStatusAndPaidAt(t, db, invoiceID)
	if status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q (untouched)", status, invoiceStatusOpen)
	}
	if payments := paymentsForInvoice(t, db, invoiceID); len(payments) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(payments))
	}
}

// TestPostConnectWebhookHandler_InvoicePaidMalformedObjectRejected proves
// an invoice.paid event whose data.object can't be unmarshaled into the
// expected shape is rejected rather than silently applying zero values.
func TestPostConnectWebhookHandler_InvoicePaidMalformedObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	// amount_paid as a string (instead of an integer) fails to unmarshal
	// into invoicePaidObject.
	payload := buildConnectEventPayload(t, "evt_invoice_paid_malformed", stripeEventTypeInvoicePaid, "acct_malformed",
		map[string]any{"id": "in_malformed", "amount_paid": "not-a-number"})

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestPostConnectWebhookHandler_InvoicePaymentFailedMalformedObjectRejected
// proves an invoice.payment_failed event whose data.object can't be
// unmarshaled into the expected shape is rejected rather than silently
// applying zero values.
func TestPostConnectWebhookHandler_InvoicePaymentFailedMalformedObjectRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	// id as a number (instead of a string) fails to unmarshal into
	// invoicePaymentFailedObject.
	payload := buildConnectEventPayload(t, "evt_invoice_failed_malformed", stripeEventTypeInvoicePaymentFailed, "acct_malformed",
		map[string]any{"id": 12345})

	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
