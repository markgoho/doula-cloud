package payments_test

import (
	"database/sql"
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// invoiceStatusOpen is the invoices.status value shared by seed helpers
// and assertions across this package's test files -- pulled out once
// goconst's package-wide "open" repeat count crossed its threshold.
const invoiceStatusOpen = "open"

// The other four invoices.status values, named for the same goconst
// reason as invoiceStatusOpen above -- #271's new fixtures and
// assertions crossed the threshold for each of these too.
const (
	invoiceStatusDraft         = "draft"
	invoiceStatusPaid          = "paid"
	invoiceStatusVoid          = "void"
	invoiceStatusUncollectible = "uncollectible"
)

// seedContractWithStatus seeds a Contract row directly, bypassing the
// contracts package's own handlers, with an explicit status so tests can
// prove both GetInvoicesHandler's cross-status listing and
// PostInvoiceHandler's #275 status precondition.
func seedContractWithStatus(t *testing.T, db *testdb.DB, engagementID, status string) (contractID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, $2::contract_status, 'Test prose', 15000) RETURNING id`,
		engagementID, status,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	return contractID
}

// seedDraftContract is seedContractWithStatus at "draft", used only where
// a test's own point is something other than the Invoice-create
// precondition (the Connect-gate tests below, and every GetInvoicesHandler
// fixture, which lists across Contract status entirely). Stays local:
// contracts/template_test.go's own seedContract writes a caller-given
// status and prose for a template-rendering test, a different concern
// from this package's invoice-billing fixture.
func seedDraftContract(t *testing.T, db *testdb.DB, engagementID string) (contractID string) {
	t.Helper()
	return seedContractWithStatus(t, db, engagementID, "draft")
}

// seedSignedContract is seedContractWithStatus at "signed" -- the only
// status contracts.TransitionBill admits, per #275. Every
// PostInvoiceHandler test that exercises past the status precondition
// (the happy path and everything that used to seed a draft Contract as a
// convenience fixture before that precondition existed) uses this now.
func seedSignedContract(t *testing.T, db *testdb.DB, engagementID string) (contractID string) {
	t.Helper()
	return seedContractWithStatus(t, db, engagementID, "signed")
}

// seedConnectAccount sets practiceID's stored Stripe Connect account id
// and marks card_payments active, bypassing both PostConnectHandler and
// the webhook that ordinarily flips that column -- the shape every
// PostInvoiceHandler happy-path fixture wants: an account that can
// actually take a Client's payment, not merely one that exists (#270
// tightened the gate from "a row exists" to this).
func seedConnectAccount(t *testing.T, db *testdb.DB, practiceID, accountID string) {
	t.Helper()
	seedConnectAccountWithCardStatus(t, db, practiceID, accountID, "active")
}

// seedConnectAccountWithCardStatus is seedConnectAccount with an explicit
// card_payments status, for the tests proving PostInvoiceHandler's gate
// consults that column rather than only whether an account id is stored
// (e.g. an account left 'restricted' after an abandoned or reviewed-and-
// declined onboarding).
func seedConnectAccountWithCardStatus(t *testing.T, db *testdb.DB, practiceID, accountID, cardStatus string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1, stripe_connect_card_payments_status = $2 WHERE id = $3`,
		accountID, cardStatus, practiceID,
	); err != nil {
		t.Fatalf("seed connect account: %v", err)
	}
	// #271: every fixture that connects Stripe is, by construction, a
	// Stripe-billing Practice -- this stands in for the "ask once" write
	// resolveBillingMode would otherwise require on a Practice's first
	// PostInvoiceHandler call.
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
}

// seedInvoice inserts an invoices row directly at an explicit createdAt,
// so listing-order tests are deterministic rather than racing against
// now()'s resolution. Stays local: testdb has no invoice export, and no
// other package's tests need an invoice at a controlled timestamp --
// client's seedClientInvoice covers a different shape (no explicit
// createdAt).
func seedInvoice(t *testing.T, db *testdb.DB, practiceID, contractID, stripeInvoiceID, status string, amountCents int64, createdAt time.Time) (invoiceID string) {
	t.Helper()
	return seedInvoiceDue(t, db, practiceID, contractID, stripeInvoiceID, status, amountCents, createdAt,
		payments.DueAfterTerms(createdAt, payments.DefaultPaymentTermsDays))
}

// seedInvoiceDue is seedInvoice with the due date said out loud (#768) --
// what an overdue test needs, since being late is a fact about due_at
// against the comparison instant and nothing else. A due date in the past
// is how a test moves that comparison without waiting for one to arrive.
func seedInvoiceDue(t *testing.T, db *testdb.DB, practiceID, contractID, stripeInvoiceID, status string, amountCents int64, createdAt, dueAt time.Time) (invoiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, created_at, reference, due_at, paid_at)
		 VALUES ($1, $2, $3, $4::invoice_status, $5, 'usd', $6, $3, $7, CASE WHEN $4::invoice_status = 'paid' THEN $6::timestamptz END) RETURNING id`,
		practiceID, contractID, stripeInvoiceID, status, amountCents, createdAt, dueAt,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	return invoiceID
}

func invoiceCount(t *testing.T, db *testdb.DB) int {
	t.Helper()
	count := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM invoices`)
	return count
}

