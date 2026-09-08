package payments_test

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
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
		`INSERT INTO contracts (engagement_id, status, prose) VALUES ($1, $2::contract_status, 'Test prose') RETURNING id`,
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
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, created_at, reference)
		 VALUES ($1, $2, $3, $4::invoice_status, $5, 'usd', $6, $3) RETURNING id`,
		practiceID, contractID, stripeInvoiceID, status, amountCents, createdAt,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	return invoiceID
}

func invoiceCount(t *testing.T, db *testdb.DB) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM invoices`).Scan(&count); err != nil {
		t.Fatalf("count invoices: %v", err)
	}
	return count
}

func invoiceStatus(t *testing.T, db *testdb.DB, invoiceID string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM invoices WHERE id = $1`, invoiceID).Scan(&status); err != nil {
		t.Fatalf("query invoice status: %v", err)
	}
	return status
}

func newInvoiceServer(t *testing.T, db *testdb.DB, uid string, client payments.Client) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	payments.Mount(g, ir, client)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postInvoiceBody(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/contract/invoices", bytes.NewBufferString(body))
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

func postInvoice(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string, amountCents int64) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.CreateInvoiceRequest{AmountCents: amountCents})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return postInvoiceBody(t, srv, session, practiceID, engagementID, string(body))
}

func getInvoices(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID, cursor string) *http.Response {
	t.Helper()
	url := srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/contract/invoices"
	if cursor != "" {
		url += "?cursor=" + cursor
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
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
	db := testdb.New(t)
	const uid = "invoice-gate-not-connected"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != string(apierr.CodeFailedPrecondition) {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if out.Message != payments.MsgClientsCannotPay {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgClientsCannotPay)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
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
	db := testdb.New(t)
	const uid = "invoice-gate-restricted"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccountWithCardStatus(t, db, practiceID, accountID, "restricted")

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_CardPaymentsActiveWithNoAccountReturns500 proves
// fetchConnectAccountID's own guard: nothing in the schema ties
// card_payments_status to stripe_connect_account_id, so a row that
// somehow reaches 'active' with no account id linked -- unreachable
// through the webhook, which only ever writes that column by matching an
// existing account id, but not through a bare UPDATE -- 500s rather than
// calling Stripe with an empty account id.
func TestPostInvoiceHandler_CardPaymentsActiveWithNoAccountReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-active-no-account"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	testdb.SeedClientsCanPay(t, db, practiceID)
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- must never reach Stripe with an empty account id", len(client.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_CreatesInvoiceWhenConnected proves the full
// creation path once a Practice is connected: the fake Stripe port
// receives the connected account id, the Client's name/email, the fixed
// InvoiceLineItemDescription, and the Staff-supplied amount -- and nothing
// else -- and the persisted row lands in 'open' status (Draft then
// finalized).
func TestPostInvoiceHandler_CreatesInvoiceWhenConnected(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-create"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee") // any Staff with practice access, no owner gating
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.InvoiceView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
	if out.PaidAt != nil {
		t.Fatalf("invoice.paidAt = %v, want nil", out.PaidAt)
	}

	if len(client.CreateInvoiceCalls) != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1", len(client.CreateInvoiceCalls))
	}
	call := client.CreateInvoiceCalls[0]
	if call.AccountID != accountID {
		t.Fatalf("CreateInvoice accountID = %q, want %q", call.AccountID, accountID)
	}
	// The Customer is resolved before the Invoice is raised (#780), and
	// her name and email -- and nothing else, no clinical field -- are all
	// that reach Stripe to make it.
	if len(client.CreateCustomerCalls) != 1 {
		t.Fatalf("CreateCustomer calls = %d, want 1", len(client.CreateCustomerCalls))
	}
	cust := client.CreateCustomerCalls[0]
	if cust.AccountID != accountID {
		t.Fatalf("CreateCustomer accountID = %q, want %q", cust.AccountID, accountID)
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

	if got := invoiceStatus(t, db, out.ID); got != invoiceStatusOpen {
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
			db := testdb.New(t)
			uid := "invoice-unbillable-" + status
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
			_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
			seedContractWithStatus(t, db, engagementID, status)
			client := payments.NewFakeClient()
			accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
				PracticeID:   practiceID,
				PracticeName: fixturePracticeName,
				BusinessURL:  fixtureOwnSiteURL,
			})
			if err != nil {
				t.Fatalf("fixture CreateAccount: %v", err)
			}
			seedConnectAccount(t, db, practiceID, accountID)

			srv, session := newInvoiceServer(t, db, uid, client)
			defer srv.Close()

			resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
			var out struct {
				Message string `json:"message"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !strings.Contains(out.Message, status) {
				t.Errorf("refusal message = %q, want it to name the actual status %q", out.Message, status)
			}
			if len(client.CreateInvoiceCalls) != 0 {
				t.Fatalf("CreateInvoice calls = %d, want 0 -- nothing must reach Stripe", len(client.CreateInvoiceCalls))
			}
			if len(client.CreateCustomerCalls) != 0 {
				t.Fatalf("CreateCustomer calls = %d, want 0 -- nothing must reach Stripe", len(client.CreateCustomerCalls))
			}
			if got := invoiceCount(t, db); got != 0 {
				t.Fatalf("invoices row count = %d, want 0", got)
			}
		})
	}
}

