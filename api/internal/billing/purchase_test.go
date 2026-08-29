package billing_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// errStripeFake is returned by FakeStripeClient methods in tests that
// exercise a handler's Stripe-failure path.
var errStripeFake = errors.New("stripe: fake failure")

// seedOwner seeds a Practice and a Staff member holding the owner role
// there -- PostPurchaseHandler is Owner-only, unlike GetBalanceHandler.
func seedOwner(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	return practiceID
}

func newPurchaseServer(t *testing.T, db *testdb.DB, uid string, stripeClient billing.StripeClient) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/billing/purchases",
		staffauth.Middleware(db.App)(billing.PostPurchaseHandler(stripeClient)))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postPurchase(t *testing.T, srv *httptest.Server, session string, practiceID string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/practices/"+practiceID+"/billing/purchases", bytes.NewBufferString(body))
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

func stripeCustomerID(t *testing.T, db *testdb.DB, practiceID string) *string {
	t.Helper()
	var id *string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT stripe_customer_id FROM practices WHERE id = $1`, practiceID).Scan(&id); err != nil {
		t.Fatalf("query stripe_customer_id: %v", err)
	}
	return id
}

// TestPostPurchaseHandler_OwnerCreatesCustomerAndCheckoutSession proves an
// Owner's first purchase lazily creates a Stripe Customer, persists its id
// on the Practice, and returns a Checkout Session URL tagged with the
// right metadata.
func TestPostPurchaseHandler_OwnerCreatesCustomerAndCheckoutSession(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-owner"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out billing.PurchaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.CheckoutURL == "" {
		t.Fatal("checkoutUrl is empty")
	}

	calls := stripeClient.Calls()
	if len(calls) != 1 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 1", len(calls))
	}
	if calls[0].PracticeID != practiceID || calls[0].Quantity != 5 {
		t.Fatalf("checkout session call = %+v, want practiceID %q, quantity 5", calls[0], practiceID)
	}
	// The sole Staff member works in New York, so the sale is wholly
	// taxable and the Checkout page keeps its ordinary single line item.
	if calls[0].NewYorkStaff != 1 || calls[0].TotalStaff != 1 {
		t.Fatalf("apportionment = %d of %d, want 1 of 1", calls[0].NewYorkStaff, calls[0].TotalStaff)
	}

	id := stripeCustomerID(t, db, practiceID)
	if id == nil || *id == "" {
		t.Fatal("stripe_customer_id was not persisted")
	}
	if calls[0].CustomerID != *id {
		t.Fatalf("checkout session customer id = %q, want %q", calls[0].CustomerID, *id)
	}
}

// TestPostPurchaseHandler_AdminCanPurchase proves ADR-0017's correction:
// an Admin, not only an Owner, may buy Credits.
func TestPostPurchaseHandler_AdminCanPurchase(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-admin"
	practiceID := seedMember(t, db, uid, "{admin}")
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 3}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out billing.PurchaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.CheckoutURL == "" {
		t.Fatal("checkoutUrl is empty")
	}
}

// TestPostPurchaseHandler_SecondPurchaseReusesExistingCustomer proves a
// Practice's second purchase does not create a second Stripe Customer.
func TestPostPurchaseHandler_SecondPurchaseReusesExistingCustomer(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-owner-repeat"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	first := postPurchase(t, srv, session, practiceID, `{"quantity": 3}`)
	_ = first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first purchase status = %d, want %d", first.StatusCode, http.StatusOK)
	}
	firstCustomerID := stripeCustomerID(t, db, practiceID)

	second := postPurchase(t, srv, session, practiceID, `{"quantity": 10}`)
	_ = second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second purchase status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	if got := stripeClient.CustomerCallCount(); got != 1 {
		t.Fatalf("CreateCustomer call count = %d, want 1", got)
	}
	if got := stripeCustomerID(t, db, practiceID); got == nil || *got != *firstCustomerID {
		t.Fatalf("stripe_customer_id changed across purchases: first %v, second %v", firstCustomerID, got)
	}
	if got := len(stripeClient.Calls()); got != 2 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 2", got)
	}
}

// TestPostPurchaseHandler_NonOwnerForbidden proves a non-Owner Staff
// member cannot initiate a purchase.
func TestPostPurchaseHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-non-owner"
	practiceID := seedMember(t, db, uid, "{doula}") // doula role, not owner
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if got := len(stripeClient.Calls()); got != 0 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 0", got)
	}
}

// TestPostPurchaseHandler_InvalidQuantityRejected proves a non-positive
// quantity is rejected before any Stripe call is made.
func TestPostPurchaseHandler_InvalidQuantityRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-bad-quantity"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 0}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := len(stripeClient.Calls()); got != 0 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 0", got)
	}
}

// TestPostPurchaseHandler_InvalidBodyRejected proves malformed JSON is
// rejected with a 400.
func TestPostPurchaseHandler_InvalidBodyRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-bad-body"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `not json`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPostPurchaseHandler_CreateCustomerFailureReturns500 proves a Stripe
// Customer-creation failure surfaces as an internal error and never
// persists a customer id.
func TestPostPurchaseHandler_CreateCustomerFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-customer-fail"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()
	stripeClient.CreateCustomerErr = errStripeFake

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if got := stripeCustomerID(t, db, practiceID); got != nil {
		t.Fatalf("stripe_customer_id = %v, want nil (never persisted)", got)
	}
}

// TestPostPurchaseHandler_CreateCheckoutSessionFailureReturns500 proves a
// Stripe Checkout Session-creation failure surfaces as an internal error.
func TestPostPurchaseHandler_CreateCheckoutSessionFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-checkout-fail"
	practiceID := seedOwner(t, db, uid)
	stripeClient := billing.NewFakeStripeClient()
	stripeClient.CreateCheckoutSessionErr = errStripeFake

	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 5}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// seedStaffInState seeds a Staff member who works in workState and gives
// her a Membership at practiceID.
func seedStaffInState(t *testing.T, db *testdb.DB, practiceID, identityUID, workState string) {
	t.Helper()
	var staffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $1, $1 || '@example.com', $2) RETURNING id`,
		identityUID, workState,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff %q in %q: %v", identityUID, workState, err)
	}
	seedMembership(t, db, practiceID, staffID, "{doula}")
}