func invoiceStatus(t *testing.T, db *testdb.DB, invoiceID string) string {
	t.Helper()
	status := testdb.Scalar[string](t, db.Admin, `SELECT status FROM invoices WHERE id = $1`, invoiceID)
	return status
}

func invoicesPath(engagementID string) string {
	return "/engagements/" + engagementID + "/contract/invoices"
}

// janeClient is the Client whose name and email the Invoice tests assert
// reach Stripe.
var janeClient = fixture.NewClient(fixture.ClientName("Jane Client"), fixture.ClientEmail("jane@example.com"))

// contractWithStatus seeds an Engagement at the Practice with a Contract
// in status.
func (e *env) contractWithStatus(status string, opts ...fixture.EngagementOption) (engagementID, contractID string) {
	e.T.Helper()
	engagementID = fixture.Engagement(e.T, e.DB, e.Practice, opts...).ID
	return engagementID, seedContractWithStatus(e.T, e.DB, engagementID, status)
}

// raise raises an Invoice as the Staff member at the screen and returns
// the 201 view.
func (e *env) raise(engagementID string) payments.InvoiceView {
	e.T.Helper()
	return reqtest.Expect[payments.InvoiceView](e.T, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(payments.CreateInvoiceRequest{}))
}

// raiseWithMode is raise with the billing-mode answer (#271) on the request.
func (e *env) raiseWithMode(engagementID, billingMode string) payments.InvoiceView {
	e.T.Helper()
	return reqtest.Expect[payments.InvoiceView](e.T, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(payments.CreateInvoiceRequest{BillingMode: &billingMode}))
}

// refuseRaise raises an Invoice, requires the status want, and returns the
// error envelope it carries.
func (e *env) refuseRaise(engagementID string, want int, opts ...reqtest.Option) apierr.APIError {
	e.T.Helper()
	return e.postRefused(invoicesPath(engagementID), want, append([]reqtest.Option{reqtest.JSON(payments.CreateInvoiceRequest{})}, opts...)...)
}

func (e *env) listInvoices(engagementID string) payments.ListInvoicesResponse {
	e.T.Helper()
	return reqtest.Expect[payments.ListInvoicesResponse](e.T, http.MethodGet, e.PracticeURL(invoicesPath(engagementID)), http.StatusOK, e.SignedIn)
}

