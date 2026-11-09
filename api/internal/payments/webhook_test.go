package payments_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v86"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/stripemode"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/webhookarrival"
)

// errRetrieveFailed stands in for any failure to reach Stripe.
var errRetrieveFailed = errors.New("stripe unreachable")

// wrongSecret signs a delivery the endpoint must reject.
const wrongSecret = webhookTestSecret + "_wrong"

const (
	stripeObjectEvent   = "event"
	stripeObjectV2Event = "v2.core.event"
	// requirementMCC is one real Stripe requirement path, used across
	// both the status-read and webhook-persist tests.
	requirementMCC                            = "configuration.merchant.mcc"
	stripeObjectAccount                       = "account"
	stripeObjectInvoice                       = "invoice"
	stripeEventTypeCapabilityStatus           = "v2.core.account[configuration.merchant].capability_status_updated"
	stripeEventTypeAccountCreated             = "v2.core.account.created"
	stripeEventTypeInvoicePaid                = "invoice.paid"
	stripeEventTypeInvoicePaymentFailed       = "invoice.payment_failed"
	stripeEventTypeInvoiceMarkedUncollectible = "invoice.marked_uncollectible"
	stripeEventTypeUnhandled                  = "customer.updated"
	stripeConnectWebhookSecret                = webhookTestSecret
	stripeAccountWebhookSecret                = webhookTestSecret
	objectKey                                 = "object"
	typeKey                                   = "type"
	dataKey                                   = "data"
	accountKey                                = "account"
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
	return newConnectWebhookServerWithEnqueuer(db, client, &tasknudge.FakeEnqueuer{})
}

// newConnectWebhookServerWithEnqueuer is newConnectWebhookServerWith with
// the tasknudge.Enqueuer also chosen by the caller -- ADR-0013's tests
// use this to inject a FakeEnqueuer configured to fail.
func newConnectWebhookServerWithEnqueuer(db *testdb.DB, client payments.Client, enq tasknudge.Enqueuer) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/connect-webhook", webhookarrival.Record(db.App, webhookarrival.StripeConnect,
		payments.PostConnectWebhookHandler(db.App, client, stripeConnectWebhookSecret, stripemode.Test, enq)))
	return httptest.NewServer(mux)
}

// referenceClient answers RetrieveInvoicePaymentReference while leaving
// real signature verification to the embedded StripeAPIClient -- the same
// seam accountFetchClient uses for the thin-event route.
type referenceClient struct {
	*payments.StripeAPIClient
	reference string
	err       error
	method    string
	awaiting  bool
	methodErr error
}

func (c *referenceClient) RetrieveInvoicePaymentReference(_ context.Context, _, _ string) (string, error) {
	if c.err != nil {
		return "", c.err
	}
	return c.reference, nil
}

func (c *referenceClient) RetrieveInvoiceFailedAttempt(_ context.Context, _, _ string) (payments.FailedAttempt, error) {
	if c.methodErr != nil {
		return payments.FailedAttempt{}, c.methodErr
	}
	return payments.FailedAttempt{PaymentMethodType: c.method, AwaitingAction: c.awaiting}, nil
}

// newReferenceClient is a referenceClient that answers every payment
// reference lookup with reference.
func newReferenceClient(reference string) *referenceClient {
	return &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		reference:       reference,
	}
}

// connectHook mounts the Connect webhook on a server of its own, answering
// through client.
func (e *env) connectHook(client payments.Client) *httptest.Server {
	e.t.Helper()
	srv := newConnectWebhookServerWith(e.db, client)
	e.t.Cleanup(srv.Close)
	return srv
}

// seedStripeInvoice seeds an Engagement at the Practice with an Invoice
// Stripe issued as stripeInvoiceID, in status.
func (e *env) seedStripeInvoice(stripeInvoiceID, status string, amountCents int64) (engagementID, invoiceID string) {
	e.t.Helper()
	engagementID, contractID := e.contractWithStatus("draft")
	return engagementID, seedInvoice(e.t, e.db, e.Practice.ID, contractID, stripeInvoiceID, status, amountCents, time.Now())
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
	return accountEventPayloadAt(t, eventID, eventType, accountID, time.Now())
}

// accountEventPayloadAt is accountEventPayload with Stripe's own `created`
// time chosen by the caller, for the history tests (#1812) that assert the
// row keeps it.
func accountEventPayloadAt(t *testing.T, eventID, eventType, accountID string, created time.Time) []byte {
	t.Helper()
	body := map[string]any{
		"id": eventID,
		// A thin notification's object discriminator, not "event" --
		// stripe-go refuses to parse one as the other in either
		// direction (v2_events.go's EventNotificationFromJSON).
		objectKey: stripeObjectV2Event,
		typeKey:   eventType,
		"created": created.UTC().Format(time.RFC3339),
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

	resp := reqtest.Post(t, srv.URL+"/stripe/connect-webhook", reqtest.Body("", payload), reqtest.Header("Stripe-Signature", signed.Header))
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
	practiceID := testdb.SeedPractice(t, db, name)
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
	return newAccountWebhookServerWithEnqueuer(db, client, &tasknudge.FakeEnqueuer{})
}

// newAccountWebhookServerWithEnqueuer is newAccountWebhookServer with the
// tasknudge.Enqueuer also chosen by the caller, mirroring
// newConnectWebhookServerWithEnqueuer.
func newAccountWebhookServerWithEnqueuer(db *testdb.DB, client payments.Client, enq tasknudge.Enqueuer) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /stripe/account-webhook", webhookarrival.Record(db.App, webhookarrival.StripeAccount,
		payments.PostAccountWebhookHandler(db.App, client, stripeAccountWebhookSecret, stripemode.Test, enq)))
	return httptest.NewServer(mux)
}

