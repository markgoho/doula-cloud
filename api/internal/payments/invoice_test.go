package payments_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// seedEngagement inserts a Client (with the given name/email, so tests can
// assert exactly what reaches the fake Stripe port) and an Engagement
// linking them to practiceID, using the superuser Admin connection.
func seedEngagement(t *testing.T, db *testdb.DB, practiceID, clientName, clientEmail string) (engagementID string) {
	t.Helper()
	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`, clientName, clientEmail,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id) VALUES ($1, $2) RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID
}

// seedContractWithStatus seeds a Contract row directly, bypassing the
// contracts package's own handlers (this package has no dependency on
// it), with an explicit status so tests can prove GetInvoicesHandler still
// lists Invoices billed against a since-voided Contract.
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

func seedContract(t *testing.T, db *testdb.DB, engagementID string) (contractID string) {
	t.Helper()
	return seedContractWithStatus(t, db, engagementID, "draft")
}

// seedConnectAccount sets practiceID's stored Stripe Connect account id
// directly, bypassing PostConnectHandler.
func seedConnectAccount(t *testing.T, db *testdb.DB, practiceID, accountID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_account_id = $1 WHERE id = $2`, accountID, practiceID,
	); err != nil {
		t.Fatalf("seed connect account: %v", err)
	}
}