// requireNothingRaised proves a refusal reached neither Stripe nor the
// invoices table.
func (e *env) requireNothingRaised() {
	e.T.Helper()
	if got := len(e.Stripe.CreateInvoiceCalls); got != 0 {
		e.T.Fatalf("CreateInvoice calls = %d, want 0 -- nothing must reach Stripe", got)
	}
	if got := invoiceCount(e.T, e.DB); got != 0 {
		e.T.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_NotConnectedRefuses proves an Invoice attempted
// at a Practice with no Stripe Connect account at all 409s with
// MsgClientsCannotPay, whatever role the caller holds -- #270 collapsed
// the old 200 connectRequired/isOwner gate (which routed an Owner into
// the #79 connect flow and told a non-Owner to ask one) into a single
// role-blind refusal, because the frontend now decides that routing
// itself from EngagementDetail.ClientsCanPay before the form is ever
// shown.
func TestPostInvoiceHandler_NotConnectedRefuses(t *testing.T) {
	e := newEnv(t, withBillingMode(payments.BillingModeStripe))
	engagementID, _ := e.signedContract()

	out := e.refuseRaise(engagementID, http.StatusConflict)

	if out.Code != apierr.CodeFailedPrecondition {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if out.Message != payments.MsgClientsCannotPay {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgClientsCannotPay)
	}
	e.requireNothingRaised()
}

// TestPostInvoiceHandler_RestrictedCardPaymentsRefuses proves the gate bug
// #270 found: an Owner who opened the Stripe account, abandoned the
// hosted form halfway (or was reviewed and declined) leaves
// stripe_connect_account_id non-null with card_payments still
// 'restricted' -- fetchConnectAccount's old "an account id is stored"
// test would have let this Invoice through even though Stripe will not
// let anyone pay it. The tightened gate refuses it the same way an
// unconnected Practice is refused.
func TestPostInvoiceHandler_RestrictedCardPaymentsRefuses(t *testing.T) {
	e := newEnv(t, connectedWithCardStatus("restricted"))
	engagementID, _ := e.signedContract()

	e.refuseRaise(engagementID, http.StatusConflict)

	e.requireNothingRaised()
}

// TestPostInvoiceHandler_CardPaymentsActiveWithNoAccountReturns500 proves
// fetchConnectAccountID's own guard: nothing in the schema ties
// card_payments_status to stripe_connect_account_id, so a row that
// somehow reaches 'active' with no account id linked -- unreachable
// through the webhook, which only ever writes that column by matching an
// existing account id, but not through a bare UPDATE -- 500s rather than
// calling Stripe with an empty account id.
func TestPostInvoiceHandler_CardPaymentsActiveWithNoAccountReturns500(t *testing.T) {
	e := newEnv(t, withBillingMode(payments.BillingModeStripe))
	testdb.SeedClientsCanPay(t, e.DB, e.Practice.ID)
	engagementID, _ := e.signedContract()

	e.refuseRaise(engagementID, http.StatusInternalServerError)

	if got := len(e.Stripe.CreateInvoiceCalls); got != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- must never reach Stripe with an empty account id", got)
	}
}

// TestPostInvoiceHandler_CreatesInvoiceWhenConnected proves the full
// creation path once a Practice is connected: the fake Stripe port
// receives the connected account id, the Client's name/email, the fixed
// InvoiceLineItemDescription, and the Staff-supplied amount -- and nothing
// else -- and the persisted row lands in 'open' status (Draft then
// finalized).
func TestPostInvoiceHandler_CreatesInvoiceWhenConnected(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected()) // any Staff with practice access, no owner gating
	engagementID, contractID := e.signedContract(janeClient)

	out := e.raise(engagementID)

	if out.ContractID != contractID {
		t.Fatalf("invoice.contractId = %q, want %q", out.ContractID, contractID)
	}
	if out.Status != invoiceStatusOpen {
		t.Fatalf("invoice.status = %q, want %q", out.Status, invoiceStatusOpen)
	}
	if out.AmountCents != 15000 {
		t.Fatalf("invoice.amountCents = %d, want 15000", out.AmountCents)
	}
	if out.Currency != "usd" {
		t.Fatalf("invoice.currency = %q, want %q", out.Currency, "usd")
	}
	if out.PaidOn != nil {
		t.Fatalf("invoice.paidOn = %v, want nil", *out.PaidOn)
	}

	client := e.Stripe
	if len(client.CreateInvoiceCalls) != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1", len(client.CreateInvoiceCalls))
	}
	call := client.CreateInvoiceCalls[0]
	if call.AccountID != e.AccountID {
		t.Fatalf("CreateInvoice accountID = %q, want %q", call.AccountID, e.AccountID)
	}
	// The Customer is resolved before the Invoice is raised (#780), and
	// her name and email -- and nothing else, no clinical field -- are all
	// that reach Stripe to make it.
	if len(client.CreateCustomerCalls) != 1 {
		t.Fatalf("CreateCustomer calls = %d, want 1", len(client.CreateCustomerCalls))
	}
	cust := client.CreateCustomerCalls[0]
	if cust.AccountID != e.AccountID {
		t.Fatalf("CreateCustomer accountID = %q, want %q", cust.AccountID, e.AccountID)
	}
	if cust.CustomerEmail != "jane@example.com" || cust.CustomerName != "Jane Client" {
		t.Fatalf("CreateCustomer customer = (%q, %q), want (%q, %q)", cust.CustomerName, cust.CustomerEmail, "Jane Client", "jane@example.com")
	}
	if call.CustomerID == "" {
		t.Fatal("CreateInvoice customerID is empty, want the resolved Customer")
	}
	if call.Description != payments.InvoiceLineItemDescription {
		t.Fatalf("CreateInvoice description = %q, want %q", call.Description, payments.InvoiceLineItemDescription)
	}
	if call.AmountCents != 15000 {
		t.Fatalf("CreateInvoice amountCents = %d, want 15000", call.AmountCents)
	}
	if len(client.FinalizeInvoiceIDs) != 1 {
		t.Fatalf("FinalizeInvoice calls = %d, want 1", len(client.FinalizeInvoiceIDs))
	}

	if got := invoiceStatus(t, e.DB, out.ID); got != invoiceStatusOpen {
		t.Fatalf("persisted invoice status = %q, want %q", got, invoiceStatusOpen)
	}
}

// TestPostInvoiceHandler_UnbillableContractRefused is #275's core
// regression: a Draft, a Sent (unsigned) or a Voided Contract each refuse
// an Invoice with a conflict, before anything reaches Stripe or the
// database -- proving the bug ("a voided Contract can still be invoiced,
// and Stripe really bills it") is fixed for every non-signed state, not
// only the voided one the ticket names.
func TestPostInvoiceHandler_UnbillableContractRefused(t *testing.T) {
	for _, status := range []string{"draft", "sent", "voided"} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Doula()), connected())
			engagementID, _ := e.contractWithStatus(status)

			out := e.refuseRaise(engagementID, http.StatusConflict)

			if !strings.Contains(out.Message, status) {
				t.Errorf("refusal message = %q, want it to name the actual status %q", out.Message, status)
			}
			if len(e.Stripe.CreateCustomerCalls) != 0 {
				t.Fatalf("CreateCustomer calls = %d, want 0 -- nothing must reach Stripe", len(e.Stripe.CreateCustomerCalls))
			}
			e.requireNothingRaised()
		})
	}
}