func postAccountWebhook(t *testing.T, srv *httptest.Server, payload []byte, signingSecret string) *http.Response {
	t.Helper()
	signed := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: payload, Secret: signingSecret})

	resp := reqtest.Post(t, srv.URL+"/stripe/account-webhook", reqtest.Body("", payload), reqtest.Header("Stripe-Signature", signed.Header))
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

// accountHook mounts the account webhook on a server of its own, answering
// through client.
func (e *env) accountHook(client payments.Client) *httptest.Server {
	e.t.Helper()
	srv := newAccountWebhookServer(e.db, client)
	e.t.Cleanup(srv.Close)
	return srv
}

// deliverAccount posts payload to the account webhook, signed with its
// secret, and requires the status want.
func deliverAccount(t *testing.T, srv *httptest.Server, payload []byte, want int) {
	t.Helper()
	deliverAccountSignedWith(t, srv, payload, stripeAccountWebhookSecret, want)
}

func deliverAccountSignedWith(t *testing.T, srv *httptest.Server, payload []byte, secret string, want int) {
	t.Helper()
	resp := postAccountWebhook(t, srv, payload, secret)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

// deliverConnectSignedWith posts payload to the Connect webhook, signed
// with secret, and requires the status want.
func deliverConnectSignedWith(t *testing.T, srv *httptest.Server, payload []byte, secret string, want int) {
	t.Helper()
	resp := postConnectWebhook(t, srv, payload, secret)
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

// restrictedAccountClient is an account whose capabilities are restricted
// with the given requirements outstanding.
func restrictedAccountClient(due ...string) *accountFetchClient {
	client := activeAccountClient()
	client.status = payments.AccountStatus{
		CardPayments:    payments.CapabilityRestricted,
		Payouts:         payments.CapabilityRestricted,
		RequirementsDue: due,
	}
	return client
}

// TestPostAccountWebhookHandler_PersistsFetchedCapabilityStatuses proves
// the whole point of the thin-event path: the event says only that
// *something* changed, so the handler fetches the account and writes
// what Stripe actually reports, tagged with the event id that caused it.
func TestPostAccountWebhookHandler_PersistsFetchedCapabilityStatuses(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	client := activeAccountClient()

	deliverAccount(t, e.accountHook(client), capabilityStatusPayload(t, "evt_capability_once", e.AccountID), http.StatusOK)

	if len(client.fetched) != 1 || client.fetched[0] != e.AccountID {
		t.Fatalf("fetched = %v, want one retrieve of the account named in related_object", client.fetched)
	}
	cardPayments, payouts, requirements, eventID := connectState(t, e.db, e.Practice.ID)
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
	e := newEnv(t, connectAccountOnly())
	client := restrictedAccountClient(requirementMCC, "identity.business_details.registered_name")

	deliverAccount(t, e.accountHook(client), capabilityStatusPayload(t, "evt_requirements", e.AccountID), http.StatusOK)

	_, _, requirements, _ := connectState(t, e.db, e.Practice.ID)
	if len(requirements) != 2 || requirements[0] != requirementMCC {
		t.Fatalf("requirements = %v, want both outstanding field paths in order", requirements)
	}
}

// payoutOutboxForPractice returns the status of every payout_outbox row
// for practiceID, oldest first, so a test can assert both how many
// episodes queued a row and what became of each.
func payoutOutboxForPractice(t *testing.T, db *testdb.DB, practiceID string) []string {
	t.Helper()
	statuses := testdb.Column[string](t, db.Admin, `SELECT status FROM payout_outbox WHERE practice_id = $1 ORDER BY created_at`, practiceID)
	return statuses
}

// TestPostAccountWebhookHandler_QueuesPayoutNotificationOnEmptyToNonEmptyTransition
// proves #343's trigger: a fresh account (requirements_due starts empty,
// 00034's column default) that gets its first non-empty requirements_due
// queues exactly one payout_outbox row.
func TestPostAccountWebhookHandler_QueuesPayoutNotificationOnEmptyToNonEmptyTransition(t *testing.T) {
	e := newEnv(t, connectAccountOnly())

	deliverAccount(t, e.accountHook(restrictedAccountClient(requirementMCC)), capabilityStatusPayload(t, "evt_payout_trigger", e.AccountID), http.StatusOK)

	statuses := payoutOutboxForPractice(t, e.db, e.Practice.ID)
	if len(statuses) != 1 || statuses[0] != testPayoutStatusPending {
		t.Fatalf("payout_outbox rows = %v, want exactly one pending row", statuses)
	}
}

// TestPostAccountWebhookHandler_NoPayoutNotificationWhenRequirementsStayEmpty
// proves an event that reports nothing outstanding never queues a row.
func TestPostAccountWebhookHandler_NoPayoutNotificationWhenRequirementsStayEmpty(t *testing.T) {
	e := newEnv(t, connectAccountOnly())

	deliverAccount(t, e.accountHook(activeAccountClient()), capabilityStatusPayload(t, "evt_payout_no_trigger", e.AccountID), http.StatusOK)

	if statuses := payoutOutboxForPractice(t, e.db, e.Practice.ID); len(statuses) != 0 {
		t.Fatalf("payout_outbox rows = %v, want none for an account with nothing outstanding", statuses)
	}
}

// TestPostAccountWebhookHandler_DoesNotRequeuePayoutNotificationWhileRequirementsStayOutstanding
// proves the episode discipline: a second delivery whose requirements_due
// is still non-empty (even a different set of fields) does not queue a
// second row while the first is still pending.
func TestPostAccountWebhookHandler_DoesNotRequeuePayoutNotificationWhileRequirementsStayOutstanding(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	client := restrictedAccountClient(requirementMCC)
	srv := e.accountHook(client)

	deliverAccount(t, srv, capabilityStatusPayload(t, "evt_payout_first", e.AccountID), http.StatusOK)
	client.status.RequirementsDue = []string{requirementMCC, testRequirementDOB}
	deliverAccount(t, srv, capabilityStatusPayload(t, "evt_payout_second", e.AccountID), http.StatusOK)

	if statuses := payoutOutboxForPractice(t, e.db, e.Practice.ID); len(statuses) != 1 {
		t.Fatalf("payout_outbox rows = %v, want still exactly one row for the same open episode", statuses)
	}
}

// TestPostAccountWebhookHandler_RequeuesPayoutNotificationAfterEpisodeReopens
// proves re-arming: once the first episode's row has actually been sent
// (billing.Worker's job, simulated here) and requirements clear and then
// reappear, a second, independent row queues.
func TestPostAccountWebhookHandler_RequeuesPayoutNotificationAfterEpisodeReopens(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	client := restrictedAccountClient(requirementMCC)
	srv := e.accountHook(client)

	deliverAccount(t, srv, capabilityStatusPayload(t, "evt_payout_reopen_open", e.AccountID), http.StatusOK)
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`UPDATE payout_outbox SET status = 'sent' WHERE practice_id = $1`, e.Practice.ID,
	); err != nil {
		t.Fatalf("simulate worker send: %v", err)
	}

	client.status.RequirementsDue = []string{}
	deliverAccount(t, srv, capabilityStatusPayload(t, "evt_payout_reopen_clear", e.AccountID), http.StatusOK)
	client.status.RequirementsDue = []string{requirementMCC}
	deliverAccount(t, srv, capabilityStatusPayload(t, "evt_payout_reopen_again", e.AccountID), http.StatusOK)

	statuses := payoutOutboxForPractice(t, e.db, e.Practice.ID)
	if len(statuses) != 2 || statuses[0] != testPayoutStatusSent || statuses[1] != testPayoutStatusPending {
		t.Fatalf("payout_outbox rows = %v, want [sent pending] -- the first episode sent, a second queued after it reopened", statuses)
	}
}

// TestPostAccountWebhookHandler_ReplayDoesNotDoubleQueuePayoutNotification
// proves the general idempotency guarantee (claimEvent's alreadyProcessed
// short-circuit) covers this new write too: replaying the event that
// queued a row must not queue a second one.
func TestPostAccountWebhookHandler_ReplayDoesNotDoubleQueuePayoutNotification(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	srv := e.accountHook(restrictedAccountClient(requirementMCC))
	payload := capabilityStatusPayload(t, "evt_payout_replay", e.AccountID)

	deliverAccount(t, srv, payload, http.StatusOK)
	deliverAccount(t, srv, payload, http.StatusOK)

	if statuses := payoutOutboxForPractice(t, e.db, e.Practice.ID); len(statuses) != 1 {
		t.Fatalf("payout_outbox rows = %v, want exactly one despite the replay", statuses)
	}
}

// TestPostAccountWebhookHandler_NilRequirementsPersistAsEmptyArray
// proves a Client implementation that leaves RequirementsDue nil does
// not violate the column's NOT NULL.
func TestPostAccountWebhookHandler_NilRequirementsPersistAsEmptyArray(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	client := activeAccountClient()
	client.status.RequirementsDue = nil

	deliverAccount(t, e.accountHook(client), capabilityStatusPayload(t, "evt_nil_reqs", e.AccountID), http.StatusOK)

	_, _, requirements, _ := connectState(t, e.db, e.Practice.ID)
	if requirements == nil || len(requirements) != 0 {
		t.Fatalf("requirements = %v, want an empty array", requirements)
	}
}

// TestPostAccountWebhookHandler_ReplayedEventIsNoOp is the idempotency
// test: replaying the same Stripe event id must not re-apply anything.
func TestPostAccountWebhookHandler_ReplayedEventIsNoOp(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	srv := e.accountHook(activeAccountClient())
	payload := capabilityStatusPayload(t, "evt_account_replayed", e.AccountID)

	deliverAccount(t, srv, payload, http.StatusOK)
	// Move the row back behind what the event carried. A replay that
	// re-applied would flip it forward again.
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_card_payments_status = 'restricted' WHERE id = $1`, e.Practice.ID,
	); err != nil {
		t.Fatalf("reset status: %v", err)
	}
	deliverAccount(t, srv, payload, http.StatusOK)

	cardPayments, _, _, _ := connectState(t, e.db, e.Practice.ID)
	if cardPayments != string(payments.CapabilityRestricted) {
		t.Fatalf("card_payments = %q after replay, want the reset value to stand", cardPayments)
	}
}

// TestPostAccountWebhookHandler_LeavesTheAccountAloneWhenItShould proves
// every delivery the account endpoint refuses or acknowledges without
// acting on leaves the Practice's default state standing, and reaches
// Stripe not at all (bar the one lookup that finds no Practice owns the
// account): an unsigned or wrongly-signed thin event, a
// validly-signed live event on test keys, an event for an account no
// Practice claims (dropped with a 200 -- Stripe retries indefinitely on
// anything else), the account events Stripe also sends (created,
// [identity].updated and the rest), a body past the bound, and the
// FakeClient double that other packages' route tests go through, whose
// zero-value AccountEvent has an empty type that must be dropped rather
// than mistaken for a capability update.
func TestPostAccountWebhookHandler_LeavesTheAccountAloneWhenItShould(t *testing.T) {
	cases := []struct {
		name    string
		payload func(accountID string) []byte
		signed  string
		client  payments.Client
		want    int
		// fetches is true where the handler asks Stripe about the account
		// before it finds that no Practice claims it.
		fetches bool
	}{
		{
			name:    "a wrong signature",
			payload: func(id string) []byte { return capabilityStatusPayload(t, "evt_account_bad_sig", id) },
			signed:  wrongSecret, want: http.StatusBadRequest,
		},
		{
			name:    "a live event on test keys",
			payload: func(id string) []byte { return asLive(t, capabilityStatusPayload(t, "evt_account_live", id)) },
			signed:  stripeAccountWebhookSecret, want: http.StatusBadRequest,
		},
		{
			name: "an account no Practice claims",
			payload: func(string) []byte {
				return capabilityStatusPayload(t, "evt_account_unrecognized", "acct_unrecognized")
			},
			signed: stripeAccountWebhookSecret, want: http.StatusOK, fetches: true,
		},
		{
			name: "an event type it does not handle",
			payload: func(id string) []byte {
				return accountEventPayload(t, "evt_account_created", stripeEventTypeAccountCreated, id)
			},
			signed: stripeAccountWebhookSecret, want: http.StatusOK,
		},
		{
			name:    "a body past the bound",
			payload: func(string) []byte { return bytes.Repeat([]byte("a"), (1<<20)+1) },
			signed:  stripeAccountWebhookSecret, want: http.StatusBadRequest,
		},
		{
			name:    "the FakeClient double",
			payload: func(id string) []byte { return capabilityStatusPayload(t, "evt_fake_client", id) },
			signed:  stripeAccountWebhookSecret, client: payments.NewFakeClient(), want: http.StatusOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, connectAccountOnly())
			fetcher := activeAccountClient()
			client := tc.client
			if client == nil {
				client = fetcher
			}

			deliverAccountSignedWith(t, e.accountHook(client), tc.payload(e.AccountID), tc.signed, tc.want)

			if len(fetcher.fetched) != 0 && !tc.fetches {
				t.Fatalf("fetched = %v, want no Stripe call", fetcher.fetched)
			}
			cardPayments, _, _, _ := connectState(t, e.db, e.Practice.ID)
			if cardPayments != string(payments.CapabilityUnsupported) {
				t.Fatalf("card_payments = %q, want the default to stand", cardPayments)
			}
		})
	}
}

// TestPostAccountWebhookHandler_RetrieveFailureReturns500 proves a
// failure to reach Stripe is a 500 and claims nothing, so Stripe's retry
// is not swallowed later as a replay.
func TestPostAccountWebhookHandler_RetrieveFailureReturns500(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	client := activeAccountClient()
	client.err = errRetrieveFailed
	srv := e.accountHook(client)
	payload := capabilityStatusPayload(t, "evt_retrieve_fail", e.AccountID)

	deliverAccount(t, srv, payload, http.StatusInternalServerError)

	// The retry succeeds, proving the failed delivery claimed no event id.
	client.err = nil
	deliverAccount(t, srv, payload, http.StatusOK)
	cardPayments, _, _, _ := connectState(t, e.db, e.Practice.ID)
	if cardPayments != string(payments.CapabilityActive) {
		t.Fatalf("card_payments = %q after retry, want active", cardPayments)
	}
}

func TestPostConnectWebhookHandler_OversizedBodyRejected(t *testing.T) {
	db := testdb.New(t)
	srv := newConnectWebhookServer(db)
	defer srv.Close()

	oversized := bytes.Repeat([]byte("a"), (1<<20)+1)
	resp := reqtest.Post(t, srv.URL+"/stripe/connect-webhook", reqtest.Body("", oversized), reqtest.Header("Stripe-Signature", "t=1,v1=deadbeef"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// asLive marks a raw event payload as coming from the live account.
func asLive(t *testing.T, payload []byte) []byte {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	body["livemode"] = true
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return b
}

// invoiceMarkedUncollectiblePayload builds a raw
// invoice.marked_uncollectible event body referencing stripeInvoiceID.
func invoiceMarkedUncollectiblePayload(t *testing.T, eventID, accountID, stripeInvoiceID string) []byte {
	t.Helper()
	return buildConnectEventPayload(t, eventID, stripeEventTypeInvoiceMarkedUncollectible, accountID, map[string]any{
		"id":      stripeInvoiceID,
		objectKey: stripeObjectInvoice,
	})
}

// TestPostConnectWebhookHandler_RefusesWhatItCannotTrust proves a
// wrongly-signed snapshot event never reaches the database (it uses an
// invoice event because the Connect endpoint no longer carries account
// events at all -- Accounts v2 sends those as thin events to
// PostAccountWebhookHandler (#247)), and that an event whose data.object
// cannot be read into the expected shape is refused rather than applied
// with zero values.
func TestPostConnectWebhookHandler_RefusesWhatItCannotTrust(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T) []byte
		signed  string
		want    int
	}{
		{
			name: "a wrong signature",
			payload: func(t *testing.T) []byte {
				return invoicePaymentFailedPayload(t, "evt_bad_sig", "acct_bad_sig", "in_bad_sig")
			},
			signed: wrongSecret, want: http.StatusBadRequest,
		},
		{
			name: "an invoice.paid whose amount is not a number",
			payload: func(t *testing.T) []byte {
				return buildConnectEventPayload(t, "evt_invoice_paid_malformed", stripeEventTypeInvoicePaid, "acct_malformed",
					map[string]any{"id": "in_malformed", "amount_paid": notANumber})
			},
			signed: stripeConnectWebhookSecret, want: http.StatusInternalServerError,
		},
		{
			name: "an invoice.payment_failed whose id is a number",
			payload: func(t *testing.T) []byte {
				return buildConnectEventPayload(t, "evt_invoice_failed_malformed", stripeEventTypeInvoicePaymentFailed, "acct_malformed",
					map[string]any{"id": 12345})
			},
			signed: stripeConnectWebhookSecret, want: http.StatusInternalServerError,
		},
		{
			name: "an invoice.marked_uncollectible whose id is a number",
			payload: func(t *testing.T) []byte {
				return buildConnectEventPayload(t, "evt_marked_uncollectible_malformed", stripeEventTypeInvoiceMarkedUncollectible, "acct_malformed",
					map[string]any{"id": 12345})
			},
			signed: stripeConnectWebhookSecret, want: http.StatusInternalServerError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			deliverConnectSignedWith(t, e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test")), tc.payload(t), tc.signed, tc.want)
		})
	}
}

// TestPostConnectWebhookHandler_AcknowledgesWhatItCannotApply proves an
// event it has nothing to do for is dropped with a 200, since Stripe
// retries on anything but a 2xx: a genuinely unhandled snapshot event
// type, and an invoice event naming an Invoice DoulaCloud never raised.
func TestPostConnectWebhookHandler_AcknowledgesWhatItCannotApply(t *testing.T) {
	cases := []struct {
		name    string
		payload func(t *testing.T, accountID string) []byte
	}{
		{"an unhandled event type", func(t *testing.T, id string) []byte { return otherConnectEventPayload(t, "evt_other", id) }},
		{"an invoice.paid for an Invoice never raised", func(t *testing.T, id string) []byte {
			return invoicePaidPayload(t, "evt_unknown_invoice", id, "in_does_not_exist", time.Now())
		}},
		{"an invoice.payment_failed for an Invoice never raised", func(t *testing.T, id string) []byte {
			return invoicePaymentFailedPayload(t, "evt_unknown_invoice_failed", id, "in_does_not_exist_failed")
		}},
		{"an invoice.marked_uncollectible for an Invoice never raised", func(t *testing.T, id string) []byte {
			return invoiceMarkedUncollectiblePayload(t, "evt_marked_uncollectible_unknown", id, "in_does_not_exist")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, connectAccountOnly())
			deliverConnect(t, e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test")), tc.payload(t, e.AccountID))
		})
	}
}

// TestPostConnectWebhookHandler_LiveEventRefusedOnTestKeys proves a
// validly-signed live event is refused when the keys are in test mode.
func TestPostConnectWebhookHandler_LiveEventRefusedOnTestKeys(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_live_test", invoiceStatusOpen, 5000)
	srv := e.connectHook(newReferenceClient("pi_live_1"))

	payload := asLive(t, invoicePaidPayload(t, "evt_live_invoice", e.AccountID, "in_live_test", time.Now()))
	deliverConnectSignedWith(t, srv, payload, stripeConnectWebhookSecret, http.StatusBadRequest)

	if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidCreatesPaymentAndFlipsStatus
// proves a validly-signed invoice.paid event for a known Stripe invoice
// id creates exactly one payments row (amount paid, reference, paid-at)
// and flips the matching Invoice's status to paid with a matching
// paid_at.
func TestPostConnectWebhookHandler_InvoicePaidCreatesPaymentAndFlipsStatus(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_paid_test", invoiceStatusOpen, 5000)
	srv := e.connectHook(newReferenceClient("pi_test_1"))

	paidAt := time.Now().Truncate(time.Second).UTC()
	deliverConnect(t, srv, invoicePaidPayload(t, "evt_invoice_paid", e.AccountID, "in_paid_test", paidAt))

	status, gotPaidAt := invoiceStatusAndPaidAt(t, e.db, invoiceID)
	if status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
	if gotPaidAt == nil || !gotPaidAt.Equal(paidAt) {
		t.Fatalf("invoice paid_at = %v, want %v", gotPaidAt, paidAt)
	}

	rows := paymentsForInvoice(t, e.db, invoiceID)
	if len(rows) != 1 {
		t.Fatalf("payments for invoice = %d, want exactly 1", len(rows))
	}
	if rows[0].stripePaymentReference != "pi_test_1" || rows[0].amountCents != 5000 || !rows[0].paidAt.Equal(paidAt) {
		t.Fatalf("payment = %+v, want reference pi_test_1, amount 5000, paidAt %v", rows[0], paidAt)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidQueuesPaymentReceivedOutboxRow
// proves #344's trigger point: a validly-signed invoice.paid event queues
// exactly one payment_received_outbox row, in the same tx, linked via
// payment_id to the payments row it just created and carrying that
// Practice's id.
func TestPostConnectWebhookHandler_InvoicePaidQueuesPaymentReceivedOutboxRow(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_paid_outbox_test", invoiceStatusOpen, 5000)
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_invoice_paid_outbox", e.AccountID, "in_paid_outbox_test", time.Now().Truncate(time.Second).UTC()))

	if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 1 {
		t.Fatalf("payments for invoice = %d, want exactly 1", len(rows))
	}
	var count int
	var gotPracticeID string
	if err := e.db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*), max(pro.practice_id::text)
		 FROM payment_received_outbox pro
		 JOIN payments p ON p.id = pro.payment_id
		 WHERE p.invoice_id = $1`, invoiceID,
	).Scan(&count, &gotPracticeID); err != nil {
		t.Fatalf("query payment_received_outbox: %v", err)
	}
	if count != 1 {
		t.Fatalf("payment_received_outbox rows = %d, want exactly 1", count)
	}
	if gotPracticeID != e.Practice.ID {
		t.Fatalf("outbox row practice_id = %q, want %q", gotPracticeID, e.Practice.ID)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidReplayIsNoOp is the explicit
// idempotency test AC calls for: replaying the same invoice.paid event id
// must not create a second payments row.
func TestPostConnectWebhookHandler_InvoicePaidReplayIsNoOp(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_paid_replay", invoiceStatusOpen, 5000)
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))
	payload := invoicePaidPayload(t, "evt_invoice_paid_replay", e.AccountID, "in_paid_replay", time.Now())

	deliverConnect(t, srv, payload)
	deliverConnect(t, srv, payload)

	if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 1 {
		t.Fatalf("payments for invoice after replay = %d, want exactly 1", len(rows))
	}
	// A replayed event must not queue a second payment_received_outbox
	// row either -- this is the guarantee 00035's migration comment
	// leans on claimEvent's stripe_webhook_events claim for, since the
	// table itself carries no dedup index (unlike payout_outbox).
	outboxCount := testdb.Scalar[int](t, e.db.Admin, `SELECT count(*) FROM payment_received_outbox pro JOIN payments p ON p.id = pro.payment_id WHERE p.invoice_id = $1`, invoiceID)
	if outboxCount != 1 {
		t.Fatalf("payment_received_outbox rows after replay = %d, want exactly 1", outboxCount)
	}
}

