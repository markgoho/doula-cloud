package payments_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// errRetrieveFailed stands in for any failure to reach Stripe.
var errRetrieveFailed = errors.New("stripe unreachable")

const (
	stripeObjectEvent   = "event"
	stripeObjectV2Event = "v2.core.event"
	// requirementMCC is one real Stripe requirement path, used across
	// both the status-read and webhook-persist tests.
	requirementMCC                      = "configuration.merchant.mcc"
	stripeObjectAccount                 = "account"
	stripeObjectInvoice                 = "invoice"
	stripeEventTypeCapabilityStatus     = "v2.core.account[configuration.merchant].capability_status_updated"
	stripeEventTypeAccountCreated       = "v2.core.account.created"
	stripeEventTypeInvoicePaid          = "invoice.paid"
	stripeEventTypeInvoicePaymentFailed = "invoice.payment_failed"
	stripeEventTypeUnhandled            = "customer.updated"
	stripeConnectWebhookSecret          = webhookTestSecret
	stripeAccountWebhookSecret          = webhookTestSecret
	objectKey                           = "object"
	typeKey                             = "type"
	dataKey                             = "data"
	accountKey                          = "account"
)

func newConnectWebhookServer(db *testdb.DB) *httptest.Server {
	return newConnectWebhookServerWith(db, payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))
}

// newConnectWebhookServerWith is newConnectWebhookServer with the Client
// chosen by the caller. invoice.paid now reads the Stripe payment
// reference back through the port (the event no longer carries one), so
// those tests need a Client that can answer that call as well as verify
// the signature.
func newConnectWebhookServerWith(db *testdb.DB, client payments.Client) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/connect-webhook", payments.PostConnectWebhookHandler(db.App, client, stripeConnectWebhookSecret))
	return httptest.NewServer(mux)
}

// referenceClient answers RetrieveInvoicePaymentReference while leaving
// real signature verification to the embedded StripeAPIClient -- the same
// seam accountFetchClient uses for the thin-event route.
type referenceClient struct {
	*payments.StripeAPIClient
	reference string
	err       error
}

func (c *referenceClient) RetrieveInvoicePaymentReference(_ context.Context, _, _ string) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	return c.reference, nil
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

