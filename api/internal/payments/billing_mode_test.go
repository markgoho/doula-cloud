package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func getBillingMode(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	resp := reqtest.Get(t, srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", reqtest.Session(session))
	return resp
}

func putBillingMode(t *testing.T, srv *httptest.Server, session, practiceID, mode string) *http.Response {
	t.Helper()
	resp := reqtest.Put(t, srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", reqtest.JSON(payments.PutBillingModeRequest{BillingMode: mode}), reqtest.Session(session))
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
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
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
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
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
	mode, ok := billingModeOfPractice(t, db, practiceID)
	if !ok || mode != string(payments.BillingModeByHand) {
		t.Fatalf("billing_mode = (%q, ok=%v), want (%q, true)", mode, ok, payments.BillingModeByHand)
	}

	actionCount := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM activity WHERE practice_id = $1 AND subject_kind = 'practice' AND action = 'billing_mode_changed'`,
		practiceID)
	if actionCount != 1 {
		t.Fatalf("billing_mode_changed activity rows = %d, want 1", actionCount)
	}
}

// TestPutBillingModeHandler_SameModeRecordsNothing proves a save that
// names the mode already stored answers 200 with that mode and writes no
// Activity row (#1968). Settings > Payments always offers its Save, so a
// press with nothing changed is ordinary, and the ledger must not say the
// billing mode changed when it did not.
func TestPutBillingModeHandler_SameModeRecordsNothing(t *testing.T) {
	db := testdb.New(t)
	const uid = "billing-mode-owner-same"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SeedBillingMode(t, db, practiceID, string(payments.BillingModeStripe))
	srv, session := newInvoiceServer(t, db, uid, payments.NewFakeClient())
	defer srv.Close()

	resp := putBillingMode(t, srv, session, practiceID, string(payments.BillingModeStripe))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out payments.BillingModeView
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.BillingMode == nil || *out.BillingMode != string(payments.BillingModeStripe) {
		t.Fatalf("billingMode = %v, want %q", out.BillingMode, payments.BillingModeStripe)
	}

	actionCount := testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM activity WHERE practice_id = $1 AND subject_kind = 'practice' AND action = 'billing_mode_changed'`,
		practiceID)
	if actionCount != 0 {
		t.Fatalf("billing_mode_changed activity rows = %d, want 0 for a save that changed nothing", actionCount)
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
	mode, _ := billingModeOfPractice(t, db, practiceID)
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

	resp := reqtest.Put(t, srv.URL+"/api/practices/"+practiceID+"/payments/billing-mode", reqtest.RawJSON([]byte("{not json")), reqtest.Session(session))
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