// TestPostConnectWebhookHandler_InvoicePaidAlreadyPaidGuardSkipsSecondWrite
// proves #271's already-paid guard: a *distinct* invoice.paid event
// (never claimed before, so claimEvent's own replay dedup does not apply)
// arriving for an Invoice already 'paid' -- exactly the shape Stripe's
// own echo of PostManualPaymentHandler's paid_out_of_band call takes --
// is acknowledged but writes nothing. Unlike
// TestPostConnectWebhookHandler_InvoicePaidReplayIsNoOp, which replays
// the same event id, this proves the FOR UPDATE status check itself
// catches the case, independent of event-id dedup.
func TestPostConnectWebhookHandler_InvoicePaidAlreadyPaidGuardSkipsSecondWrite(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_already_paid_guard", invoiceStatusPaid, 5000)
	// Stands in for a manual Payment already recorded against this
	// Invoice (PostManualPaymentHandler's own INSERT, done directly here
	// rather than through the handler since this test's only point is the
	// webhook-side guard).
	if _, err := e.db.Admin.ExecContext(t.Context(),
		`INSERT INTO payments (invoice_id, stripe_payment_reference, amount_cents, paid_at, kind, method)
		 VALUES ($1, '', 5000, now(), 'manual', 'check')`,
		invoiceID,
	); err != nil {
		t.Fatalf("seed manual payment: %v", err)
	}
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_already_paid_guard", e.AccountID, "in_already_paid_guard", time.Now()))

	if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 1 {
		t.Fatalf("payments for invoice = %d, want exactly 1 (the manual row, untouched)", len(rows))
	}
}