// TestPostPurchaseHandler_ApportionsByWhereStaffWork proves the headcount
// New York's sales tax is computed on (#389) is counted from the
// Practice's own roster: two of its four people work in New York, and
// that pair -- not the quantity, and not the caseload -- is what reaches
// Stripe.
func TestPostPurchaseHandler_ApportionsByWhereStaffWork(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-apportion-owner"
	practiceID := seedOwner(t, db, uid) // the Owner herself works in NY
	seedStaffInState(t, db, practiceID, "purchase-apportion-ny", "NY")
	seedStaffInState(t, db, practiceID, "purchase-apportion-nj", "NJ")
	seedStaffInState(t, db, practiceID, "purchase-apportion-ca", "CA")

	// A second Practice's out-of-state doula must not dilute the ratio.
	other := seedPractice(t, db, "Other Practice")
	seedStaffInState(t, db, other, "purchase-apportion-other", "TX")

	stripeClient := billing.NewFakeStripeClient()
	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 20}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	calls := stripeClient.Calls()
	if len(calls) != 1 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 1", len(calls))
	}
	if calls[0].NewYorkStaff != 2 || calls[0].TotalStaff != 4 {
		t.Fatalf("apportionment = %d of %d, want 2 of 4", calls[0].NewYorkStaff, calls[0].TotalStaff)
	}
}

// TestPostPurchaseHandler_WhollyOutOfStatePracticeCountsNoNewYorkStaff
// proves a Practice with nobody in New York reaches Stripe with a zero
// numerator, which is what makes her purchase carry no New York tax.
func TestPostPurchaseHandler_WhollyOutOfStatePracticeCountsNoNewYorkStaff(t *testing.T) {
	db := testdb.New(t)
	const uid = "purchase-out-of-state-owner"
	practiceID := seedPractice(t, db, "Out Of State Practice")
	var ownerID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, 'Owner', 'oos-owner@example.com', 'NJ') RETURNING id`,
		uid,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	seedMembership(t, db, practiceID, ownerID, "{owner}")
	seedStaffInState(t, db, practiceID, "purchase-out-of-state-pa", "PA")

	stripeClient := billing.NewFakeStripeClient()
	srv, session := newPurchaseServer(t, db, uid, stripeClient)
	defer srv.Close()

	resp := postPurchase(t, srv, session, practiceID, `{"quantity": 4}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	calls := stripeClient.Calls()
	if len(calls) != 1 {
		t.Fatalf("CreateCheckoutSession calls = %d, want 1", len(calls))
	}
	if calls[0].NewYorkStaff != 0 || calls[0].TotalStaff != 2 {
		t.Fatalf("apportionment = %d of %d, want 0 of 2", calls[0].NewYorkStaff, calls[0].TotalStaff)
	}
}
