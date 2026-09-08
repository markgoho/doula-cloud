package payments_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

func getBillingMode(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", nil)
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

func putBillingMode(t *testing.T, srv *httptest.Server, session, practiceID, mode string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payments.PutBillingModeRequest{BillingMode: mode})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", bytes.NewBuffer(body))
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

// TestGetBillingModeHandler_UnsetReturnsNoValue proves a Practice that
// has never chosen a billing mode reads back an empty BillingModeView.
func TestGetBillingModeHandler_UnsetReturnsNoValue(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-unset"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getBillingMode(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.BillingModeView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.BillingMode != nil {
		t.Fatalf("billingMode = %v, want nil", *out.BillingMode)
	}
}

// TestGetBillingModeHandler_AnyStaffCanRead proves a Doula -- who cannot
// read Invoice/Payment money at all under ADR-0008 -- can still read this
// one Practice-level fact, per #270's own reasoning for the sibling
// "whether the Practice can raise an Invoice at all" row.
func TestGetBillingModeHandler_AnyStaffCanRead(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-doula-read"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeByHand))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := getBillingMode(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.BillingModeView
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.BillingMode == nil || *out.BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("billingMode = %v, want %q", out.BillingMode, payments.BillingModeByHand)
	}
}

// TestPutBillingModeHandler_OwnerCanChangeEstablishedMode proves an
// Owner may change an already-established billing_mode, and the change
// is recorded to the Activity log.
func TestPutBillingModeHandler_OwnerCanChangeEstablishedMode(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-owner-change"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := putBillingMode(t, srv, session, practiceID, string(payments.BillingModeByHand))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	mode, ok, err := billingModeOfPractice(t, db, practiceID)
	if err != nil {
		t.Fatalf("query billing mode: %v", err)
	}
	if !ok || mode != string(payments.BillingModeByHand) {
		t.Fatalf("billing_mode = (%q, ok=%v), want (%q, true)", mode, ok, payments.BillingModeByHand)
	}

	var actionCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE practice_id = $1 AND subject_kind = 'practice' AND action = 'billing_mode_changed'`,
		practiceID,
	).Scan(&actionCount); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("billing_mode_changed activity rows = %d, want 1", actionCount)
	}
}

// TestPutBillingModeHandler_NonOwnerForbidden proves an Admin -- who may
// read this fact -- may not change it once established; only an Owner
// may (#271, by analogy to Connect onboarding).
func TestPutBillingModeHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-admin-forbidden"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := putBillingMode(t, srv, session, practiceID, string(payments.BillingModeByHand))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	mode, _, err := billingModeOfPractice(t, db, practiceID)
	if err != nil {
		t.Fatalf("query billing mode: %v", err)
	}
	if mode != string(payments.BillingModeStripe) {
		t.Fatalf("billing_mode = %q, want unchanged %q", mode, payments.BillingModeStripe)
	}
}

// TestPutBillingModeHandler_MalformedBodyRefused proves invalid JSON
// 400s.
func TestPutBillingModeHandler_MalformedBodyRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-malformed-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutBillingModeHandler_InvalidValueRefused proves a nonsense
// billingMode value 400s rather than being written.
func TestPutBillingModeHandler_InvalidValueRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-invalid-put"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := putBillingMode(t, srv, session, practiceID, "cash_app")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