// TestPostInvoiceHandler_ClientWithNoEmailRefuses proves ADR-0017's
// ride-along: invoicing a Client with no email on file refuses rather
// than sending an empty string to Stripe.
func TestPostInvoiceHandler_ClientWithNoEmailRefuses(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-no-email"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "No Email Client", "")
	seedSignedContract(t, db, engagementID)
	fakeClient := payments.NewFakeClient()
	accountID, err := fakeClient.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv, session := newInvoiceServer(t, db, uid, fakeClient)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(fakeClient.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- must never send an empty string to Stripe", len(fakeClient.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_NoContractReturns404 proves an Engagement with no
// Contract yet 404s rather than creating an Invoice against nothing.
func TestPostInvoiceHandler_NoContractReturns404(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_MalformedEngagementIDReturns400 proves a
// syntactically invalid :engagementId is rejected before any DB lookup.
func TestPostInvoiceHandler_MalformedEngagementIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-malformed-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, "not-a-uuid", 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostInvoiceHandler_EngagementNotFoundReturns404 proves a
// well-formed but unknown :engagementId 404s.
func TestPostInvoiceHandler_EngagementNotFoundReturns404(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-unknown-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostInvoiceHandler_InvalidAmountReturns400 proves a zero or negative
// amountCents is rejected before any Stripe call.
func TestPostInvoiceHandler_InvalidAmountReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-invalid-amount"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	for _, amount := range []int64{0, -100} {
		resp := postInvoice(t, srv, session, practiceID, engagementID, amount)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("amountCents=%d: status = %d, want %d", amount, resp.StatusCode, http.StatusBadRequest)
		}
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_InvalidBodyReturns400 proves malformed JSON is
// rejected.
func TestPostInvoiceHandler_InvalidBodyReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoiceBody(t, srv, session, practiceID, engagementID, `not-json`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostInvoiceHandler_CreateInvoiceFailureReturns500AndPersistsNothing
// proves a Stripe failure while creating the draft Invoice surfaces as an
// internal error and never persists a row (the port failed before any
// insert happened).
func TestPostInvoiceHandler_CreateInvoiceFailureReturns500AndPersistsNothing(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-create-fail"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)
	client.CreateInvoiceErr = errStripeFake

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_FinalizeInvoiceFailureReturns500ButPersistsDraft
// proves that if FinalizeInvoice fails after the draft Invoice was
// already created on Stripe and inserted locally, the 500 response still
// leaves the draft row committed -- Doula Cloud never loses track of an
// Invoice that exists on Stripe.
func TestPostInvoiceHandler_FinalizeInvoiceFailureReturns500ButPersistsDraft(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-finalize-fail"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)
	client.FinalizeInvoiceErr = errStripeFake

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if got := invoiceCount(t, db); got != 1 {
		t.Fatalf("invoices row count = %d, want 1", got)
	}
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM invoices LIMIT 1`).Scan(&status); err != nil {
		t.Fatalf("query invoice status: %v", err)
	}
	if status != invoiceStatusDraft {
		t.Fatalf("persisted invoice status = %q, want %q", status, invoiceStatusDraft)
	}
}

// TestGetInvoicesHandler_ListsAcrossVoidedContract proves an Invoice
// billed against a since-voided Contract still lists under its
// Engagement, newest first, alongside an Invoice against the Contract
// that replaced it -- listing is scoped to the Engagement, not just "the
// current Contract row".
func TestGetInvoicesHandler_ListsAcrossVoidedContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-across-void"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	voidedContractID := seedContractWithStatus(t, db, engagementID, "voided")
	currentContractID := seedDraftContract(t, db, engagementID)

	base := time.Now().Add(-time.Hour)
	oldInvoiceID := seedInvoice(t, db, practiceID, voidedContractID, "in_old", invoiceStatusPaid, 10000, base)
	newInvoiceID := seedInvoice(t, db, practiceID, currentContractID, "in_new", invoiceStatusOpen, 20000, base.Add(time.Minute))

	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.ListInvoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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

// TestGetInvoicesHandler_PaidAtRoundTrips proves a paid Invoice's paid_at
// is surfaced in the response.
func TestGetInvoicesHandler_PaidAtRoundTrips(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-paid-at"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid", invoiceStatusPaid, 10000, time.Now())
	paidAt := time.Now().Round(time.Second)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = $1 WHERE id = $2`, paidAt, invoiceID); err != nil {
		t.Fatalf("seed paid_at: %v", err)
	}

	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()

	var out payments.ListInvoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].PaidAt == nil {
		t.Fatalf("items = %+v, want one item with paidAt set", out.Items)
	}
	if !out.Items[0].PaidAt.Equal(paidAt) {
		t.Fatalf("paidAt = %v, want %v", out.Items[0].PaidAt, paidAt)
	}
}

// TestGetInvoicesHandler_MalformedEngagementIDReturns400 proves a
// syntactically invalid :engagementId is rejected before any DB lookup.
func TestGetInvoicesHandler_MalformedEngagementIDReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-malformed-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, "not-a-uuid", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestGetInvoicesHandler_EngagementNotFoundReturns404 proves a
// well-formed but unknown :engagementId 404s.
func TestGetInvoicesHandler_EngagementNotFoundReturns404(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-unknown-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, "00000000-0000-0000-0000-000000000000", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInvoicesHandler_EmptyBeforeAnyContract proves an Engagement with
// no Contract (and therefore no Invoices) yet returns an empty list, not
// a 404 -- listing tolerates "nothing yet" the way creation doesn't.
func TestGetInvoicesHandler_EmptyBeforeAnyContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.ListInvoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("items = %d, want 0", len(out.Items))
	}
}

// TestGetInvoicesHandler_PaginatesWithCursor proves a page beyond
// invoicePageSize (30) sets hasMore/nextCursor, and that cursor correctly
// resumes on the next page.
func TestGetInvoicesHandler_PaginatesWithCursor(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-paginate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedDraftContract(t, db, engagementID)

	const total = 31
	base := time.Now().Add(-time.Hour)
	ids := make([]string, total)
	for i := range total {
		ids[i] = seedInvoice(t, db, practiceID, contractID, "in_page_"+strconv.Itoa(i), invoiceStatusOpen, int64(1000+i), base.Add(time.Duration(i)*time.Second))
	}

	client := payments.NewFakeClient()
	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	first := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer first.Body.Close()
	var firstPage payments.ListInvoicesResponse
	if err := json.NewDecoder(first.Body).Decode(&firstPage); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
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

	second := getInvoices(t, srv, session, practiceID, engagementID, *firstPage.NextCursor)
	defer second.Body.Close()
	var secondPage payments.ListInvoicesResponse
	if err := json.NewDecoder(second.Body).Decode(&secondPage); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
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

// TestGetInvoicesHandler_InvalidCursorReturns400 proves every way a
// caller-supplied cursor can fail to decode is rejected with 400, rather
// than a panic or a silently wrong page.
func TestGetInvoicesHandler_InvalidCursorReturns400(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-list-bad-cursor"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	cases := map[string]string{
		// '!' is outside the URL-safe base64 alphabet and has no special
		// meaning in a query string (unlike '%'), so this fails to decode
		// without confusing percent-encoding.
		"not valid base64":            "!!!not-valid-base64!!!",
		"valid base64, no separator":  base64.URLEncoding.EncodeToString([]byte("nosep")),
		"valid base64, bad timestamp": base64.URLEncoding.EncodeToString([]byte("not-a-time|some-id")),
	}
	for name, cursor := range cases {
		resp := getInvoices(t, srv, session, practiceID, engagementID, cursor)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want %d", name, resp.StatusCode, http.StatusBadRequest)
		}
	}
}

// postInvoiceWithBillingMode is postInvoiceBody with a billingMode field
// set on the request, for #271's "ask once, inline" path.
// postInvoiceWithBillingModeAmountCents is the fixed amount every caller
// of postInvoiceWithBillingMode wants -- none of them are testing the
// amount, so it is not a parameter (golangci-lint's unparam).
const postInvoiceWithBillingModeAmountCents = 15000

func postInvoiceWithBillingMode(t *testing.T, srv *httptest.Server, session, practiceID, engagementID, billingMode string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.CreateInvoiceRequest{AmountCents: postInvoiceWithBillingModeAmountCents, BillingMode: &billingMode})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return postInvoiceBody(t, srv, session, practiceID, engagementID, string(body))
}