// TestPostInvoiceHandler_ClientWithNoEmailRefuses proves ADR-0017's
// ride-along: invoicing a Client with no email on file refuses rather
// than sending an empty string to Stripe.
func TestPostInvoiceHandler_ClientWithNoEmailRefuses(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract(fixture.NewClient(fixture.NoEmail()))

	out := e.refuseRaise(engagementID, http.StatusUnprocessableEntity)

	if out.Message != payments.MsgClientNoEmail {
		t.Errorf("message = %q, want %q", out.Message, payments.MsgClientNoEmail)
	}
	if len(e.Stripe.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- must never send an empty string to Stripe", len(e.Stripe.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_NoContractReturns404 proves an Engagement with no
// Contract yet 404s rather than creating an Invoice against nothing.
func TestPostInvoiceHandler_NoContractReturns404(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID := fixture.Engagement(t, e.DB, e.Practice).ID

	e.refuseRaise(engagementID, http.StatusNotFound)

	if len(e.Stripe.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(e.Stripe.CreateInvoiceCalls))
	}
}

// TestInvoiceHandlers_RefuseABadEngagementID proves a syntactically
// invalid :engagementId is rejected before any DB lookup, and a
// well-formed but unknown one 404s, on both the raise and the list.
func TestInvoiceHandlers_RefuseABadEngagementID(t *testing.T) {
	cases := []struct {
		name         string
		method       string
		engagementID string
		want         int
	}{
		{"raising on an id that is not a uuid", http.MethodPost, notAUUID, http.StatusBadRequest},
		{"raising on an unknown Engagement", http.MethodPost, unknownID, http.StatusNotFound},
		{"listing on an id that is not a uuid", http.MethodGet, notAUUID, http.StatusBadRequest},
		{"listing on an unknown Engagement", http.MethodGet, unknownID, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.expect(tc.method, invoicesPath(tc.engagementID), tc.want)
		})
	}
}

// TestPostInvoiceHandler_ContractorWithoutAttachmentNotFound proves
// #947's reach test: raising an Invoice stays open to any Staff by role
// (#68), but a contractor Doula who holds no granted attachment on the
// Engagement is refused with a 404 before the handler runs at all --
// AttachingWrite's own refusal (staffauth/attach.go), the same one every
// other Engagement-scoped write already carries. Proven by zero Invoices
// persisted and zero Stripe calls, not just the status code, so a bug
// that let the handler run and fail some other way wouldn't pass this
// test by accident.
func TestPostInvoiceHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	engagementID, _ := e.signedContract()

	e.refuseRaise(engagementID, http.StatusNotFound)

	e.requireNothingRaised()
}

// TestPostInvoiceHandler_ContractorWithGrantedAttachmentSucceeds proves
// the other half of #947's reach test: a contractor Doula who *does*
// hold a granted attachment on the Engagement raises an Invoice the same
// as anyone else -- reach, not role, is what #68 and this route's own
// AttachingWrite gate on.
func TestPostInvoiceHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	engagementID, _ := e.signedContract(fixture.WithDoula(e.Staff))
	byHand := string(payments.BillingModeByHand)

	body := reqtest.ExpectBody(t, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(payments.CreateInvoiceRequest{BillingMode: &byHand}))

	if got := invoiceCount(t, e.DB); got != 1 {
		t.Fatalf("invoices row count = %d, want 1", got)
	}

	// #2002: she may raise the Invoice (ADR-0008's write table), but what
	// comes back is not money she may read (its read table: "her own
	// agreed fee only -- never the Practice's price"). The 201 says the
	// Invoice exists and which rail it went on, and names no amount.
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"amountCents", "currency", "refundedCents", "activePaymentId"} {
		if _, present := fields[key]; present {
			t.Fatalf("a contractor's 201 carries %q; ADR-0008 keeps Invoice money from her", key)
		}
	}
	var raised payments.RaisedInvoice
	if err := json.Unmarshal(body, &raised); err != nil {
		t.Fatalf("decode raised invoice: %v", err)
	}
	if raised.ID == "" || raised.BillingMode != byHand {
		t.Fatalf("raised = %+v, want an id and billingMode %q", raised, byHand)
	}
}