// accountEventPayload builds a raw v2 thin event notification. A thin
// notification carries no object at all -- only the header fields and a
// related_object pointing at what changed -- which is exactly why the
// handler has to fetch the account rather than read the payload.
func accountEventPayload(t *testing.T, eventID, eventType, accountID string) []byte {
	t.Helper()
	body := map[string]any{
		"id": eventID,
		// A thin notification's object discriminator, not "event" --
		// stripe-go refuses to parse one as the other in either
		// direction (v2_events.go's EventNotificationFromJSON).
		objectKey: stripeObjectV2Event,
		typeKey:   eventType,
		"created": time.Now().UTC().Format(time.RFC3339),
		"related_object": map[string]any{
			"id":    accountID,
			typeKey: "v2.core.account",
			"url":   "/v2/core/accounts/" + accountID,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// capabilityStatusPayload builds the one thin event
// PostAccountWebhookHandler acts on.
func capabilityStatusPayload(t *testing.T, eventID, accountID string) []byte {
	t.Helper()
	return accountEventPayload(t, eventID, stripeEventTypeCapabilityStatus, accountID)
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
// stripeInvoiceID, with the amount actually paid and the Unix paid-at
// timestamp on its data.object.
//
// There is deliberately no payment_intent here. A real invoice.paid body
// under API version 2026-07-29.dahlia carries none -- nor a charge, nor a
// payments list (checked against the Sandbox during #247's walk) -- so a
// fixture that included one would let a handler pass a test it could not
// pass in production. The reference comes from the Client port instead.
func invoicePaidPayload(t *testing.T, eventID, accountID, stripeInvoiceID string, paidAt time.Time) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoicePaid, accountID, map[string]any{
		"id":          stripeInvoiceID,
		objectKey:     stripeObjectInvoice,
		"amount_paid": invoicePaidAmountCents,
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

// connectState reads the persisted v2 Connect state directly via the
// superuser Admin connection, bypassing RLS. eventID is what makes a
// change traceable back to the delivery that caused it (the audit-trail
// expectation in CLAUDE.md).
func connectState(t *testing.T, db *testdb.DB, practiceID string) (cardPayments, payouts string, requirements []string, eventID sql.NullString) {
	t.Helper()
	// The text[] column comes back through array_to_json rather than
	// scanned directly: database/sql has no array type, so the shape a
	// plain Scan gets depends on the driver rather than on what was
	// stored. JSON is the same bytes on any of them.
	var requirementsJSON string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_connect_card_payments_status, stripe_connect_payouts_status,
			array_to_json(stripe_connect_requirements_due)::text, stripe_connect_status_event_id
		FROM practices WHERE id = $1`, practiceID,
	).Scan(&cardPayments, &payouts, &requirementsJSON, &eventID); err != nil {
		t.Fatalf("query connect state: %v", err)
	}
	if err := json.Unmarshal([]byte(requirementsJSON), &requirements); err != nil {
		t.Fatalf("decode requirements array: %v", err)
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

// accountFetchClient is the seam PostAccountWebhookHandler's tests need:
// real signature verification (the embedded StripeAPIClient's
// ParseAccountEvent, which is a pure computation) over a fake account
// fetch, because a thin event carries no state and the handler must go
// and get it. Embedding promotes every other Client method unchanged.
type accountFetchClient struct {
	*payments.StripeAPIClient
	status  payments.AccountStatus
	err     error
	fetched []string
}

func (c *accountFetchClient) RetrieveAccount(_ context.Context, accountID string) (payments.AccountStatus, error) {
	c.fetched = append(c.fetched, accountID)
	if c.err != nil {
		return payments.AccountStatus{}, c.err
	}
	return c.status, nil
}

func newAccountWebhookServer(db *testdb.DB, client payments.Client) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/account-webhook", payments.PostAccountWebhookHandler(db.App, client, stripeAccountWebhookSecret))
	return httptest.NewServer(mux)
}

func postAccountWebhook(t *testing.T, srv *httptest.Server, payload []byte, signingSecret string) *http.Response {
	t.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/stripe/account-webhook", bytes.NewReader(payload))
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

// activeAccountClient is the common fixture: a fully onboarded account
// with nothing outstanding.
func activeAccountClient() *accountFetchClient {
	return &accountFetchClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		status: payments.AccountStatus{
			CardPayments:    payments.CapabilityActive,
			Payouts:         payments.CapabilityActive,
			RequirementsDue: []string{},
		},
	}
}

// TestPostAccountWebhookHandler_PersistsFetchedCapabilityStatuses proves
// the whole point of the thin-event path: the event says only that
// *something* changed, so the handler fetches the account and writes
// what Stripe actually reports, tagged with the event id that caused it.
func TestPostAccountWebhookHandler_PersistsFetchedCapabilityStatuses(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Account Webhook Practice", "acct_recognized")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := capabilityStatusPayload(t, "evt_capability_once", "acct_recognized")
	resp := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(client.fetched) != 1 || client.fetched[0] != "acct_recognized" {
		t.Fatalf("fetched = %v, want one retrieve of the account named in related_object", client.fetched)
	}
	cardPayments, payouts, requirements, eventID := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityActive) || payouts != string(payments.CapabilityActive) {
		t.Fatalf("statuses = (%q, %q), want both active", cardPayments, payouts)
	}
	if len(requirements) != 0 {
		t.Fatalf("requirements = %v, want none outstanding", requirements)
	}
	if !eventID.Valid || eventID.String != "evt_capability_once" {
		t.Fatalf("event id = %+v, want the delivering event recorded for audit", eventID)
	}
}

// TestPostAccountWebhookHandler_PersistsOutstandingRequirements proves
// the replacement for v1's details_submitted: what is still owed, not
// merely that something is.
func TestPostAccountWebhookHandler_PersistsOutstandingRequirements(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Requirements Practice", "acct_requirements")
	client := activeAccountClient()
	client.status = payments.AccountStatus{
		CardPayments:    payments.CapabilityRestricted,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: []string{requirementMCC, "identity.business_details.registered_name"},
	}
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := capabilityStatusPayload(t, "evt_requirements", "acct_requirements")
	resp := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_, _, requirements, _ := connectState(t, db, practiceID)
	if len(requirements) != 2 || requirements[0] != requirementMCC {
		t.Fatalf("requirements = %v, want both outstanding field paths in order", requirements)
	}
}

// TestPostAccountWebhookHandler_NilRequirementsPersistAsEmptyArray
// proves a Client implementation that leaves RequirementsDue nil does
// not violate the column's NOT NULL.
func TestPostAccountWebhookHandler_NilRequirementsPersistAsEmptyArray(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Nil Requirements Practice", "acct_nil_reqs")
	client := activeAccountClient()
	client.status = payments.AccountStatus{
		CardPayments:    payments.CapabilityActive,
		Payouts:         payments.CapabilityActive,
		RequirementsDue: nil,
	}
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	resp := postAccountWebhook(t, srv, capabilityStatusPayload(t, "evt_nil_reqs", "acct_nil_reqs"), stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_, _, requirements, _ := connectState(t, db, practiceID)
	if requirements == nil || len(requirements) != 0 {
		t.Fatalf("requirements = %v, want an empty array", requirements)
	}
}

// TestPostAccountWebhookHandler_ReplayedEventIsNoOp is the idempotency
// test: replaying the same Stripe event id must not re-apply anything.
func TestPostAccountWebhookHandler_ReplayedEventIsNoOp(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Account Replay Practice", "acct_replay")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := capabilityStatusPayload(t, "evt_account_replayed", "acct_replay")

	first := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first delivery status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	// Move the row back behind what the event carried. A replay that
	// re-applied would flip it forward again.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_card_payments_status = 'restricted' WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("reset status: %v", err)
	}

	second := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d, want %d", second.StatusCode, http.StatusOK)
	}
	cardPayments, _, _, _ := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityRestricted) {
		t.Fatalf("card_payments = %q after replay, want the reset value to stand", cardPayments)
	}
}

// TestPostAccountWebhookHandler_InvalidSignatureRejected proves an
// unsigned or wrongly-signed thin event never reaches the database.
func TestPostAccountWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Account Bad Signature Practice", "acct_bad_sig")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := capabilityStatusPayload(t, "evt_account_bad_sig", "acct_bad_sig")
	resp := postAccountWebhook(t, srv, payload, "whsec_wrong_secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if len(client.fetched) != 0 {
		t.Fatalf("fetched = %v, want no Stripe call on a rejected signature", client.fetched)
	}
	cardPayments, _, _, _ := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityUnsupported) {
		t.Fatalf("card_payments = %q, want the default to stand", cardPayments)
	}
}

// TestPostAccountWebhookHandler_UnrecognizedAccountDroppedButAcknowledged
// proves an event for an account no Practice claims is dropped with a
// 200 -- Stripe retries indefinitely on anything else.
func TestPostAccountWebhookHandler_UnrecognizedAccountDroppedButAcknowledged(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "Account Unrecognized Practice", "acct_known")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	resp := postAccountWebhook(t, srv, capabilityStatusPayload(t, "evt_account_unrecognized", "acct_unrecognized"), stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPostAccountWebhookHandler_OtherEventTypesAcknowledgedNotProcessed
// proves the account events Stripe also sends -- created,
// [identity].updated and the rest -- are dropped without a Stripe fetch.
func TestPostAccountWebhookHandler_OtherEventTypesAcknowledgedNotProcessed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Account Other Event Practice", "acct_other_event")
	client := activeAccountClient()
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := accountEventPayload(t, "evt_account_created", stripeEventTypeAccountCreated, "acct_other_event")
	resp := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if len(client.fetched) != 0 {
		t.Fatalf("fetched = %v, want no Stripe call for an unhandled event type", client.fetched)
	}
	cardPayments, _, _, _ := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityUnsupported) {
		t.Fatalf("card_payments = %q, want the default to stand", cardPayments)
	}
}

// TestPostAccountWebhookHandler_RetrieveFailureReturns500 proves a
// failure to reach Stripe is a 500 and claims nothing, so Stripe's retry
// is not swallowed later as a replay.
func TestPostAccountWebhookHandler_RetrieveFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Account Retrieve Failure Practice", "acct_retrieve_fail")
	client := activeAccountClient()
	client.err = errRetrieveFailed
	srv := newAccountWebhookServer(db, client)
	defer srv.Close()

	payload := capabilityStatusPayload(t, "evt_retrieve_fail", "acct_retrieve_fail")
	resp := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	// The retry succeeds, proving the failed delivery claimed no event id.
	client.err = nil
	retry := postAccountWebhook(t, srv, payload, stripeAccountWebhookSecret)
	defer retry.Body.Close()
	if retry.StatusCode != http.StatusOK {
		t.Fatalf("retry status = %d, want %d", retry.StatusCode, http.StatusOK)
	}
	cardPayments, _, _, _ := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityActive) {
		t.Fatalf("card_payments = %q after retry, want active", cardPayments)
	}
}

// TestPostAccountWebhookHandler_OversizedBodyRejected mirrors the
// Connect endpoint's own body bound.
func TestPostAccountWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	seedConnectedPractice(t, db, "Account Oversized Practice", "acct_account_oversized")
	srv := newAccountWebhookServer(db, activeAccountClient())
	defer srv.Close()

	resp := postAccountWebhook(t, srv, bytes.Repeat([]byte("a"), (1<<20)+1), stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

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

// TestPostConnectWebhookHandler_InvalidSignatureRejected proves a
// wrongly-signed snapshot event never reaches the database. It uses an
// invoice event because the Connect endpoint no longer carries account
// events at all -- Accounts v2 sends those as thin events to
// PostAccountWebhookHandler (#247).
func TestPostConnectWebhookHandler_InvalidSignatureRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	payload := invoicePaymentFailedPayload(t, "evt_bad_sig", "acct_bad_sig", "in_bad_sig")
	resp := postConnectWebhook(t, srv, payload, "whsec_wrong_secret")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed
// proves a genuinely unhandled snapshot event type is acknowledged and
// dropped rather than erroring, since Stripe retries on anything but a
// 2xx.
func TestPostConnectWebhookHandler_OtherEventTypesAcknowledgedNotProcessed(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	resp := postConnectWebhook(t, srv, otherConnectEventPayload(t, "evt_other", "acct_other"), stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
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
	srv := newConnectWebhookServerWith(db, &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		reference:       "pi_test_1",
	})
	defer srv.Close()

	paidAt := time.Now().Truncate(time.Second).UTC()
	payload := invoicePaidPayload(t, "evt_invoice_paid", "acct_invoice_paid", "in_paid_test", paidAt)
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

	payload := invoicePaidPayload(t, "evt_invoice_paid_replay", "acct_invoice_paid_replay", "in_paid_replay", time.Now())

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

	payload := invoicePaidPayload(t, "evt_unknown_invoice", "acct_unknown_invoice", "in_does_not_exist", time.Now())
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

	payload := invoicePaidPayload(t, "evt_wrong_account", "acct_unrecognized_for_invoice", "in_wrong_account", time.Now())
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

// TestPostAccountWebhookHandler_FakeClientEventIsDroppedNotApplied covers
// the FakeClient path other packages' route tests go through: the double
// verifies nothing and reports a zero-value AccountEvent, whose empty
// type must be dropped and acknowledged rather than mistaken for a
// capability update. Injecting the double is what lets a caller with no
// database prove the route is mounted.
func TestPostAccountWebhookHandler_FakeClientEventIsDroppedNotApplied(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Fake Client Practice", "acct_fake_client")
	srv := newAccountWebhookServer(db, payments.NewFakeClient())
	defer srv.Close()

	resp := postAccountWebhook(t, srv, capabilityStatusPayload(t, "evt_fake_client", "acct_fake_client"), stripeAccountWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	cardPayments, _, _, _ := connectState(t, db, practiceID)
	if cardPayments != string(payments.CapabilityUnsupported) {
		t.Fatalf("card_payments = %q, want the default to stand", cardPayments)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidRecordsFetchedPaymentReference
// pins the fix for what #247's walk found: under API version
// 2026-07-29.dahlia an Invoice carries no payment_intent and no charge,
// and the invoice.paid body carries no payments list, so the old
// `payment_intent` JSON tag silently unmarshaled to "" and every payments
// row was written with an empty Stripe reference. The reference now comes
// through the port.
func TestPostConnectWebhookHandler_InvoicePaidRecordsFetchedPaymentReference(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Reference Practice", "acct_reference")
	engagementID := seedEngagement(t, db, practiceID, "Ref Client", "ref@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_reference", invoiceStatusOpen, invoicePaidAmountCents, time.Now())
	client := &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		reference:       "pi_from_stripe",
	}
	srv := newConnectWebhookServerWith(db, client)
	defer srv.Close()

	payload := invoicePaidPayload(t, "evt_reference", "acct_reference", "in_reference", time.Unix(1_700_000_000, 0))
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var reference string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_payment_reference FROM payments WHERE invoice_id = $1`, invoiceID,
	).Scan(&reference); err != nil {
		t.Fatalf("read payment reference: %v", err)
	}
	if reference != "pi_from_stripe" {
		t.Fatalf("stripe_payment_reference = %q, want the fetched PaymentIntent id", reference)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidSurvivesReferenceLookupFailure
// proves a failure to fetch the reference does not lose the payment. The
// money has already moved; answering 500 would make Stripe redeliver an
// event we cannot improve on, so the row is written with an empty
// reference and the failure is logged.
func TestPostConnectWebhookHandler_InvoicePaidSurvivesReferenceLookupFailure(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedConnectedPractice(t, db, "Reference Failure Practice", "acct_ref_fail")
	engagementID := seedEngagement(t, db, practiceID, "Fail Client", "fail@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_ref_fail", invoiceStatusOpen, invoicePaidAmountCents, time.Now())
	client := &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		err:             errRetrieveFailed,
	}
	srv := newConnectWebhookServerWith(db, client)
	defer srv.Close()

	payload := invoicePaidPayload(t, "evt_ref_fail", "acct_ref_fail", "in_ref_fail", time.Unix(1_700_000_000, 0))
	resp := postConnectWebhook(t, srv, payload, stripeConnectWebhookSecret)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d -- the payment must not be lost", resp.StatusCode, http.StatusOK)
	}
	var reference string
	var amount int64
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_payment_reference, amount_cents FROM payments WHERE invoice_id = $1`, invoiceID,
	).Scan(&reference, &amount); err != nil {
		t.Fatalf("read payment: %v", err)
	}
	if reference != "" {
		t.Fatalf("stripe_payment_reference = %q, want empty", reference)
	}
	if amount != invoicePaidAmountCents {
		t.Fatalf("amount_cents = %d, want %d", amount, invoicePaidAmountCents)
	}
}
