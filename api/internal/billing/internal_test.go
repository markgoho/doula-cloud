package billing_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

const internalTestSecret = "internal-test-secret"

func newInternalBillingServer(db *testdb.DB, client billing.StripeClient) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("POST /api/internal/billing/refunds", billing.RefundHandler(db.App, client, internalTestSecret))
	mux.Handle("GET /api/internal/billing/dormant-practices", billing.DormantPracticesHandler(db.App, internalTestSecret))
	return httptest.NewServer(mux)
}

// internalRequest calls one of the internal endpoints and returns the
// status and the whole response body, read and closed here so no caller
// has to remember to.
func internalRequest(t *testing.T, srv *httptest.Server, method, path, secret, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if secret != "" {
		req.Header.Set("X-Internal-Secret", secret)
	}
	// One name per test, which is what a retry within a test would
	// reuse. TestRefundHandler_RefusesAnUnnamedRequest builds its own
	// request instead, to send none.
	req.Header.Set("Idempotency-Key", t.Name())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	read, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, read
}

// TestRefundHandler_IssuesTheRefundAndRecordsIt proves the endpoint that
// honours a refund request: Stripe is called against the original
// payment, and the ledger keeps the receipt.
func TestRefundHandler_IssuesTheRefundAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Refund Endpoint")
	seedPurchase(t, db, practiceID, 3, 2000, 300, "pi_endpoint", time.Now())
	client := billing.NewFakeStripeClient()
	srv := newInternalBillingServer(db, client)

	status, body := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/refunds", internalTestSecret,
		`{"practiceId":"`+practiceID+`","quantity":2}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var receipt billing.RefundReceipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if receipt.Credits != 2 || receipt.AmountCents != 4200 || receipt.PaymentIntentID != "pi_endpoint" {
		t.Fatalf("receipt = %+v, want 2 credits at 4200 cents against pi_endpoint", receipt)
	}

	var rows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE practice_id = $1 AND origin = 'refund'`, practiceID,
	).Scan(&rows); err != nil {
		t.Fatalf("count refund rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("refund rows = %d, want 1 committed", rows)
	}
	if calls := client.RefundCalls(); len(calls) != 1 {
		t.Fatalf("stripe refund calls = %+v, want 1", calls)
	}
}

// TestRefundHandler_RefusesAnUnnamedRequest proves a refund request must
// name itself: without the header there is nothing to recognise a retry
// by, and the retry would move the money a second time.
func TestRefundHandler_RefusesAnUnnamedRequest(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Unnamed")
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/internal/billing/refunds",
		bytes.NewReader([]byte(`{"practiceId":"`+practiceID+`","quantity":1}`)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Internal-Secret", internalTestSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestRefundHandler_RefusesWithoutTheInternalSecret proves the endpoint
// is not reachable by a Practice, or by anyone else without the operator
// secret -- the same guard every /api/internal endpoint carries.
func TestRefundHandler_RefusesWithoutTheInternalSecret(t *testing.T) {
	db := testdb.New(t)
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	for _, secret := range []string{"", "wrong-secret"} {
		status, _ := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/refunds", secret, `{}`)
		if status != http.StatusUnauthorized {
			t.Fatalf("secret %q: status = %d, want 401", secret, status)
		}
	}
}

// TestRefundHandler_RejectsMalformedRequests proves what the endpoint
// refuses before it ever reaches the ledger.
func TestRefundHandler_RejectsMalformedRequests(t *testing.T) {
	db := testdb.New(t)
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	for name, body := range map[string]string{
		"not json":           `{`,
		"practice not uuid":  `{"practiceId":"not-a-uuid","quantity":1}`,
		"quantity below one": `{"practiceId":"11111111-1111-1111-1111-111111111111","quantity":0}`,
	} {
		status, _ := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/refunds", internalTestSecret, body)
		if status != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, status)
		}
	}
}

// TestRefundHandler_ReportsARefusalAsAConflict proves a refusal the
// ledger makes -- nothing refundable -- reaches the caller as its own
// answer rather than as an internal error.
func TestRefundHandler_ReportsARefusalAsAConflict(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Nothing To Refund")
	seedSignupBonus(t, db, practiceID)
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	status, _ := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/refunds", internalTestSecret,
		`{"practiceId":"`+practiceID+`","quantity":1}`)
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
}

// TestRefundHandler_ReportsAStripeFailureAsAnError proves a refund Stripe
// refused is not reported as done.
func TestRefundHandler_ReportsAStripeFailureAsAnError(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Stripe Refuses")
	seedPurchase(t, db, practiceID, 1, 2000, 0, "pi_refused", time.Now())
	client := billing.NewFakeStripeClient()
	client.RefundPaymentErr = errStripeUnavailable
	srv := newInternalBillingServer(db, client)

	status, _ := internalRequest(t, srv, http.MethodPost, "/api/internal/billing/refunds", internalTestSecret,
		`{"practiceId":"`+practiceID+`","quantity":1}`)
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", status)
	}
}

// TestDormantPracticesHandler_ListsBalancesNobodyHasTouched proves the
// mailing list is readable through the same guard, and only through it.
func TestDormantPracticesHandler_ListsBalancesNobodyHasTouched(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Dormant Endpoint")
	seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_dormant_endpoint", time.Now().AddDate(-3, 0, 0))
	srv := newInternalBillingServer(db, billing.NewFakeStripeClient())

	if status, _ := internalRequest(t, srv, http.MethodGet, "/api/internal/billing/dormant-practices", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("status without the secret = %d, want 401", status)
	}

	status, body := internalRequest(t, srv, http.MethodGet, "/api/internal/billing/dormant-practices", internalTestSecret, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	var dormant []billing.DormantPractice
	if err := json.Unmarshal(body, &dormant); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(dormant) != 1 || dormant[0].PracticeID != practiceID || dormant[0].Balance != 2 {
		t.Fatalf("dormant = %+v, want the seeded Practice with 2 Credits", dormant)
	}
}