// TestPostInvoiceHandler_AmountDerivesFromContractIgnoringBody proves
// #947's core AC: the amount an Invoice is raised for is the signed
// Contract's own amount_cents, never a caller-supplied figure, even when
// a request body tries to name one -- CreateInvoiceRequest carries no
// such field any more, so an old caller's amountCents is simply ignored
// as an unknown key. The seeded Contract carries 15000
// (seedSignedContract -> seedContractWithStatus); the attempted 999 must
// never reach the created Invoice, Stripe's own CreateInvoice call, or
// the invoice_raised activity entry.
func TestPostInvoiceHandler_AmountDerivesFromContractIgnoringBody(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	out := reqtest.Expect[payments.InvoiceView](t, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.RawJSON([]byte(`{"amountCents": 999}`)))

	if out.AmountCents != 15000 {
		t.Fatalf("invoice.amountCents = %d, want 15000 (the Contract's, not the attempted 999)", out.AmountCents)
	}
	if calls := e.Stripe.CreateInvoiceCalls; len(calls) != 1 || calls[0].AmountCents != 15000 {
		t.Fatalf("CreateInvoice calls = %v, want one call at 15000", calls)
	}

	diff := testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT diff FROM activity WHERE subject_id = $1 AND action = 'invoice_raised'`, engagementID)
	var decoded struct {
		AmountCents int64 `json:"amountCents"`
	}
	if err := json.Unmarshal(diff, &decoded); err != nil {
		t.Fatalf("decode activity diff: %v", err)
	}
	if decoded.AmountCents != 15000 {
		t.Fatalf("activity diff amountCents = %d, want 15000", decoded.AmountCents)
	}
}

// TestPostInvoiceHandler_InvalidBodyReturns400 proves malformed JSON is
// rejected.
func TestPostInvoiceHandler_InvalidBodyReturns400(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), connected())
	engagementID, _ := e.signedContract()

	e.refuseRaise(engagementID, http.StatusBadRequest, reqtest.RawJSON([]byte(`not-json`)))
}

// TestPostInvoiceHandler_StripeFailureLeavesTheRightRows proves what a
// Stripe failure leaves behind. A failure creating the draft surfaces as
// an internal error and never persists a row (the port failed before any
// insert happened). A failure finalizing it, after the draft was created
// on Stripe and inserted locally, still leaves the row committed --
// DoulaCloud never loses track of an Invoice that exists on Stripe -- but
// voided (TransitionAbandonDraft) and recorded as invoice_not_sent by
// DoulaCloud, so a draft nothing else would ever move cannot sit in the
// way of the Client's erasure or the Practice's deletion.
func TestPostInvoiceHandler_StripeFailureLeavesTheRightRows(t *testing.T) {
	cases := []struct {
		name       string
		fail       func(*payments.FakeClient)
		wantRows   int
		wantStatus string
	}{
		{"creating the draft", func(c *payments.FakeClient) { c.CreateInvoiceErr = errStripeFake }, 0, ""},
		{"finalizing the draft", func(c *payments.FakeClient) { c.FinalizeInvoiceErr = errStripeFake }, 1, invoiceStatusVoid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stripe := payments.NewFakeClient()
			tc.fail(stripe)
			e := newEnv(t, signedInAs(fixture.Doula()), withStripe(stripe), connected())
			engagementID, _ := e.signedContract()

			e.refuseRaise(engagementID, http.StatusInternalServerError)

			if got := invoiceCount(t, e.DB); got != tc.wantRows {
				t.Fatalf("invoices row count = %d, want %d", got, tc.wantRows)
			}
			if tc.wantRows == 0 {
				return
			}
			if status := testdb.Scalar[string](t, e.DB.Admin, `SELECT status FROM invoices LIMIT 1`); status != tc.wantStatus {
				t.Fatalf("persisted invoice status = %q, want %q", status, tc.wantStatus)
			}
			notSent := activityRows(t, e.DB, "invoice_not_sent", engagementID)
			if len(notSent) != 1 || notSent[0].actorKind != "system" {
				t.Fatalf("invoice_not_sent rows = %+v, want one by DoulaCloud", notSent)
			}
		})
	}
}

// TestGetInvoicesHandler_ListsAcrossVoidedContract proves an Invoice
// billed against a since-voided Contract still lists under its
// Engagement, newest first, alongside an Invoice against the Contract
// that replaced it -- listing is scoped to the Engagement, not just "the
// current Contract row".
func TestGetInvoicesHandler_ListsAcrossVoidedContract(t *testing.T) {
	e := newEnv(t)
	engagementID, voidedContractID := e.contractWithStatus("voided")
	currentContractID := seedDraftContract(t, e.DB, engagementID)

	base := time.Now().Add(-time.Hour)
	oldInvoiceID := seedInvoice(t, e.DB, e.Practice.ID, voidedContractID, "in_old", invoiceStatusPaid, 10000, base)
	newInvoiceID := seedInvoice(t, e.DB, e.Practice.ID, currentContractID, "in_new", invoiceStatusOpen, 20000, base.Add(time.Minute))

	out := e.listInvoices(engagementID)

	if out.HasMore {
		t.Fatal("hasMore = true, want false")
	}
	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(out.Items))
	}
	if out.Items[0].ID != newInvoiceID || out.Items[1].ID != oldInvoiceID {
		t.Fatalf("items = [%q, %q], want [%q, %q] (newest first)", out.Items[0].ID, out.Items[1].ID, newInvoiceID, oldInvoiceID)
	}
	if out.Items[1].ContractID != voidedContractID {
		t.Fatalf("items[1].contractId = %q, want %q (the voided contract)", out.Items[1].ContractID, voidedContractID)
	}
}

// TestGetInvoicesHandler_ByHandPaidOnIsTheTypedDay proves a by-hand
// Payment's day reaches the response as the day the recorder typed
// (#1965). manual_payment.go stores Oct 25 as 2026-10-25T00:00Z, which is
// the evening of Oct 24 in the Practice's New York; read as an instant it
// would be a day early.
func TestGetInvoicesHandler_ByHandPaidOnIsTheTypedDay(t *testing.T) {
	e := newEnv(t)
	engagementID, contractID := e.contractWithStatus("draft")
	invoiceID := seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_paid", invoiceStatusPaid, 10000, time.Now())
	seedManualPayment(t, e.DB, invoiceID, methodCheck, "")
	if _, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = '2026-10-25T00:00:00Z' WHERE id = $1`, invoiceID); err != nil {
		t.Fatalf("seed paid_at: %v", err)
	}

	out := e.listInvoices(engagementID)

	if len(out.Items) != 1 || out.Items[0].PaidOn == nil {
		t.Fatalf("items = %+v, want one item with paidOn set", out.Items)
	}
	if *out.Items[0].PaidOn != dayOct25 {
		t.Fatalf("paidOn = %q, want 2026-10-25", *out.Items[0].PaidOn)
	}
}