// billingModeOfPractice reads practiceID's raw billing_mode column.
func billingModeOfPractice(t *testing.T, db *testdb.DB, practiceID string) (mode string, ok bool, err error) {
	t.Helper()
	var raw sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT billing_mode FROM practices WHERE id = $1`, practiceID).Scan(&raw); err != nil {
		return "", false, fmt.Errorf("query billing mode: %w", err)
	}
	return raw.String, raw.Valid, nil
}

// TestPostInvoiceHandler_BillingModeRequiredWhenUnset proves #271's
// refusal when a Practice has never chosen a billing mode and the request
// raising its first Invoice supplies none either.
func TestPostInvoiceHandler_BillingModeRequiredWhenUnset(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-billing-mode-required"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	var out struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Message != payments.MsgBillingModeRequired {
		t.Fatalf("message = %q, want %q", out.Message, payments.MsgBillingModeRequired)
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_BillingModeInvalidValueRefused proves a
// nonsense billingMode value 400s rather than being written.
func TestPostInvoiceHandler_BillingModeInvalidValueRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-billing-mode-invalid"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, "cash_app")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	mode, ok, err := billingModeOfPractice(t, db, practiceID)
	if err != nil {
		t.Fatalf("query billing mode: %v", err)
	}
	if ok {
		t.Fatalf("billing_mode = %q, want unset after a rejected value", mode)
	}
}

// TestPostInvoiceHandler_ByHandInvoiceCreatedOpenNoStripeCall proves
// #271's by-hand rail: raised 'open' immediately, no Stripe call at all,
// a sequence-derived reference, and the Practice's billing_mode is
// persisted from the request's first-ever value.
func TestPostInvoiceHandler_ByHandInvoiceCreatedOpenNoStripeCall(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-by-hand-create"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, string(payments.BillingModeByHand))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.InvoiceView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- a by-hand Invoice must never reach Stripe", len(client.CreateInvoiceCalls))
	}
	if len(client.FinalizeInvoiceIDs) != 0 {
		t.Fatalf("FinalizeInvoice calls = %d, want 0", len(client.FinalizeInvoiceIDs))
	}
	mode, ok, err := billingModeOfPractice(t, db, practiceID)
	if err != nil {
		t.Fatalf("query billing mode: %v", err)
	}
	if !ok || mode != string(payments.BillingModeByHand) {
		t.Fatalf("billing_mode = (%q, ok=%v), want (%q, true)", mode, ok, payments.BillingModeByHand)
	}
}

// TestPostInvoiceHandler_ByHandInvoiceSequenceIncrementsAcrossInvoices
// proves the per-Practice reference sequence claims a distinct number for
// each by-hand Invoice, in order.
func TestPostInvoiceHandler_ByHandInvoiceSequenceIncrementsAcrossInvoices(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-by-hand-sequence"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	first := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, string(payments.BillingModeByHand))
	var firstOut payments.InvoiceView
	if err := json.NewDecoder(first.Body).Decode(&firstOut); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	_ = first.Body.Close()

	// The mode is already set now, so the second request need not (and,
	// per #271, must not have its own value honored) repeat billingMode.
	second := postInvoice(t, srv, session, practiceID, engagementID, 22000)
	var secondOut payments.InvoiceView
	if err := json.NewDecoder(second.Body).Decode(&secondOut); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	_ = second.Body.Close()

	if firstOut.Reference != "INV-0001" {
		t.Fatalf("first reference = %q, want %q", firstOut.Reference, "INV-0001")
	}
	if secondOut.Reference != "INV-0002" {
		t.Fatalf("second reference = %q, want %q", secondOut.Reference, "INV-0002")
	}
}

// TestPostInvoiceHandler_ByHandInvoiceIgnoresLaterBillingModeRequest
// proves #271's "changing the mode is the Owner's alone, afterward" rule
// from the create side: once a Practice's billing_mode is set, a later
// Invoice-raise request naming a different mode is silently ignored
// (the established mode governs), never silently switched.
func TestPostInvoiceHandler_ByHandInvoiceIgnoresLaterBillingModeRequest(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-by-hand-ignore-later-mode"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	first := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, string(payments.BillingModeByHand))
	_ = first.Body.Close()

	// This request names "stripe", but the Practice already chose
	// by_hand -- it must be ignored, and this Invoice raised by hand too.
	resp := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, string(payments.BillingModeStripe))
	defer resp.Body.Close()
	var out payments.InvoiceView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("invoice.billingMode = %q, want %q (the already-established mode)", out.BillingMode, payments.BillingModeByHand)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0 -- billingMode=stripe must be ignored once a mode is set", len(client.CreateInvoiceCalls))
	}
}

// TestPostInvoiceHandler_ByHandInvoiceClientWithNoEmailStillSucceeds
// amends #430's refusal (errClientNoEmail): a by-hand Invoice mails
// nothing, so an absent Client email must not block it.
func TestPostInvoiceHandler_ByHandInvoiceClientWithNoEmailStillSucceeds(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-by-hand-no-email"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoiceWithBillingMode(t, srv, session, practiceID, engagementID, string(payments.BillingModeByHand))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
}

// TestPostInvoiceHandler_StripeInvoiceUsesStripeNumberAsReference proves
// #271's Stripe-rail reference: once FinalizeInvoice succeeds, the
// persisted Invoice's reference is Stripe's own `number`, not its
// internal invoice id.
func TestPostInvoiceHandler_StripeInvoiceUsesStripeNumberAsReference(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-stripe-reference"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedSignedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), payments.AccountProfile{
		PracticeID:   practiceID,
		PracticeName: fixturePracticeName,
		BusinessURL:  fixtureOwnSiteURL,
	})
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv, session := newInvoiceServer(t, db, uid, client)
	defer srv.Close()

	resp := postInvoice(t, srv, session, practiceID, engagementID, 15000)
	defer resp.Body.Close()
	var out payments.InvoiceView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
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
	db := testdb.New(t)
	const uid = "invoice-list-by-hand-mode"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedSignedContract(t, db, engagementID)
	seedByHandInvoice(t, db, practiceID, contractID)

	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getInvoices(t, srv, session, practiceID, engagementID, "")
	defer resp.Body.Close()
	var out payments.ListInvoicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(out.Items))
	}
	if out.Items[0].BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("billingMode = %q, want %q", out.Items[0].BillingMode, payments.BillingModeByHand)
	}
}