// TestPostConnectWebhookHandler_InvoicePaidSurvivesNudgeEnqueueFailure is
// ADR-0013's correctness constraint proven at this write site: the
// handler's own tx already committed the payments row and the
// payment_received_outbox row by the time commitAndAck runs, so a Cloud
// Tasks enqueue failure afterward must never turn the acknowledgment
// into anything but 200 -- a non-2xx here would make Stripe redeliver an
// event whose payment already recorded.
func TestPostConnectWebhookHandler_InvoicePaidSurvivesNudgeEnqueueFailure(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_paid_nudge_fail", invoiceStatusOpen, 5000)
	failingEnq := &tasknudge.FakeEnqueuer{Err: errors.New("cloud tasks unavailable")}
	srv := newConnectWebhookServerWithEnqueuer(e.db, payments.NewStripeAPIClient("sk_test_unused", "https://app.test"), failingEnq)
	t.Cleanup(srv.Close)

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_invoice_paid_nudge_fail", e.AccountID, "in_paid_nudge_fail", time.Now()))

	if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 1 {
		t.Fatalf("payments for invoice = %d, want exactly 1 -- the write must have committed despite the enqueue failure", len(rows))
	}
}

// methodUSBankAccount is Stripe's payment method type for a US bank
// (ACH) debit.
const methodUSBankAccount = "us_bank_account"