// TestGetInvoicesHandler_EmptyBeforeAnyContract proves an Engagement with
// no Contract (and therefore no Invoices) yet returns an empty list, not
// a 404 -- listing tolerates "nothing yet" the way creation doesn't. An
// employed Doula reaches the per-Engagement Invoice history the same as
// an Owner or Admin (ADR-0008's money row as amended by #282).
func TestGetInvoicesHandler_EmptyBeforeAnyContract(t *testing.T) {
	cases := []struct {
		name string
		who  fixture.StaffOption
	}{
		{"an Owner", fixture.Owner()},
		{caseDoula, fixture.Doula()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.who))
			engagementID := fixture.Engagement(t, e.DB, e.Practice).ID

			if out := e.listInvoices(engagementID); len(out.Items) != 0 {
				t.Fatalf("items = %d, want 0", len(out.Items))
			}
		})
	}
}

// TestGetInvoicesHandler_PaginatesWithCursor proves a page beyond
// invoicePageSize (30) sets hasMore/nextCursor, and that cursor correctly
// resumes on the next page.
func TestGetInvoicesHandler_PaginatesWithCursor(t *testing.T) {
	e := newEnv(t)
	engagementID, contractID := e.contractWithStatus("draft")

	const total = 31
	base := time.Now().Add(-time.Hour)
	ids := make([]string, total)
	for i := range total {
		ids[i] = seedInvoice(t, e.DB, e.Practice.ID, contractID, "in_page_"+strconv.Itoa(i), invoiceStatusOpen, int64(1000+i), base.Add(time.Duration(i)*time.Second))
	}

	firstPage := e.listInvoices(engagementID)
	if !firstPage.HasMore {
		t.Fatal("hasMore = false on first page, want true")
	}
	if firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		t.Fatal("nextCursor is empty on first page, want a cursor")
	}
	if len(firstPage.Items) != 30 {
		t.Fatalf("first page items = %d, want 30", len(firstPage.Items))
	}
	// Newest first: the 31st seeded invoice (index total-1) is newest.
	if firstPage.Items[0].ID != ids[total-1] {
		t.Fatalf("first page items[0] = %q, want %q (newest)", firstPage.Items[0].ID, ids[total-1])
	}

	secondPage := reqtest.Expect[payments.ListInvoicesResponse](t, http.MethodGet,
		e.PracticeURL(invoicesPath(engagementID)+"?cursor="+*firstPage.NextCursor), http.StatusOK, e.SignedIn)
	if secondPage.HasMore {
		t.Fatal("hasMore = true on second page, want false")
	}
	if len(secondPage.Items) != 1 {
		t.Fatalf("second page items = %d, want 1", len(secondPage.Items))
	}
	if secondPage.Items[0].ID != ids[0] {
		t.Fatalf("second page items[0] = %q, want %q (oldest)", secondPage.Items[0].ID, ids[0])
	}
}