// seedInvoice inserts an invoices row directly at an explicit createdAt,
// so listing-order tests are deterministic rather than racing against
// now()'s resolution.
func seedInvoice(t *testing.T, db *testdb.DB, practiceID, contractID, stripeInvoiceID, status string, amountCents int64, createdAt time.Time) (invoiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, created_at)
		 VALUES ($1, $2, $3, $4::invoice_status, $5, 'usd', $6) RETURNING id`,
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

func newInvoiceServer(verifier fakeVerifier, db *testdb.DB, client payments.Client) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(verifier, db.App)(payments.PostInvoiceHandler(client)))
	mux.Handle("GET /practices/{practiceId}/engagements/{engagementId}/contract/invoices",
		staffauth.Middleware(verifier, db.App)(payments.GetInvoicesHandler()))
	return httptest.NewServer(mux)
}

func postInvoiceBody(t *testing.T, srv *httptest.Server, practiceID, engagementID, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/contract/invoices", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func postInvoice(t *testing.T, srv *httptest.Server, practiceID, engagementID string, amountCents int64) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.CreateInvoiceRequest{AmountCents: amountCents})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return postInvoiceBody(t, srv, practiceID, engagementID, string(body))
}

func getInvoices(t *testing.T, srv *httptest.Server, practiceID, engagementID, cursor string) *http.Response {
	t.Helper()
	url := srv.URL + "/practices/" + practiceID + "/engagements/" + engagementID + "/contract/invoices"
	if cursor != "" {
		url += "?cursor=" + cursor
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestPostInvoiceHandler_NotConnectedOwnerGetsConnectRequired proves an
// Owner attempting to create the first Invoice at an unconnected Practice
// gets routed toward the #79 connect flow instead of an Invoice, and that
// nothing is created or sent to Stripe.
func TestPostInvoiceHandler_NotConnectedOwnerGetsConnectRequired(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-gate-owner"
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.PostInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.ConnectRequired {
		t.Fatal("connectRequired = false, want true")
	}
	if !out.IsOwner {
		t.Fatal("isOwner = false, want true")
	}
	if out.Invoice != nil {
		t.Fatalf("invoice = %+v, want nil", out.Invoice)
	}
	if len(client.CreateInvoiceCalls) != 0 {
		t.Fatalf("CreateInvoice calls = %d, want 0", len(client.CreateInvoiceCalls))
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
	}
}

// TestPostInvoiceHandler_NotConnectedNonOwnerGetsAskAnOwnerState proves a
// non-Owner Staff member gets the same connectRequired gate but with
// isOwner false, so the frontend shows the static "ask an Owner" message
// instead of a connect button.
func TestPostInvoiceHandler_NotConnectedNonOwnerGetsAskAnOwnerState(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-gate-non-owner"
	practiceID := seedMember(t, db, uid) // doula role, not owner
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.PostInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.ConnectRequired {
		t.Fatal("connectRequired = false, want true")
	}
	if out.IsOwner {
		t.Fatal("isOwner = true, want false")
	}
	if got := invoiceCount(t, db); got != 0 {
		t.Fatalf("invoices row count = %d, want 0", got)
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
	practiceID := seedMember(t, db, uid) // any Staff with practice access, no owner gating
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), practiceID)
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var out payments.PostInvoiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ConnectRequired {
		t.Fatal("connectRequired = true, want false")
	}
	if out.Invoice == nil {
		t.Fatal("invoice is nil, want a created Invoice")
	}
	if out.Invoice.ContractID != contractID {
		t.Fatalf("invoice.contractId = %q, want %q", out.Invoice.ContractID, contractID)
	}
	if out.Invoice.Status != "open" {
		t.Fatalf("invoice.status = %q, want %q", out.Invoice.Status, "open")
	}
	if out.Invoice.AmountCents != 15000 {
		t.Fatalf("invoice.amountCents = %d, want 15000", out.Invoice.AmountCents)
	}
	if out.Invoice.Currency != "usd" {
		t.Fatalf("invoice.currency = %q, want %q", out.Invoice.Currency, "usd")
	}
	if out.Invoice.PaidAt != nil {
		t.Fatalf("invoice.paidAt = %v, want nil", out.Invoice.PaidAt)
	}

	if len(client.CreateInvoiceCalls) != 1 {
		t.Fatalf("CreateInvoice calls = %d, want 1", len(client.CreateInvoiceCalls))
	}
	call := client.CreateInvoiceCalls[0]
	if call.AccountID != accountID {
		t.Fatalf("CreateInvoice accountID = %q, want %q", call.AccountID, accountID)
	}
	if call.CustomerEmail != "jane@example.com" || call.CustomerName != "Jane Client" {
		t.Fatalf("CreateInvoice customer = (%q, %q), want (%q, %q)", call.CustomerName, call.CustomerEmail, "Jane Client", "jane@example.com")
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

	if got := invoiceStatus(t, db, out.Invoice.ID); got != "open" {
		t.Fatalf("persisted invoice status = %q, want %q", got, "open")
	}
}

// TestPostInvoiceHandler_NoContractReturns404 proves an Engagement with no
// Contract yet 404s rather than creating an Invoice against nothing.
func TestPostInvoiceHandler_NoContractReturns404(t *testing.T) {
	db := testdb.New(t)
	const uid = "invoice-no-contract"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
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
	practiceID := seedMember(t, db, uid)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, "not-a-uuid", 15000)
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
	practiceID := seedMember(t, db, uid)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, "00000000-0000-0000-0000-000000000000", 15000)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), practiceID)
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	for _, amount := range []int64{0, -100} {
		resp := postInvoice(t, srv, practiceID, engagementID, amount)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), practiceID)
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoiceBody(t, srv, practiceID, engagementID, `not-json`)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), practiceID)
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)
	client.CreateInvoiceErr = errStripeFake

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	seedContract(t, db, engagementID)
	client := payments.NewFakeClient()
	accountID, err := client.CreateAccount(t.Context(), practiceID)
	if err != nil {
		t.Fatalf("fixture CreateAccount: %v", err)
	}
	seedConnectAccount(t, db, practiceID, accountID)
	client.FinalizeInvoiceErr = errStripeFake

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := postInvoice(t, srv, practiceID, engagementID, 15000)
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
	if status != "draft" {
		t.Fatalf("persisted invoice status = %q, want %q", status, "draft")
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	voidedContractID := seedContractWithStatus(t, db, engagementID, "voided")
	currentContractID := seedContract(t, db, engagementID)

	base := time.Now().Add(-time.Hour)
	oldInvoiceID := seedInvoice(t, db, practiceID, voidedContractID, "in_old", "paid", 10000, base)
	newInvoiceID := seedInvoice(t, db, practiceID, currentContractID, "in_new", "open", 20000, base.Add(time.Minute))

	client := payments.NewFakeClient()
	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := getInvoices(t, srv, practiceID, engagementID, "")
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)
	invoiceID := seedInvoice(t, db, practiceID, contractID, "in_paid", "paid", 10000, time.Now())
	paidAt := time.Now().Round(time.Second)
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE invoices SET paid_at = $1 WHERE id = $2`, paidAt, invoiceID); err != nil {
		t.Fatalf("seed paid_at: %v", err)
	}

	client := payments.NewFakeClient()
	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := getInvoices(t, srv, practiceID, engagementID, "")
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
	practiceID := seedMember(t, db, uid)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := getInvoices(t, srv, practiceID, "not-a-uuid", "")
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
	practiceID := seedMember(t, db, uid)
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := getInvoices(t, srv, practiceID, "00000000-0000-0000-0000-000000000000", "")
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	resp := getInvoices(t, srv, practiceID, engagementID, "")
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	contractID := seedContract(t, db, engagementID)

	const total = 31
	base := time.Now().Add(-time.Hour)
	ids := make([]string, total)
	for i := range total {
		ids[i] = seedInvoice(t, db, practiceID, contractID, "in_page_"+strconv.Itoa(i), "open", int64(1000+i), base.Add(time.Duration(i)*time.Second))
	}

	client := payments.NewFakeClient()
	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
	defer srv.Close()

	first := getInvoices(t, srv, practiceID, engagementID, "")
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

	second := getInvoices(t, srv, practiceID, engagementID, *firstPage.NextCursor)
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
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID, "Jane Client", "jane@example.com")
	client := payments.NewFakeClient()

	srv := newInvoiceServer(fakeVerifier{uid: uid}, db, client)
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
		resp := getInvoices(t, srv, practiceID, engagementID, cursor)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want %d", name, resp.StatusCode, http.StatusBadRequest)
		}
	}
}
