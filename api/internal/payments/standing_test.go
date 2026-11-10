package payments_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestPostInvoiceHandler_RefusesAnInvoiceForAnErasedClient: a by-hand
// Invoice asks for no email, so before the gate it was raised, open, for
// a Client whose erasure had already required every Invoice settled
// (ADR-0027) -- an unsettled Invoice on an erased record.
func TestPostInvoiceHandler_RefusesAnInvoiceForAnErasedClient(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Doula()))
	engagementID, _ := e.signedContract()
	clientID := testdb.Scalar[string](t, e.DB.Admin, `SELECT client_id FROM engagements WHERE id = $1`, engagementID)
	fixture.EraseClient(t, e.DB, clientID)
	byHand := string(payments.BillingModeByHand)

	got := e.postRefused(invoicesPath(engagementID), http.StatusConflict, reqtest.JSON(payments.CreateInvoiceRequest{BillingMode: &byHand}))

	if _, want := clientstanding.ActCreateInvoice.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
	if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM invoices WHERE practice_id = $1`, e.Practice.ID); n != 0 {
		t.Errorf("invoices = %d, want 0", n)
	}
}