// invalidCursors are the ways a caller-supplied cursor can fail to decode.
var invalidCursors = map[string]string{
	// '!' is outside the URL-safe base64 alphabet and has no special
	// meaning in a query string (unlike '%'), so this fails to decode
	// without confusing percent-encoding.
	"not valid base64":            "!!!not-valid-base64!!!",
	"valid base64, no separator":  base64.URLEncoding.EncodeToString([]byte("nosep")),
	"valid base64, bad timestamp": base64.URLEncoding.EncodeToString([]byte("not-a-time|some-id")),
}

// TestGetInvoicesHandler_InvalidCursorReturns400 proves every way a
// caller-supplied cursor can fail to decode is rejected with 400, rather
// than a panic or a silently wrong page.
func TestGetInvoicesHandler_InvalidCursorReturns400(t *testing.T) {
	e := newEnv(t)
	engagementID := fixture.Engagement(t, e.DB, e.Practice).ID

	for name, cursor := range invalidCursors {
		t.Run(name, func(*testing.T) {
			e.expect(http.MethodGet, invoicesPath(engagementID)+"?cursor="+cursor, http.StatusBadRequest)
		})
	}
}

// billingModeOfPractice reads practiceID's raw billing_mode column.
func billingModeOfPractice(t *testing.T, db *testdb.DB, practiceID string) (mode string, ok bool) {
	t.Helper()
	raw := testdb.Scalar[sql.NullString](t, db.Admin, `SELECT billing_mode FROM practices WHERE id = $1`, practiceID)
	return raw.String, raw.Valid
}

// TestPostInvoiceHandler_BillingModeRequiredWhenUnset proves #271's
// refusal when a Practice has never chosen a billing mode and the request
// raising its first Invoice supplies none either.
func TestPostInvoiceHandler_BillingModeRequiredWhenUnset(t *testing.T) {
	e := newEnv(t)
	engagementID, _ := e.signedContract()

	out := e.refuseRaise(engagementID, http.StatusUnprocessableEntity)

	if out.Message != payments.MsgBillingModeRequired {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgBillingModeRequired)
	}
	// #1775: the refusal names the field, so the app shows it against
	// the billing-mode question as well as in its error summary.
	if got := out.Details["billingMode"]; got != payments.DetailBillingModeRequired {
		t.Fatalf("details.billingMode = %q, want %q", got, payments.DetailBillingModeRequired)
	}
	if got := invoiceCount(t, e.DB); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_RefusedFirstInvoiceDoesNotLockInBillingMode
// proves a Practice's first-ever billing_mode choice is not persisted
// when the Invoice it was named on is refused -- staffauth.Middleware
// commits this package's tx even on a non-2xx response, so a naive
// "persist as soon as chosen" would durably lock in a mode nobody's
// Invoice ever actually used. #1590's case is the 409 for a Practice with
// no Stripe account: the next visit asks the question again.
func TestPostInvoiceHandler_RefusedFirstInvoiceDoesNotLockInBillingMode(t *testing.T) {
	cases := []struct {
		name       string
		opts       []envOption
		client     []fixture.ClientOption
		mode       string
		wantStatus int
	}{
		{"a value that is not a billing mode", nil, nil, "cash_app", http.StatusBadRequest},
		{
			"a Client with no email, on a connected Practice",
			[]envOption{signedInAs(fixture.Doula()), connectedBillingUnset()},
			[]fixture.ClientOption{fixture.NoEmail()},
			string(payments.BillingModeStripe), http.StatusUnprocessableEntity,
		},
		{"a Practice with no Stripe account", []envOption{signedInAs(fixture.Doula())}, nil, string(payments.BillingModeStripe), http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, tc.opts...)
			engagementID, _ := e.signedContract(fixture.NewClient(tc.client...))

			e.refuseRaise(engagementID, tc.wantStatus, reqtest.JSON(payments.CreateInvoiceRequest{BillingMode: &tc.mode}))

			if got := invoiceCount(t, e.DB); got != 0 {
				t.Fatalf("invoices row count = %d, want 0", got)
			}
			if mode, ok := billingModeOfPractice(t, e.DB, e.Practice.ID); ok {
				t.Fatalf("billing_mode = %q, want it left unset after a refused first Invoice", mode)
			}
		})
	}
}