// failedAttemptClient builds the referenceClient a failed-attempt test
// injects: real signature verification over a stubbed lookup of the
// method the attempt used.
func failedAttemptClient(method string, err error) *referenceClient {
	return &referenceClient{
		StripeAPIClient: payments.NewStripeAPIClient("sk_test_unused", "https://app.test"),
		method:          method,
		methodErr:       err,
	}
}

// activityRow is one activity row as a webhook test reads it back.
type activityRow struct {
	diff      map[string]any
	actorKind string
	clientID  sql.NullString
}

// activityRows reads every row of action recorded against engagementID,
// oldest first, through the superuser Admin connection.
func activityRows(t *testing.T, db *testdb.DB, action, engagementID string) []activityRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT diff, actor_kind::text, actor_client_id::text FROM activity WHERE action = $1 AND subject_id = $2 ORDER BY created_at`,
		action, engagementID)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []activityRow
	for rows.Next() {
		var raw []byte
		var row activityRow
		if err := rows.Scan(&raw, &row.actorKind, &row.clientID); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		row.diff = map[string]any{}
		if err := json.Unmarshal(raw, &row.diff); err != nil {
			t.Fatalf("unmarshal diff: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	return out
}

// TestPostConnectWebhookHandler_FailedAttemptLeavesInvoicePayable is
// #2224 and #2225. Stripe sends invoice.payment_failed on every failed
// attempt and keeps the Invoice open for another, so ours stays open too:
// a declined card in the portal and a bank debit that bounces days later
// both leave it payable, write no payments row, and record the attempt --
// hers, by the method she tried -- on the Engagement's ledger. A failure
// to ask Stripe which method was tried does not lose the record: the row
// is written with no method. And an attempt waiting on her (a redirect
// method like Klarna sends her away to approve it, before she has done
// anything) is not a failure: nothing is recorded.
func TestPostConnectWebhookHandler_FailedAttemptLeavesInvoicePayable(t *testing.T) {
	cases := []struct {
		name       string
		client     *referenceClient
		wantRows   int
		wantMethod string
	}{
		{"a declined card", failedAttemptClient("card", nil), 1, "card"},
		{"a failed bank debit", failedAttemptClient(methodUSBankAccount, nil), 1, methodUSBankAccount},
		{"a method Stripe could not be asked about", failedAttemptClient("", errRetrieveFailed), 1, ""},
		{"an attempt waiting on her", func() *referenceClient { c := failedAttemptClient("", nil); c.awaiting = true; return c }(), 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, connectAccountOnly())
			engagementID, invoiceID := e.seedStripeInvoice("in_failed_attempt", invoiceStatusOpen, 5000)
			srv := e.connectHook(tc.client)

			deliverConnect(t, srv, invoicePaymentFailedPayload(t, "evt_failed_attempt", e.AccountID, "in_failed_attempt"))

			if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusOpen {
				t.Fatalf("invoice status = %q, want %q -- Stripe keeps it open for another try", status, invoiceStatusOpen)
			}
			if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 0 {
				t.Fatalf("payments for invoice = %d, want 0", len(rows))
			}
			got := activityRows(t, e.db, "invoice_payment_failed", engagementID)
			if len(got) != tc.wantRows {
				t.Fatalf("invoice_payment_failed rows = %d, want %d", len(got), tc.wantRows)
			}
			if tc.wantRows == 0 {
				return
			}
			if clientID := clientOfEngagement(t, e.db, engagementID); got[0].actorKind != "client" || got[0].clientID.String != clientID {
				t.Fatalf("actor = %s/%v, want the Client %s -- the attempt was hers", got[0].actorKind, got[0].clientID, clientID)
			}
			method, has := got[0].diff["paymentMethodType"]
			if tc.wantMethod == "" && has {
				t.Fatalf("diff = %v, want no method when Stripe could not be asked", got[0].diff)
			}
			if tc.wantMethod != "" && method != tc.wantMethod {
				t.Fatalf("diff = %v, want paymentMethodType %q", got[0].diff, tc.wantMethod)
			}
		})
	}
}

// TestPostConnectWebhookHandler_InvoicePaymentFailedReplayIsNoOp proves
// replaying the same invoice.payment_failed event id twice records the
// attempt once.
func TestPostConnectWebhookHandler_InvoicePaymentFailedReplayIsNoOp(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	engagementID, invoiceID := e.seedStripeInvoice("in_failed_replay", invoiceStatusOpen, 5000)
	srv := e.connectHook(failedAttemptClient("card", nil))
	payload := invoicePaymentFailedPayload(t, "evt_invoice_failed_replay", e.AccountID, "in_failed_replay")

	deliverConnect(t, srv, payload)
	deliverConnect(t, srv, payload)

	if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusOpen)
	}
	if got := activityRows(t, e.db, "invoice_payment_failed", engagementID); len(got) != 1 {
		t.Fatalf("invoice_payment_failed rows = %d, want 1 after a replay", len(got))
	}
}

// TestPostConnectWebhookHandler_MarkedUncollectibleWritesOffAnOpenInvoice
// is the one way a Stripe-backed Invoice reaches uncollectible (#2224):
// Stripe says so, because the Practice wrote it off in her own Stripe
// Dashboard. Nobody in DoulaCloud did it, so the row's actor is
// DoulaCloud and its diff says where it came from.
func TestPostConnectWebhookHandler_MarkedUncollectibleWritesOffAnOpenInvoice(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	engagementID, invoiceID := e.seedStripeInvoice("in_marked_uncollectible", invoiceStatusOpen, 5000)
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))
	payload := invoiceMarkedUncollectiblePayload(t, "evt_marked_uncollectible", e.AccountID, "in_marked_uncollectible")

	deliverConnect(t, srv, payload)
	deliverConnect(t, srv, payload)

	if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusUncollectible {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusUncollectible)
	}
	got := activityRows(t, e.db, "invoice_written_off", engagementID)
	if len(got) != 1 {
		t.Fatalf("invoice_written_off rows = %d, want 1 after a replay", len(got))
	}
	if got[0].actorKind != "system" {
		t.Fatalf("actor_kind = %q, want system -- no Staff member of this system wrote it off", got[0].actorKind)
	}
	if got[0].diff["origin"] != "stripe_dashboard" {
		t.Fatalf("diff = %v, want origin stripe_dashboard", got[0].diff)
	}
}

// TestPostConnectWebhookHandler_MarkedUncollectibleLeavesASettledInvoiceAlone
// proves the event moves only an open Invoice. One already paid or
// voided here keeps its status and gains no row; the event is
// acknowledged, since Stripe retries anything else.
func TestPostConnectWebhookHandler_MarkedUncollectibleLeavesASettledInvoiceAlone(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	engagementID, invoiceID := e.seedStripeInvoice("in_marked_uncollectible_paid", invoiceStatusPaid, 5000)
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))

	deliverConnect(t, srv, invoiceMarkedUncollectiblePayload(t, "evt_marked_uncollectible_paid", e.AccountID, "in_marked_uncollectible_paid"))

	if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusPaid {
		t.Fatalf("invoice status = %q, want %q", status, invoiceStatusPaid)
	}
	if got := activityRows(t, e.db, "invoice_written_off", engagementID); len(got) != 0 {
		t.Fatalf("invoice_written_off rows = %d, want 0", len(got))
	}
}

// TestPostConnectWebhookHandler_InvoicePaidUnrecognizedAccountDroppedButAcknowledged
// proves an invoice.paid event whose account field matches no Practice is
// logged and dropped, even though the Stripe invoice id itself is known
// under a different account -- the Invoice's status stays untouched.
func TestPostConnectWebhookHandler_InvoicePaidUnrecognizedAccountDroppedButAcknowledged(t *testing.T) {
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_wrong_account", invoiceStatusOpen, 5000)
	srv := e.connectHook(payments.NewStripeAPIClient("sk_test_unused", "https://app.test"))

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_wrong_account", "acct_unrecognized_for_invoice", "in_wrong_account", time.Now()))

	if status, _ := invoiceStatusAndPaidAt(t, e.db, invoiceID); status != invoiceStatusOpen {
		t.Fatalf("invoice status = %q, want %q (untouched)", status, invoiceStatusOpen)
	}
	if rows := paymentsForInvoice(t, e.db, invoiceID); len(rows) != 0 {
		t.Fatalf("payments for invoice = %d, want 0", len(rows))
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
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_reference", invoiceStatusOpen, invoicePaidAmountCents)
	srv := e.connectHook(newReferenceClient("pi_from_stripe"))

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_reference", e.AccountID, "in_reference", time.Unix(1_700_000_000, 0)))

	reference := testdb.Scalar[string](t, e.db.Admin, `SELECT stripe_payment_reference FROM payments WHERE invoice_id = $1`, invoiceID)
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
	e := newEnv(t, connectAccountOnly())
	_, invoiceID := e.seedStripeInvoice("in_ref_fail", invoiceStatusOpen, invoicePaidAmountCents)
	client := newReferenceClient("")
	client.err = errRetrieveFailed
	srv := e.connectHook(client)

	deliverConnect(t, srv, invoicePaidPayload(t, "evt_ref_fail", e.AccountID, "in_ref_fail", time.Unix(1_700_000_000, 0)))

	var reference string
	var amount int64
	if err := e.db.Admin.QueryRowContext(t.Context(),
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
