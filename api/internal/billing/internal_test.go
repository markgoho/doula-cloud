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
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/testdb"
)

const internalTestSecret = "internal-test-secret"

func newInternalBillingServer(db *testdb.DB, client billing.StripeClient) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /api/internal/billing/dormant-practices", billing.DormantPracticesHandler(db.App, internalauth.FromSecret(internalTestSecret)))
	mux.Handle("POST /api/internal/billing/founding-grants", billing.FoundingGrantHandler(db.App, internalauth.FromSecret(internalTestSecret)))
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
	// reuse.
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

// TestDormantPracticesHandler_ListsBalancesNobodyHasTouched proves the
// mailing list is readable through the same guard, and only through it.
func TestDormantPracticesHandler_ListsBalancesNobodyHasTouched(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Dormant Endpoint")
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