// TestPostInvoiceHandler_ByHandInvoiceCreatedOpenNoStripeCall proves
// #271's by-hand rail: raised 'open' immediately, no Stripe call at all,
// a sequence-derived reference, and the Practice's billing_mode is
// persisted from the request's first-ever value.
func TestPostInvoiceHandler_ByHandInvoiceCreatedOpenNoStripeCall(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, contractID := e.signedContract()

	out := e.raiseWithMode(engagementID, string(payments.BillingModeByHand))

	if out.ContractID != contractID {
		t.Fatalf("invoice.contractId = %q, want %q", out.ContractID, contractID)
	}
	if out.Status != invoiceStatusOpen {
		t.Fatalf("invoice.status = %q, want %q", out.Status, invoiceStatusOpen)
	}
	if out.BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("invoice.billingMode = %q, want %q", out.BillingMode, payments.BillingModeByHand)
	}
	if out.Reference != "INV-0001" {
		t.Fatalf("invoice.reference = %q, want %q", out.Reference, "INV-0001")
	}
	if len(e.Stripe.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- a by-hand Invoice must never reach Stripe", len(e.Stripe.CreateInvoiceCalls))
	}
	if len(e.Stripe.FinalizeInvoiceIDs) != 0 {
		t.Fatalf("FinalizeInvoice calls = %d, want 0", len(e.Stripe.FinalizeInvoiceIDs))
	}
	mode, ok := billingModeOfPractice(t, e.DB, e.Practice.ID)
	if !ok || mode != string(payments.BillingModeByHand) {
		t.Fatalf("billing_mode = (%q, ok=%v), want (%q, true)", mode, ok, payments.BillingModeByHand)
	}
}

// TestPostInvoiceHandler_ByHandInvoiceSequenceIncrementsAcrossInvoices
// proves the per-Practice reference sequence claims a distinct number for
// each by-hand Invoice, in order.
func TestPostInvoiceHandler_ByHandInvoiceSequenceIncrementsAcrossInvoices(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, _ := e.signedContract()

	first := e.raiseWithMode(engagementID, string(payments.BillingModeByHand))
	// The mode is already set now, so the second request need not (and,
	// per #271, must not have its own value honored) repeat billingMode.
	second := e.raise(engagementID)

	if first.Reference != "INV-0001" {
		t.Fatalf("first reference = %q, want %q", first.Reference, "INV-0001")
	}
	if second.Reference != "INV-0002" {
		t.Fatalf("second reference = %q, want %q", second.Reference, "INV-0002")
	}
}

// TestPostInvoiceHandler_ByHandInvoiceIgnoresLaterBillingModeRequest
// proves #271's "changing the mode is the Owner's alone, afterward" rule
// from the create side: once a Practice's billing_mode is set, a later
// Invoice-raise request naming a different mode is silently ignored
// (the established mode governs), never silently switched.
func TestPostInvoiceHandler_ByHandInvoiceIgnoresLaterBillingModeRequest(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, _ := e.signedContract()
	e.raiseWithMode(engagementID, string(payments.BillingModeByHand))

	// This request names "stripe", but the Practice already chose
	// by_hand -- it must be ignored, and this Invoice raised by hand too.
	out := e.raiseWithMode(engagementID, string(payments.BillingModeStripe))

	if out.BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("invoice.billingMode = %q, want %q (the already-established mode)", out.BillingMode, payments.BillingModeByHand)
	}
	if len(e.Stripe.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- billingMode=stripe must be ignored once a mode is set", len(e.Stripe.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_ByHandInvoiceClientWithNoEmailStillSucceeds
// amends #430's refusal (errClientNoEmail): a by-hand Invoice mails
// nothing, so an absent Client email must not block it.
func TestPostInvoiceHandler_ByHandInvoiceClientWithNoEmailStillSucceeds(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, _ := e.signedContract(fixture.NewClient(fixture.NoEmail()))

	e.raiseWithMode(engagementID, string(payments.BillingModeByHand))
}

// TestPostInvoiceHandler_StripeInvoiceUsesStripeNumberAsReference proves
// #271's Stripe-rail reference: once FinalizeInvoice succeeds, the
// persisted Invoice's reference is Stripe's own `number`, not its
// internal invoice id.
func TestPostInvoiceHandler_StripeInvoiceUsesStripeNumberAsReference(t *testing.T) {
	e := newEnv(t, connected())
	engagementID, _ := e.signedContract()

	out := e.raise(engagementID)

	if out.BillingMode != string(payments.BillingModeStripe) {
		t.Fatalf("invoice.billingMode = %q, want %q", out.BillingMode, payments.BillingModeStripe)
	}
	if !strings.HasPrefix(out.Reference, "STRIPE-") {
		t.Fatalf("invoice.reference = %q, want the fake client's deterministic STRIPE-<id> fallback", out.Reference)
	}
}

// TestGetInvoicesHandler_ByHandInvoiceReportsByHandBillingMode proves a
// listed by-hand Invoice (NULL stripe_invoice_id) reports
// billingMode="by_hand" -- billingModeOf's NULL branch.
func TestGetInvoicesHandler_ByHandInvoiceReportsByHandBillingMode(t *testing.T) {
	e := newEnv(t)
	engagementID, contractID := e.signedContract()
	seedByHandInvoice(t, e.DB, e.Practice.ID, contractID)

	out := e.listInvoices(engagementID)

	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	if out.Items[0].BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("billingMode = %q, want %q", out.Items[0].BillingMode, payments.BillingModeByHand)
	}
}
