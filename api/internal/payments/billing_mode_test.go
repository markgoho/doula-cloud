package payments_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const billingModePath = "/payments/billing-mode"

func (e *env) billingMode() payments.BillingModeView {
	e.t.Helper()
	return reqtest.Expect[payments.BillingModeView](e.t, http.MethodGet, e.url(billingModePath), http.StatusOK, e.signedIn)
}

func (e *env) putBillingMode(mode string, want int) {
	e.t.Helper()
	e.expect(http.MethodPut, billingModePath, want, reqtest.JSON(payments.PutBillingModeRequest{BillingMode: mode}))
}

func (e *env) billingModeChangedRows() int {
	e.t.Helper()
	return testdb.Scalar[int](e.t, e.db.Admin, `SELECT count(*) FROM activity WHERE practice_id = $1 AND subject_kind = 'practice' AND action = 'billing_mode_changed'`, e.Practice.ID)
}

// TestGetBillingModeHandler_UnsetReturnsNoValue proves a Practice that
// has never chosen a billing mode reads back an empty BillingModeView.
func TestGetBillingModeHandler_UnsetReturnsNoValue(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))

	if out := e.billingMode(); out.BillingMode != nil {
		t.Fatalf("billingMode = %v, want nil", *out.BillingMode)
	}
}

// TestGetBillingModeHandler_AnyStaffCanRead proves a Doula -- who cannot
// read Invoice/Payment money at all under ADR-0008 -- can still read this
// one Practice-level fact, per #270's own reasoning for the sibling
// "whether the Practice can raise an Invoice at all" row.
func TestGetBillingModeHandler_AnyStaffCanRead(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()), withBillingMode(payments.BillingModeByHand))

	if out := e.billingMode(); out.BillingMode == nil || *out.BillingMode != string(payments.BillingModeByHand) {
		t.Fatalf("billingMode = %v, want %q", out.BillingMode, payments.BillingModeByHand)
	}
}

// TestPutBillingModeHandler_OwnerCanChangeEstablishedMode proves an
// Owner may change an already-established billing_mode, and the change
// is recorded to the Activity log.
func TestPutBillingModeHandler_OwnerCanChangeEstablishedMode(t *testing.T) {
	e := newEnv(t, withBillingMode(payments.BillingModeStripe))

	e.putBillingMode(string(payments.BillingModeByHand), http.StatusOK)

	mode, ok := billingModeOfPractice(t, e.db, e.Practice.ID)
	if !ok || mode != string(payments.BillingModeByHand) {
		t.Fatalf("billing_mode = (%q, ok=%v), want (%q, true)", mode, ok, payments.BillingModeByHand)
	}
	if got := e.billingModeChangedRows(); got != 1 {
		t.Fatalf("billing_mode_changed activity rows = %d, want 1", got)
	}
}

// TestPutBillingModeHandler_SameModeRecordsNothing proves a save that
// names the mode already stored answers 200 with that mode and writes no
// Activity row (#1968). Settings > Payments always offers its Save, so a
// press with nothing changed is ordinary, and the ledger must not say the
// billing mode changed when it did not.
func TestPutBillingModeHandler_SameModeRecordsNothing(t *testing.T) {
	e := newEnv(t, withBillingMode(payments.BillingModeStripe))

	out := reqtest.Expect[payments.BillingModeView](t, http.MethodPut, e.url(billingModePath), http.StatusOK,
		e.signedIn, reqtest.JSON(payments.PutBillingModeRequest{BillingMode: string(payments.BillingModeStripe)}))

	if out.BillingMode == nil || *out.BillingMode != string(payments.BillingModeStripe) {
		t.Fatalf("billingMode = %v, want %q", out.BillingMode, payments.BillingModeStripe)
	}
	if got := e.billingModeChangedRows(); got != 0 {
		t.Fatalf("billing_mode_changed activity rows = %d, want 0 for a save that changed nothing", got)
	}
}

// TestPutBillingModeHandler_Refusals proves an Admin -- who may read this
// fact -- may not change it once established; only an Owner may (#271, by
// analogy to Connect onboarding). It also proves a body that is not JSON
// and a nonsense billingMode value 400 rather than being written.
func TestPutBillingModeHandler_Refusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		staff fixture.StaffOption
		body  reqtest.Option
		want  int
	}{
		{"an Admin", fixture.Admin(), reqtest.JSON(payments.PutBillingModeRequest{BillingMode: string(payments.BillingModeByHand)}), http.StatusForbidden},
		{caseBodyNotJSON, fixture.Owner(), reqtest.RawJSON([]byte("{not json")), http.StatusBadRequest},
		{"an invalid value", fixture.Owner(), reqtest.JSON(payments.PutBillingModeRequest{BillingMode: "cash_app"}), http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff), withBillingMode(payments.BillingModeStripe))

			e.expect(http.MethodPut, billingModePath, tc.want, tc.body)

			if mode, _ := billingModeOfPractice(t, e.db, e.Practice.ID); mode != string(payments.BillingModeStripe) {
				t.Fatalf("billing_mode = %q, want unchanged %q", mode, payments.BillingModeStripe)
			}
		})
	}
}
